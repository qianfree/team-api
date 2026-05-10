package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/common"

	loauth "github.com/qianfree/team-api/internal/logic/common/oauth"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/override"
)

// Adaptor Claude 供应商适配器
type Adaptor struct {
	info *common.RelayInfo
}

// Init 初始化适配器
func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// GetRequestURL 构建上游请求 URL
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeChatCompletions, constant.RelayModeClaudeMessages:
		return baseURL + "/v1/messages", nil
	default:
		return "", fmt.Errorf("unsupported relay mode for Claude: %d", info.RelayMode)
	}
}

// SetupRequestHeader 设置上游请求头
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	apiKey := info.ChannelMeta.ApiKey

	// OAuth 模式：使用 Bearer token 而非 x-api-key
	if loauth.IsOAuthKeyData(apiKey) {
		var oauthData loauth.OAuthKeyData
		if err := json.Unmarshal([]byte(apiKey), &oauthData); err == nil {
			header.Set("Authorization", "Bearer "+oauthData.AccessToken)
			header.Set("anthropic-version", "2023-06-01")
			header.Set("Content-Type", "application/json")
			header.Set("Accept", "application/json")
			return nil
		}
	}

	header.Set("x-api-key", apiKey)
	header.Set("anthropic-version", "2023-06-01")
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")

	if info.RequestHeaders != nil {
		for _, h := range []string{"X-Request-Id", "anthropic-beta"} {
			if v := info.RequestHeaders.Get(h); v != "" {
				header.Set(h, v)
			}
		}
	}

	return nil
}

// ConvertRequest 根据入站格式转换请求体为 Claude 格式
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	var converted io.Reader
	switch info.InboundFormat {
	case constant.RelayFormatClaude:
		converted = bytes.NewReader(requestBody)
	case constant.RelayFormatOpenAI:
		r, err := ConvertOpenAIToClaude(requestBody, info)
		if err != nil {
			return nil, err
		}
		converted = r
	case constant.RelayFormatGemini:
		r, err := ConvertGeminiToClaude(requestBody, info)
		if err != nil {
			return nil, err
		}
		converted = r
	case constant.RelayFormatResponses:
		r, err := ConvertResponsesToClaude(requestBody, info)
		if err != nil {
			return nil, err
		}
		converted = r
	default:
		converted = bytes.NewReader(requestBody)
	}
	result := replaceModelIfNeeded(converted, info)

	// Thinking 后缀路由
	if info.ThinkingEnabled {
		result = injectClaudeThinking(result, info)
	} else if info.ReasoningEffort != "" {
		result = injectClaudeEffort(result, info)
	}

	return result, nil
}

// DoRequest 发送请求到上游
func (a *Adaptor) DoRequest(ctx context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	reqURL, err := a.GetRequestURL(info)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	if err := a.SetupRequestHeader(httpReq.Header, info); err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}

	if hdrOverrides, hdrErr := override.ApplyHeaderOverride(info); hdrErr == nil && len(hdrOverrides) > 0 {
		override.MergeHeaderOverrides(httpReq.Header, hdrOverrides)
	}

	timeout := info.ChannelMeta.Settings.TimeoutSeconds
	if timeout <= 0 {
		timeout = 60
	}

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request failed: %w", err)
	}

	return resp, nil
}

// injectClaudeThinking 注入 Claude thinking 配置（-thinking 后缀）
// 设 thinking.type=enabled, budget_tokens=80%*max_tokens, temperature=1.0
func injectClaudeThinking(r io.Reader, info *common.RelayInfo) io.Reader {
	body, err := io.ReadAll(r)
	if err != nil {
		return r
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawMap); err != nil {
		return bytes.NewReader(body)
	}

	// 获取 max_tokens
	var maxTokens int
	if mt, ok := rawMap["max_tokens"]; ok {
		_ = json.Unmarshal(mt, &maxTokens)
	}
	if maxTokens < 1280 {
		maxTokens = 16384
	}
	budgetTokens := maxTokens * 80 / 100
	if budgetTokens < 1280 {
		budgetTokens = 1280
	}

	// 设置 thinking
	rawMap["thinking"] = json.RawMessage(fmt.Sprintf(`{"type":"enabled","budget_tokens":%d}`, budgetTokens))
	// Claude thinking 要求 temperature=1.0
	rawMap["temperature"] = json.RawMessage(`1.0`)

	result, err := json.Marshal(rawMap)
	if err != nil {
		return bytes.NewReader(body)
	}
	return bytes.NewReader(result)
}

// injectClaudeEffort 注入 Claude effort 级别（-high/-low 等后缀）
// 使用 adaptive thinking 模式
func injectClaudeEffort(r io.Reader, info *common.RelayInfo) io.Reader {
	body, err := io.ReadAll(r)
	if err != nil {
		return r
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawMap); err != nil {
		return bytes.NewReader(body)
	}

	// 仅在客户端未显式设置 thinking 时注入
	if _, exists := rawMap["thinking"]; !exists {
		rawMap["thinking"] = json.RawMessage(fmt.Sprintf(`{"type":"adaptive"}`))
	}

	result, err := json.Marshal(rawMap)
	if err != nil {
		return bytes.NewReader(body)
	}
	return bytes.NewReader(result)
}

// DoResponse 处理上游响应，根据客户端格式分发
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	clientFormat := info.GetOriginalClientFormat()

	switch clientFormat {
	case constant.RelayFormatClaude:
		return a.handleClaudeNativeResponse(ctx, resp, info, writer)
	case constant.RelayFormatOpenAI:
		if info.IsStream {
			return a.handleStreamToOpenAI(ctx, resp, info, writer)
		}
		return a.handleNonStreamToOpenAI(ctx, resp, info, writer)
	default:
		// 兜底：默认 OpenAI 转换
		if info.IsStream {
			return a.handleStreamToOpenAI(ctx, resp, info, writer)
		}
		return a.handleNonStreamToOpenAI(ctx, resp, info, writer)
	}
}

// GetChannelName 返回渠道名称
func (a *Adaptor) GetChannelName() string {
	return "Claude"
}

// 确保接口实现
var _ common.Adaptor = (*Adaptor)(nil)

// replaceModelIfNeeded 如果渠道有模型映射，替换请求体中的模型名
func replaceModelIfNeeded(r io.Reader, info *common.RelayInfo) io.Reader {
	if !info.ChannelMeta.IsModelMapped {
		return r
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return r
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawMap); err != nil {
		return bytes.NewReader(body)
	}
	rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
	result, err := json.Marshal(rawMap)
	if err != nil {
		return bytes.NewReader(body)
	}
	return bytes.NewReader(result)
}
