package aireqanalyze

import (
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

// AIClient OpenAI 兼容端点的极简客户端（/chat/completions）。
// 独立于系统渠道体系：不经调度、不计费、不进审计日志——这正是「独立 AI」
// 架构决策的落地（见设计文档 3.2 节）。分析内容是敏感数据，端点应指向
// 内网自部署模型；内网自签 HTTPS 部署时放宽证书校验由调用方按需扩展。
type AIClient struct {
	endpoint string // 最终请求端点：base 模式 = base_url+"/chat/completions"；full 模式 = 配置原值
	apiKey   string
	model    string
	client   *http.Client
}

// NewAIClient 按配置构造客户端（端点模式在此一次性解析）。
// base：ai_base_url 为基础地址，自动拼接 /chat/completions（标准 OpenAI 兼容端点）；
// full：ai_base_url 为完整请求地址原样使用——应对路径非标准的兼容端点
// （如某些网关的自定义路径）。
func NewAIClient(cfg *Config) *AIClient {
	base := strings.TrimRight(cfg.AIBaseURL, "/")
	endpoint := base + "/chat/completions"
	if cfg.AIEndpointMode == EndpointModeFull {
		endpoint = base
	}
	return &AIClient{
		endpoint: endpoint,
		apiKey:   cfg.AIAPIKey,
		model:    cfg.AIModel,
		client: &http.Client{
			Timeout: time.Duration(cfg.AITimeoutMs) * time.Millisecond,
			Transport: &http.Transport{
				MaxIdleConns:    4,
				IdleConnTimeout: 90 * time.Second,
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
		},
	}
}

// ChatMessage 一条对话消息。
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest /chat/completions 请求体。
type chatRequest struct {
	Model          string        `json:"model"`
	Messages       []ChatMessage `json:"messages"`
	MaxTokens      int           `json:"max_tokens,omitempty"`
	Temperature    float64       `json:"temperature,omitempty"`
	ResponseFormat *respFormat   `json:"response_format,omitempty"`
}

type respFormat struct {
	Type string `json:"type"`
}

// chatResponse /chat/completions 响应体（只解析分析所需的字段）。
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// AIResult 一次分析的返回。
type AIResult struct {
	Content          string // 模型输出文本
	PromptTokens     int    // 输入 token
	CompletionTokens int    // 输出 token
	TotalTokens      int    // 总 token（端点未返回时按输入+输出兜底）
}

// AIRejectedError 分析端点明确拒绝本请求（HTTP 4xx 或响应携带 error 字段），
// 典型场景是内容安全审查拦截——送审内容本身含违规文本被端点拒收。
// 与传输错误/服务端故障区分：端点工作正常，只是拒绝了这批内容，
// 调用方可据此降级为更小批次重试以定位被拒单元。
type AIRejectedError struct {
	Status  int
	Message string
}

func (e *AIRejectedError) Error() string {
	return fmt.Sprintf("分析 AI 返回错误（HTTP %d）: %s", e.Status, e.Message)
}

// truncateText 供错误信息截断原文片段（按字符截，避免中文乱码）。
func truncateText(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

// ChatOptions 单次调用选项。
type ChatOptions struct {
	MaxTokens    int     // 覆盖配置的 max_tokens（0 = 用配置值）
	Temperature  float64 // 采样温度（0 = 确定性输出，适合结构化分析）
	ResponseJSON bool    // 请求 JSON 输出模式（端点不支持时由调用方降级解析）
	TimeoutMs    int     // 覆盖配置的超时（0 = 用配置值）
}

// Chat 执行一次对话补全。
func (c *AIClient) Chat(ctx context.Context, messages []ChatMessage, opts ChatOptions) (*AIResult, error) {
	maxTokens := opts.MaxTokens
	reqOpts := chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: opts.Temperature,
	}
	if maxTokens > 0 {
		reqOpts.MaxTokens = maxTokens
	}
	if opts.ResponseJSON {
		reqOpts.ResponseFormat = &respFormat{Type: "json_object"}
	}

	body, err := json.Marshal(reqOpts)
	if err != nil {
		return nil, err
	}

	reqCtx := ctx
	if opts.TimeoutMs > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, time.Duration(opts.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, gerror.Wrap(err, "调用分析 AI 失败")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 8 MiB 上限防御异常端点
	if err != nil {
		return nil, gerror.Wrap(err, "读取分析 AI 响应失败")
	}

	var parsed chatResponse
	// 容错解析：非 JSON 响应体（如网关 HTML 错误页）不立即失败，先按状态码分类
	_ = json.Unmarshal(respBody, &parsed)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := ""
		if parsed.Error != nil {
			msg = parsed.Error.Message
		}
		if msg == "" {
			msg = truncateText(string(respBody), 200)
		}
		// 4xx = 端点明确拒绝本请求（内容安全审查拦截、参数错误等），
		// 与传输/服务端故障区分——调用方可据此降级为更小批次重试
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return nil, &AIRejectedError{Status: resp.StatusCode, Message: msg}
		}
		return nil, gerror.Newf("分析 AI 返回非成功状态（HTTP %d）: %s", resp.StatusCode, msg)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		// 2xx 但带错误字段（少见的服务端约定）
		return nil, &AIRejectedError{Status: resp.StatusCode, Message: parsed.Error.Message}
	}
	if parsed.Choices == nil || len(parsed.Choices) == 0 {
		if !json.Valid(respBody) {
			return nil, gerror.Newf("分析 AI 响应非 JSON（HTTP %d）: %s", resp.StatusCode, truncateText(string(respBody), 200))
		}
		return nil, gerror.Newf("分析 AI 响应缺少 choices（HTTP %d）", resp.StatusCode)
	}

	total := parsed.Usage.TotalTokens
	if total == 0 {
		total = parsed.Usage.PromptTokens + parsed.Usage.CompletionTokens
	}
	return &AIResult{
		Content:          parsed.Choices[0].Message.Content,
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
		TotalTokens:      total,
	}, nil
}

// ConnTestResult 连通性测试结果。
type ConnTestResult struct {
	OK         bool   `json:"ok"`
	Model      string `json:"model"`
	LatencyMs  int64  `json:"latency_ms"`
	TokensUsed int    `json:"tokens_used"`
	Reply      string `json:"reply"`
	Error      string `json:"error,omitempty"`
}

// TestConnection 用最小补全验证端点/密钥/模型可用性。
// 返回的结果始终带完整信息（OK=false 时 Error 有值），不返回 error——
// 连通性测试的失败本身就是有效结果，由接口层原样呈现给管理员。
func (c *AIClient) TestConnection(ctx context.Context) *ConnTestResult {
	res := &ConnTestResult{Model: c.model}

	start := time.Now()
	aiRes, err := c.Chat(ctx, []ChatMessage{
		{Role: "user", Content: "连通性测试，请回复：ok"},
	}, ChatOptions{MaxTokens: 16, Temperature: 0})
	res.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		res.Error = err.Error()
		return res
	}

	res.OK = true
	res.TokensUsed = aiRes.TotalTokens
	res.Reply = aiRes.Content
	return res
}

// String 供日志输出的脱敏摘要。
func (c *AIClient) String() string {
	key := ""
	if c.apiKey != "" {
		key = fmt.Sprintf("key(%d chars)", len(c.apiKey))
	}
	return fmt.Sprintf("AIClient{endpoint=%s model=%s %s}", c.endpoint, c.model, key)
}
