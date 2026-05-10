package zhipu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/channel/claude"
	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/override"
)

// Adaptor 智谱 GLM 供应商适配器（V4 OpenAI 兼容接口）
type Adaptor struct {
	info *common.RelayInfo
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// GetRequestURL 构建上游请求 URL。
// 智谱 V4 API 路径为 /api/paas/v4/xxx，与标准 OpenAI 路径不同。
// Claude 协议请求走 Anthropic 兼容端点 /api/anthropic/v1/messages。
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeClaudeMessages:
		return baseURL + "/api/anthropic/v1/messages", nil
	case constant.RelayModeChatCompletions, constant.RelayModeResponses:
		return baseURL + "/api/paas/v4/chat/completions", nil
	case constant.RelayModeEmbeddings:
		return baseURL + "/api/paas/v4/embeddings", nil
	case constant.RelayModeImagesGenerations:
		return baseURL + "/api/paas/v4/images/generations", nil
	default:
		return baseURL + "/api/paas/v4/chat/completions", nil
	}
}

func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)

	// 透传客户端 Content-Type，无则回退 application/json
	if ct := info.RequestHeaders.Get("Content-Type"); ct != "" {
		header.Set("Content-Type", ct)
	} else {
		header.Set("Content-Type", "application/json")
	}

	// 透传客户端 Accept，流式请求回退 text/event-stream，否则 application/json
	if accept := info.RequestHeaders.Get("Accept"); accept != "" {
		header.Set("Accept", accept)
	} else if info.IsStream {
		header.Set("Accept", "text/event-stream")
	} else {
		header.Set("Accept", "application/json")
	}

	return nil
}

// ConvertRequest 转换请求体。
// Claude 入站格式直接透传到智谱 Anthropic 端点，其他格式先转为 OpenAI 再做 GLM 特有适配。
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	// Claude 入站：透传请求体，仅做模型映射
	if info.InboundFormat == constant.RelayFormatClaude {
		return convertClaudeRequestForZhipu(requestBody, info)
	}

	// 非 OpenAI 格式先转换为 OpenAI
	if info.InboundFormat != "" && info.InboundFormat != constant.RelayFormatOpenAI {
		converted, err := openai.ConvertToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		requestBody = converted
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawMap); err != nil {
		return bytes.NewReader(requestBody), nil
	}

	// GLM 特有的转换（TopP 裁剪、图片前缀剥离）
	rawMap = applyGLMCompatibility(rawMap)

	// 注入 stream_options（流式请求需要 usage 信息用于计费）
	rawMap = injectStreamOptions(rawMap, info)

	// 注入思考模式参数
	rawMap = injectThinkingParams(rawMap, info)

	// 模型名映射
	if info.ChannelMeta.IsModelMapped {
		rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
	}

	result, err := json.Marshal(rawMap)
	if err != nil {
		return nil, fmt.Errorf("marshal converted request failed: %w", err)
	}
	return bytes.NewReader(result), nil
}

// injectStreamOptions 为流式请求注入 stream_options:{include_usage:true}
func injectStreamOptions(rawMap map[string]json.RawMessage, info *common.RelayInfo) map[string]json.RawMessage {
	if !info.IsStream {
		return rawMap
	}
	if _, exists := rawMap["stream_options"]; !exists {
		rawMap["stream_options"] = json.RawMessage(`{"include_usage":true}`)
	}
	return rawMap
}

// injectThinkingParams 根据 RelayInfo 中的思考后缀注入 GLM 思考模式参数。
//
// GLM 思考模式参数说明（仅 GLM-4.5 及以上模型支持）：
//   - thinking.type: "enabled"(默认) / "disabled"
//   - GLM-5.1/5/5v-Turbo/4.7/4.5V：强制思考
//   - GLM-4.6/4.6V/4.5：模型自动判断是否思考
//   - thinking.clear_thinking: 控制是否清除历史 reasoning_content（默认 true）
//
// 注入优先级：客户端已显式设置 > 后缀路由注入 > 不干预（保留 GLM 默认行为）
func injectThinkingParams(rawMap map[string]json.RawMessage, info *common.RelayInfo) map[string]json.RawMessage {
	// 客户端已显式设置 thinking 参数 → 不干预
	if _, clientSet := rawMap["thinking"]; clientSet {
		return rawMap
	}

	// -nothinking 后缀：显式关闭思考
	if info.ThinkingDisabled {
		rawMap["thinking"] = json.RawMessage(`{"type":"disabled"}`)
		return rawMap
	}

	// -thinking 后缀：显式开启思考
	if info.ThinkingEnabled {
		rawMap["thinking"] = json.RawMessage(`{"type":"enabled"}`)
		return rawMap
	}

	// 无后缀：不干预，保留 GLM 默认行为（默认 enabled）
	return rawMap
}

// applyGLMCompatibility 处理 GLM 特有的兼容性问题：TopP 上限裁剪、base64 图片前缀剥离
func applyGLMCompatibility(rawMap map[string]json.RawMessage) map[string]json.RawMessage {
	// TopP 裁剪：GLM 要求 top_p < 1.0
	if topPRaw, ok := rawMap["top_p"]; ok {
		var topP float64
		if err := json.Unmarshal(topPRaw, &topP); err == nil && topP >= 1.0 {
			rawMap["top_p"] = json.RawMessage(`0.99`)
		}
	}

	// base64 图片 URL 前缀剥离：GLM 视觉模型要求纯 base64 数据
	if messagesRaw, ok := rawMap["messages"]; ok {
		messagesBytes, changed := stripImageURLPrefixes(messagesRaw)
		if changed {
			rawMap["messages"] = messagesBytes
		}
	}

	return rawMap
}

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
		if constant.RelayMode(info.RelayMode) == constant.RelayModeImagesGenerations {
			timeout = 300
		}
	}

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request failed: %w", err)
	}

	// 请求体已完整发送，立即关闭释放资源
	if httpReq.Body != nil {
		_ = httpReq.Body.Close()
	}

	return resp, nil
}

// DoResponse 处理上游响应。
// Claude 入站委托 claude.Adaptor 原生直通；其他格式委托 openai.Adaptor。
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	clientFormat := info.GetOriginalClientFormat()
	if clientFormat == constant.RelayFormatClaude {
		delegate := &claude.Adaptor{}
		delegate.Init(info)
		return delegate.DoResponse(ctx, resp, info, writer)
	}

	delegate := &openai.Adaptor{}
	delegate.Init(info)
	return delegate.DoResponse(ctx, resp, info, writer)
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

var _ common.Adaptor = (*Adaptor)(nil)

// convertClaudeRequestForZhipu 处理 Claude 入站请求的 GLM 兼容适配（模型映射）。
// Claude 协议请求体直接透传到智谱 Anthropic 兼容端点，无需格式转换。
func convertClaudeRequestForZhipu(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	if !info.ChannelMeta.IsModelMapped {
		return bytes.NewReader(requestBody), nil
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawMap); err != nil {
		return bytes.NewReader(requestBody), nil
	}
	rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)

	result, err := json.Marshal(rawMap)
	if err != nil {
		return bytes.NewReader(requestBody), nil
	}
	return bytes.NewReader(result), nil
}
