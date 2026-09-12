package relaykit_bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/logic/monitor"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
)

// TryConvertResponseViaRelaykit 尝试用 relaykit 转换器转换非流式响应。
//
// 返回值语义（hard-fail 模型，无 legacy 回退）：
//   - handled=false, err=nil：该方向无 relaykit 转换器（同格式直连 / 未覆盖模式），
//     调用方走透传或自有处理；
//   - handled=true, err!=nil：方向匹配但解析/转换失败——协议转换是该方向的唯一路径，
//     调用方应向上返回错误（不再回退旧实现）；
//   - handled=true, err=nil：转换成功，converted 为客户端格式响应体。
//
// 结构与流式桥接对称：nil 守卫留在公开入口，转换逻辑抽到 config-free 的
// convertResponseViaRelaykit 核心以便单测直接覆盖。
func TryConvertResponseViaRelaykit(ctx context.Context, info *common.RelayInfo, upstreamBody []byte) (converted []byte, usage *dto.Usage, handled bool, err error) {
	if info == nil || info.ChannelMeta == nil {
		return nil, nil, false, nil
	}
	return convertResponseViaRelaykit(ctx, info, upstreamBody)
}

// convertResponseViaRelaykit 是非流式响应转换的 config-free 核心。
// info 与 info.ChannelMeta 必须非空。
func convertResponseViaRelaykit(ctx context.Context, info *common.RelayInfo, upstreamBody []byte) ([]byte, *dto.Usage, bool, error) {
	// 响应转换方向：上游格式 → 客户端格式（与请求相反）
	upstream := EffectiveUpstreamFormat(info)
	clientFormat := info.GetOriginalClientFormat()
	converterID := ResponseConverterIDForRoute(upstream, clientFormat)
	if converterID == "" {
		return nil, nil, false, nil
	}

	spec, ok := relayconvert.LookupTextConverter(converterID)
	if !ok || spec.Resp.Convert == nil {
		return nil, nil, false, nil
	}

	// 解析上游响应体为对应格式 DTO。
	upstreamResp, err := parseUpstreamResponse(upstream, upstreamBody)
	if err != nil {
		g.Log().Warningf(ctx, "[relaykit] parse %s response failed (converter=%s): %v", upstream, converterID, err)
		return nil, nil, true, fmt.Errorf("parse upstream %s response: %w", upstream, err)
	}

	start := time.Now()
	converted, usage, err := spec.Resp.Convert(ctx, info, upstreamResp)
	duration := time.Since(start)
	monitor.TrackConverterCall(converterID, string(upstream), string(clientFormat), duration, err)
	if err != nil {
		g.Log().Warningf(ctx, "[relaykit] convert response failed (converter=%s): %v", converterID, err)
		return nil, nil, true, fmt.Errorf("convert response (%s): %w", converterID, err)
	}

	out, err := json.Marshal(converted)
	if err != nil {
		g.Log().Warningf(ctx, "[relaykit] marshal converted response failed (converter=%s): %v", converterID, err)
		return nil, nil, true, fmt.Errorf("marshal converted response (%s): %w", converterID, err)
	}

	return out, usage, true, nil
}

// parseUpstreamResponse 按上游格式把响应体解析为对应 DTO 指针。
func parseUpstreamResponse(upstream constant.RelayFormat, body []byte) (any, error) {
	switch upstream {
	case constant.RelayFormatClaude:
		var claudeResp dto.ClaudeResponse
		if err := json.Unmarshal(body, &claudeResp); err != nil {
			return nil, err
		}
		return &claudeResp, nil
	case constant.RelayFormatGemini:
		var geminiResp dto.GeminiChatResponse
		if err := json.Unmarshal(body, &geminiResp); err != nil {
			return nil, err
		}
		return &geminiResp, nil
	case constant.RelayFormatOllama:
		var ollamaResp dto.OllamaChatResponse
		if err := json.Unmarshal(body, &ollamaResp); err != nil {
			return nil, err
		}
		return &ollamaResp, nil
	case constant.RelayFormatOpenAI:
		var chatResp dto.ChatCompletionResponse
		if err := json.Unmarshal(body, &chatResp); err != nil {
			return nil, err
		}
		return &chatResp, nil
	case constant.RelayFormatResponses:
		var responsesResp dto.OpenAIResponsesResponse
		if err := json.Unmarshal(body, &responsesResp); err != nil {
			return nil, err
		}
		return &responsesResp, nil
	default:
		return nil, fmt.Errorf("unsupported upstream format %q", upstream)
	}
}

// UsageFromConvertedChatResponse 从 relaykit 转换后的 OpenAI ChatCompletionResponse 响应体中提取用量。
// 非流式响应桥接成功后用于构建计费用量（转换器已把用量写入响应体的 Usage 字段）。
// 转换后的 usage 统一为 OpenAI 口径（prompt_tokens 含缓存，cached 为其子集），故置
// CacheIncludedInPrompt 让计费按明细扣减缓存部分，避免双重计费。
// 解析失败返回 (nil, false)，调用方可回退到从原始上游体提取或返回空用量。
func UsageFromConvertedChatResponse(body []byte) (*common.Usage, bool) {
	var resp dto.ChatCompletionResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, false
	}
	return &common.Usage{
		PromptTokens:           resp.Usage.PromptTokens,
		CompletionTokens:       resp.Usage.CompletionTokens,
		TotalTokens:            resp.Usage.TotalTokens,
		CacheCreationTokens:    convertedCacheCreationTokens(resp.Usage.PromptTokensDetails),
		CacheIncludedInPrompt:  true,
		PromptTokensDetails:    common.DtoTokenDetailsToCommon(resp.Usage.PromptTokensDetails),
		CompletionTokenDetails: common.DtoTokenDetailsToCommon(resp.Usage.CompletionTokenDetails),
	}, true
}

func convertedCacheCreationTokens(details *dto.TokenDetails) int {
	if details == nil {
		return 0
	}
	return details.CachedCreationTokens
}

// ResponseConvertError 把非流式响应转换错误映射为宿主错误类型。
//
// 与流式侧 streamConvertError 同一套分类依据，区别是非流式尚未写出任何字节，
// 因此不置 ResponseWritten（上层仍可写标准错误体，甚至换渠道重试）：
//   - 内容安全拦截（ErrContentBlocked）是客户端提示词问题，映射为请求类错误，
//     避免正常渠道因用户违规内容被扣健康分/熔断；
//   - 其余为上游响应体非法，沿用上游状态码与 Retry-After 头按上游错误上报。
func ResponseConvertError(convErr error, upstreamStatus int, header http.Header) error {
	if errors.Is(convErr, relayconvert.ErrContentBlocked) {
		return constant.NewRequestError(convErr.Error(), convErr)
	}
	return constant.NewUpstreamError(upstreamStatus, "invalid response body", convErr).
		WithRetryAfter(constant.RetryAfterFromHeader(header))
}
