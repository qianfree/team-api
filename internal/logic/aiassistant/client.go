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

// streamChunk 流式分片。
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *Usage `json:"usage"`
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

// ChatStream 流式对话：逐 delta 回调 onDelta，返回最终 usage。
// ctx 取消（管理员关闭页面/停止生成）会联动中断上游请求，及时止损 token。
func (c *Client) ChatStream(ctx context.Context, messages []ChatMessage, maxTokens int, temperature float64, onDelta func(string)) (*Usage, error) {
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
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, gerror.Wrap(err, "调用 AI 助手端点失败")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := readErrMessage(resp.Body)
		return nil, gerror.Newf("AI 助手端点返回非成功状态（HTTP %d）: %s", resp.StatusCode, msg)
	}

	usage := &Usage{}
	reader := bufio.NewReaderSize(resp.Body, 64<<10)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return usage, gerror.Wrap(err, "读取 AI 助手流式响应失败")
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
				return usage, nil
			}
			continue
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // 兼容端点夹带的非 JSON 行，跳过不中断
		}
		if chunk.Usage != nil {
			*usage = *chunk.Usage
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" && onDelta != nil {
			onDelta(chunk.Choices[0].Delta.Content)
		}
	}
	// 流未发 [DONE] 自然结束（部分兼容端点的形态），usage 原样返回
	return usage, nil
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
