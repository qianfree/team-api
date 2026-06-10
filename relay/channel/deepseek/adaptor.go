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
// Completions(FIM) 模式使用 /beta/completions 端点，其余走标准 OpenAI 路径。
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
		return baseURL + "/v1/chat/completions", nil
	case constant.RelayModeClaudeMessages:
		return baseURL + "/anthropic/v1/messages", nil
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
// Claude 入站直接透传到 Anthropic 兼容端点，其他格式先转为 OpenAI 再做 DeepSeek 特有适配。
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	// Claude 入站：透传请求体，仅做模型映射和思考参数注入
	if info.InboundFormat == constant.RelayFormatClaude {
		return convertClaudeRequestForDeepSeek(requestBody, info)
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

	// 模型名映射
	if info.ChannelMeta.IsModelMapped {
		rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
	}

	// 注入 stream_options（流式请求需要 usage 信息用于计费）
	rawMap = injectStreamOptions(rawMap, info)

	// 注入思考模式参数
	rawMap = injectThinkingParams(rawMap, info)

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

// injectThinkingParams 根据 RelayInfo 中的思考后缀注入 DeepSeek 思考模式参数。
//
// DeepSeek 两类模型的参数差异：
//   - V3 思考版（deepseek-chat）：通过 thinking.type: "enabled"/"disabled" 控制；启用时需要 budget_tokens
//   - R1 系列（deepseek-reasoner）：不识别 thinking 参数，通过 reasoning_effort: "none"/"low"/"high"/"max" 控制
//
// 关闭思考时同时注入两个参数，兼容两类模型：
//   - thinking.type: "disabled"（V3 识别）
//   - reasoning_effort: "none"（R1 识别）
//
// 注入优先级：客户端已显式设置 > 后缀路由注入 > 不干预（保留 DeepSeek 默认行为）
func injectThinkingParams(rawMap map[string]json.RawMessage, info *common.RelayInfo) map[string]json.RawMessage {
	// 客户端已显式设置 thinking 参数 → 不干预
	if _, clientSet := rawMap["thinking"]; clientSet {
		return rawMap
	}

	// -nothinking 后缀：显式关闭思考
	// V3 模型：thinking.type: "disabled" 生效
	// R1 模型：thinking 参数被忽略，需 reasoning_effort: "none" 才能关闭思考
	if info.ThinkingDisabled {
		rawMap["thinking"] = json.RawMessage(`{"type":"disabled"}`)
		rawMap["reasoning_effort"] = json.RawMessage(`"none"`)
		return rawMap
	}

	// -thinking 后缀：显式开启思考 + 可选 effort
	// budget_tokens 为 V3 思考版必需字段，R1 模型忽略此参数
	if info.ThinkingEnabled {
		rawMap["thinking"] = json.RawMessage(`{"type":"enabled","budget_tokens":16000}`)
		rawMap = injectReasoningEffort(rawMap, info)
		return rawMap
	}

	// effort 后缀（-high/-low/-medium 等）：开启思考 + 设置 effort
	if info.ReasoningEffort != "" {
		// 仅在客户端未显式设置 thinking 时注入
		rawMap["thinking"] = json.RawMessage(`{"type":"enabled"}`)
		rawMap = injectReasoningEffort(rawMap, info)
		return rawMap
	}

	// 无后缀：不干预，保留 DeepSeek 默认行为（默认 enabled）
	return rawMap
}

// injectReasoningEffort 注入 reasoning_effort 参数，映射到 DeepSeek 支持的值
//
// DeepSeek V3/R1: 支持 high 和 max，映射 low/medium→high, xhigh→max
// DeepSeek V4: 仅支持 max，所有非空 effort 统一映射为 max
func injectReasoningEffort(rawMap map[string]json.RawMessage, info *common.RelayInfo) map[string]json.RawMessage {
	if info.ReasoningEffort == "" {
		return rawMap
	}
	// 仅在客户端未显式设置时注入
	if _, clientSet := rawMap["reasoning_effort"]; clientSet {
		return rawMap
	}

	// V4 模型仅支持 max
	if isDeepSeekV4Model(info.ChannelMeta.UpstreamModelName) {
		rawMap["reasoning_effort"] = json.RawMessage(`"max"`)
		return rawMap
	}

	// V3/R1: 支持 high 和 max，做兼容映射
	effort := info.ReasoningEffort
	switch effort {
	case "low", "medium", "minimal":
		effort = "high"
	case "xhigh", "max":
		effort = "max"
	default:
		effort = "high"
	}
	rawMap["reasoning_effort"], _ = json.Marshal(effort)
	return rawMap
}

// isDeepSeekV4Model 判断是否为 DeepSeek V4 系列模型
func isDeepSeekV4Model(modelName string) bool {
	return strings.HasPrefix(modelName, "deepseek-v4") ||
		strings.HasPrefix(modelName, "deepseek_v4")
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

	rawMap = injectThinkingParams(rawMap, info)

	result, err := json.Marshal(rawMap)
	if err != nil {
		return bytes.NewReader(requestBody), nil
	}
	return bytes.NewReader(result), nil
}
