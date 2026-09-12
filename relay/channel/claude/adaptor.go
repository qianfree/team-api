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
	case constant.RelayModeChatCompletions, constant.RelayModeClaudeMessages,
		constant.RelayModeGeminiChat,
		constant.RelayModeResponses, constant.RelayModeResponsesCompact:
		// Responses/Gemini 入站：请求已由 relaykit 转为 Claude Messages 格式打 /v1/messages，
		// 响应再转回客户端原生格式
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
	// OpenAI/Gemini/Responses 入站 → Claude 上游已由 relaykit 转换矩阵接管
	//（convertRequestBody 在 adaptor 之前完成转换），此处只剩 Claude 同格式路径
	//（直连判定未通过时：有模型映射/thinking 后缀/参数改写）。矩阵意外未接管时
	// 显式报错，避免把外格式请求体静默透传给 Anthropic 端点。
	if info.InboundFormat != "" && info.InboundFormat != constant.RelayFormatClaude {
		return nil, constant.NewChannelError(fmt.Sprintf("claude adaptor: relaykit converter unavailable for %s inbound", info.InboundFormat), nil)
	}
	converted := bytes.NewReader(requestBody)
	return replaceModelIfNeeded(converted, info), nil
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

	timeout := info.ChannelMeta.Settings.GetTimeoutSeconds(info.RelayMode)

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request failed: %w", err)
	}

	return resp, nil
}

// DoResponse 处理上游响应，根据客户端格式分发
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	clientFormat := info.GetOriginalClientFormat()

	switch clientFormat {
	case constant.RelayFormatClaude:
		return a.handleClaudeNativeResponse(ctx, resp, info, writer)
	case constant.RelayFormatResponses:
		// Responses 入站（codex 等）：Claude 响应转换为 Responses 格式
		if info.IsStream {
			return a.handleStreamToResponses(ctx, resp, info, writer)
		}
		return a.handleNonStreamToResponses(ctx, resp, info, writer)
	case constant.RelayFormatOpenAI:
		if info.IsStream {
			return a.handleStreamToOpenAI(ctx, resp, info, writer)
		}
		return a.handleNonStreamToOpenAI(ctx, resp, info, writer)
	case constant.RelayFormatGemini:
		// Gemini 入站（请求侧走 relaykit Gemini→Claude 转换）：响应必须转回 Gemini 格式，
		// 否则 Gemini SDK 拿到 OpenAI chunk 解析失败
		if info.IsStream {
			return a.handleStreamToGemini(ctx, resp, info, writer)
		}
		return a.handleNonStreamToGemini(ctx, resp, info, writer)
	default:
		// openai / claude / gemini / responses 四种入站格式均已显式处理
		//（见 relay_handler.go 的 relayModeToInboundFormat）。走到这里说明新增了客户端格式
		// 却漏接响应侧转换——显式失败，而不是静默按 OpenAI 格式写出让客户端 SDK 解析失败。
		return nil, constant.NewChannelError(
			fmt.Sprintf("claude adaptor: unsupported client format %q", clientFormat), nil)
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
