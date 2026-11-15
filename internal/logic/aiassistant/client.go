package aiassistant

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

// Client OpenAI 兼容端点的对话客户端（/chat/completions，流式 + 非流式）。
// 独立于系统渠道体系：不经调度、不计费（「独立 AI」架构决策，同 aianalysis）。
type Client struct {
	endpoint string // 最终请求端点：base 模式 = base_url+"/chat/completions"；full 模式 = 配置原值
	apiKey   string
	model    string
	client   *http.Client
}

// ChatMessage 一条对话消息。
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Usage 一次调用的 token 计量（端点未返回时为零值）。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewClient 按配置构造客户端（端点模式在此一次性解析）。
func NewClient(cfg *Config) *Client {
	base := strings.TrimRight(cfg.BaseURL, "/")
	endpoint := base + "/chat/completions"
	if cfg.EndpointMode == EndpointModeFull {
		endpoint = base
	}
	return &Client{
		endpoint: endpoint,
		apiKey:   cfg.APIKey,
		model:    cfg.Model,
		// 流式长回复不设整体 Timeout（会把慢生成掐断），总时长由调用方 ctx 控制
		//（handler 侧以配置超时包裹 + 客户端断开联动取消）
		client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:          4,
				IdleConnTimeout:       90 * time.Second,
				ResponseHeaderTimeout: time.Duration(cfg.TimeoutMs) * time.Millisecond,
				TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
			},
		},
	}
}

// chatRequest /chat/completions 请求体。
type chatRequest struct {
	Model         string        `json:"model"`
	Messages      []ChatMessage `json:"messages"`
	MaxTokens     int           `json:"max_tokens,omitempty"`
	Temperature   float64       `json:"temperature,omitempty"`
	Stream        bool          `json:"stream,omitempty"`
	StreamOptions *streamOpts   `json:"stream_options,omitempty"`
}

type streamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

// chatResponse 非流式响应（只解析所需字段）。
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// streamDelta 单条分片的增量字段。
// 同时兼容两种思考字段：reasoning_content（DeepSeek/Qwen/GLM 等）与
// reasoning（部分兼容端点的写法），两者都不参与正文。
type streamDelta struct {
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
	Reasoning        string `json:"reasoning"`
}

// ReasoningText 思考增量（两种字段名任取其一）。
func (d streamDelta) ReasoningText() string {
	if d.ReasoningContent != "" {
		return d.ReasoningContent
	}
	return d.Reasoning
}

// streamChunk 流式分片。
type streamChunk struct {
	Choices []struct {
		Delta        streamDelta `json:"delta"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage *Usage `json:"usage"`
}

// FinishReasonLength 上游因 max_tokens 被截断（其余取值：stop / content_filter / tool_calls）。
const FinishReasonLength = "length"

// StreamResult 一次流式调用的结果。
type StreamResult struct {
	Usage        Usage
	FinishReason string // 端点未返回时为空
}

// Truncated 本次输出是否因达到 max_tokens 被截断。
// 这是「代码不完整」的唯一可靠信号——模型自然停止（stop）但漏写闭合行属另一类问题，
// 由调用方按隐式闭合单独处理。
func (r *StreamResult) Truncated() bool {
	return r != nil && r.FinishReason == FinishReasonLength
}

// ConnTestResult 连通性测试结果（与 aianalysis 同构）。
type ConnTestResult struct {
	OK         bool   `json:"ok"`
	Model      string `json:"model"`
	LatencyMs  int64  `json:"latency_ms"`
	TokensUsed int    `json:"tokens_used"`
	Reply      string `json:"reply"`
	Error      string `json:"error,omitempty"`
}

// ChatStream 流式对话：逐增量回调 onDelta（正文）与 onReasoning（思考过程），
// 返回最终 usage。推理型模型会先持续输出思考内容再输出正文，两条通道必须分开
// 回传，否则前端只能干等正文首 token。
// ctx 取消（管理员关闭页面/停止生成）会联动中断上游请求，及时止损 token。
func (c *Client) ChatStream(ctx context.Context, messages []ChatMessage, maxTokens int, temperature float64, onDelta, onReasoning func(string)) (*StreamResult, error) {
	reqBody := chatRequest{
		Model:       c.model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      true,
		// OpenAI 兼容端点普遍支持/忽略该字段，最终 chunk 会带 usage 供成本透出
		StreamOptions: &streamOpts{IncludeUsage: true},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return &StreamResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return &StreamResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return &StreamResult{}, gerror.Wrap(err, "调用 AI 助手端点失败")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := readErrMessage(resp.Body)
		return &StreamResult{}, gerror.Newf("AI 助手端点返回非成功状态（HTTP %d）: %s", resp.StatusCode, msg)
	}

	res := &StreamResult{}
	reader := bufio.NewReaderSize(resp.Body, 64<<10)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return res, gerror.Wrap(err, "读取 AI 助手流式响应失败")
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" || strings.HasPrefix(line, ":") {
			continue // 空行（事件分隔）与注释帧（保活）
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			if data == "[DONE]" {
				return res, nil
			}
			continue
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // 兼容端点夹带的非 JSON 行，跳过不中断
		}
		if chunk.Usage != nil {
			res.Usage = *chunk.Usage
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		if fr := chunk.Choices[0].FinishReason; fr != "" {
			res.FinishReason = fr
		}
		d := chunk.Choices[0].Delta
		if r := d.ReasoningText(); r != "" && onReasoning != nil {
			onReasoning(r)
		}
		if d.Content != "" && onDelta != nil {
			onDelta(d.Content)
		}
	}
	// 流未发 [DONE] 自然结束（部分兼容端点的形态），结果原样返回
	return res, nil
}

// IsMaxTokensRejected 判断上游错误是否在拒绝本次请求的 max_tokens
// （小上限模型的典型回应："max_tokens is too large" / "maximum context length"）。
// 命中时调用方可退回配置值重试一次——它发生在写出任何增量之前，重试安全。
func IsMaxTokensRejected(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, kw := range []string{"max_tokens", "max_output_tokens", "max new tokens", "context length"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	// "This model's maximum context length is 8192 tokens" 这类：抬高 max_tokens 后
	// prompt+max_tokens 超窗，同样是本端抬高导致的，值得回退重试一次。
	return strings.Contains(msg, "maximum") && strings.Contains(msg, "token")
}

// TestConnection 用最小补全验证端点/密钥/模型可用性（非流式）。
// 失败本身是有效结果，不返回 error——由接口层原样呈现给管理员。
func (c *Client) TestConnection(ctx context.Context) *ConnTestResult {
	res := &ConnTestResult{Model: c.model}
	start := time.Now()
	content, usage, err := c.chatOnce(ctx, []ChatMessage{
		{Role: "user", Content: "连通性测试，请回复：ok"},
	}, 16, 0)
	res.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.OK = true
	res.TokensUsed = usage.TotalTokens
	res.Reply = content
	return res
}

// chatOnce 非流式单次补全。
func (c *Client) chatOnce(ctx context.Context, messages []ChatMessage, maxTokens int, temperature float64) (string, Usage, error) {
	var zero Usage
	reqBody := chatRequest{
		Model:       c.model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", zero, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", zero, gerror.Wrap(err, "调用 AI 助手端点失败")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", zero, gerror.Wrap(err, "读取 AI 助手响应失败")
	}
	var parsed chatResponse
	_ = json.Unmarshal(raw, &parsed)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := parsed.Error.Message
		if msg == "" {
			msg = truncateForLog(string(raw), 200)
		}
		return "", zero, gerror.Newf("AI 助手端点返回非成功状态（HTTP %d）: %s", resp.StatusCode, msg)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", zero, gerror.Newf("AI 助手端点返回错误: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", zero, gerror.Newf("AI 助手端点响应缺少 choices（HTTP %d）", resp.StatusCode)
	}
	return parsed.Choices[0].Message.Content, parsed.Usage, nil
}

// readErrMessage 读取错误响应体的 message（容错非 JSON 响应，如网关 HTML 错误页）。
func readErrMessage(r io.Reader) string {
	raw, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil {
		return ""
	}
	var parsed struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &parsed) == nil {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return parsed.Error.Message
		}
		if parsed.Message != "" {
			return parsed.Message
		}
	}
	return truncateForLog(string(raw), 200)
}

// truncateForLog 供错误信息截断原文片段（按字符截，避免中文乱码）。
func truncateForLog(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

// String 供日志输出的脱敏摘要。
func (c *Client) String() string {
	key := ""
	if c.apiKey != "" {
		key = fmt.Sprintf("key(%d chars)", len(c.apiKey))
	}
	return fmt.Sprintf("AssistantClient{endpoint=%s model=%s %s}", c.endpoint, c.model, key)
}
