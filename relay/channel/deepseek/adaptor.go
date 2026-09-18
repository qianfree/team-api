package deepseek

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

// Adaptor DeepSeek 供应商适配器
type Adaptor struct {
	info *common.RelayInfo
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// GetRequestURL 构建上游请求 URL。
// Completions(FIM) 模式使用 /beta/completions 端点，Responses 视渠道开关直连 /v1/responses 或转 chat，其余走标准 OpenAI 路径。
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeCompletions:
		betaURL := baseURL
		if !strings.HasSuffix(betaURL, "/beta") {
			betaURL += "/beta"
		}
		return betaURL + "/completions", nil
	case constant.RelayModeChatCompletions:
		// chat_via_responses 桥接：请求体已由 relaykit 转成 Responses 格式，URL 必须同步
		// 切到 /v1/responses——否则 Responses 体打到 chat 端点，上游按 chat 格式反序列化
		// 直接 400（tools[0] missing field `function`）
		if info.UseResponsesAPI {
			return baseURL + "/v1/responses", nil
		}
		return baseURL + "/v1/chat/completions", nil
	case constant.RelayModeClaudeMessages:
		return baseURL + "/anthropic/v1/messages", nil
	case constant.RelayModeResponses, constant.RelayModeResponsesCompact:
		// 上游原生支持 Responses 协议（渠道开启 supports_responses）：直连 Responses 端点
		if info.ChannelMeta.UpstreamSpeaksResponses() {
			if constant.RelayMode(info.RelayMode) == constant.RelayModeResponsesCompact {
				return baseURL + "/v1/responses/compact", nil
			}
			return baseURL + "/v1/responses", nil
		}
		// 兜底（chat-only 上游）：请求体先转 Chat Completions 格式再发送（与 openai.Adaptor 行为对齐）
		return baseURL + "/v1/chat/completions", nil
	case constant.RelayModeEmbeddings:
		return baseURL + "/v1/embeddings", nil
	default:
		return baseURL + "/v1/chat/completions", nil
	}
}

func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return nil
}

// ConvertRequest 转换请求体。
// Claude 入站直接透传到 Anthropic 兼容端点，Responses 模式处理 reasoning_effort 映射，其他格式先转为 OpenAI 再做 DeepSeek 特有适配。
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	// Claude 入站：透传请求体，仅做模型映射和思考参数注入
	if info.InboundFormat == constant.RelayFormatClaude {
		return convertClaudeRequestForDeepSeek(requestBody, info)
	}

	// Responses 模式：上游原生支持时保持 Responses 格式（模型映射 + reasoning.effort 注入）。
	// chat-only 上游的 Responses→chat 转换已由 relaykit 接管（矩阵 Responses→OpenAI +
	// PostProcessConvertedRequest 注入 DeepSeek 适配），此分支不可达，注册缺失时显式报错。
	if constant.RelayMode(info.RelayMode) == constant.RelayModeResponses ||
		constant.RelayMode(info.RelayMode) == constant.RelayModeResponsesCompact {
		if info.ChannelMeta.UpstreamSpeaksResponses() {
			return convertResponsesRequestForDeepSeek(requestBody, info)
		}
		return nil, constant.NewChannelError("deepseek adaptor: relaykit converter unavailable for responses inbound on chat-only channel", nil)
	}

	processed, err := postProcessRequest(requestBody, info)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(processed), nil
}

// postProcessRequest DeepSeek 请求适配：模型映射 + stream_options 注入
// （OpenAI 同格式入站的 adaptor 路径用；relaykit 路径两者均已由桥接层覆盖，
// thinking 后缀方言已随全局虚拟模型后缀机制移除，故不再实现 RequestPostProcessor）。
func postProcessRequest(requestBody []byte, info *common.RelayInfo) ([]byte, error) {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawMap); err != nil {
		return requestBody, nil
	}

	// 模型名映射
	if info.ChannelMeta.IsModelMapped {
		rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
	}

	// 注入 stream_options（流式请求需要 usage 信息用于计费）
	rawMap = injectStreamOptions(rawMap, info)

	result, err := json.Marshal(rawMap)
	if err != nil {
		return nil, fmt.Errorf("marshal converted request failed: %w", err)
	}
	return result, nil
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
		timeout = 300
	}
	if constant.RelayMode(info.RelayMode) == constant.RelayModeImagesGenerations && timeout < 600 {
		timeout = 600
	}

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	return client.Do(httpReq)
}

// DoResponse 处理上游响应。
// Claude 入站委托 claude.Adaptor 原生直通；其他格式委托 openai.Adaptor。
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	if info.GetOriginalClientFormat() == constant.RelayFormatClaude {
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

// convertClaudeRequestForDeepSeek 处理 Claude 入站请求的 DeepSeek 兼容适配（模型映射 + 思考参数）。
func convertClaudeRequestForDeepSeek(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawMap); err != nil {
		return bytes.NewReader(requestBody), nil
	}

	if info.ChannelMeta.IsModelMapped {
		rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
	}

	result, err := json.Marshal(rawMap)
	if err != nil {
		return bytes.NewReader(requestBody), nil
	}
	return bytes.NewReader(result), nil
}

// convertResponsesRequestForDeepSeek 处理 Responses API 入站请求的 DeepSeek 兼容适配。
// 功能：模型名映射 + 将 RelayInfo 中的思考后缀参数映射到 Responses API 的 reasoning.effort 字段。
// 对齐 new-api commit 9724ef1b2 的实现逻辑。
func convertResponsesRequestForDeepSeek(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawMap); err != nil {
		return bytes.NewReader(requestBody), nil
	}

	// 模型名映射
	if info.ChannelMeta.IsModelMapped {
		rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
	}

	result, err := json.Marshal(rawMap)
	if err != nil {
		return bytes.NewReader(requestBody), nil
	}
	return bytes.NewReader(result), nil
}
