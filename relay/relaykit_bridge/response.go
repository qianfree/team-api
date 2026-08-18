package relaykit_bridge

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/logic/monitor"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relaykit/relayconvert"
)

// TryConvertResponseViaRelaykit 尝试用 relaykit 转换器转换非流式响应。
// 成功返回 (转换后的响应体, Usage, true)；无匹配 / 转换失败返回 (nil, nil, false)。
//
// 结构与流式桥接对称：nil 守卫留在公开入口，转换逻辑抽到 config-free 的
// convertResponseViaRelaykit 核心以便单测直接覆盖。
func TryConvertResponseViaRelaykit(ctx context.Context, info *common.RelayInfo, upstreamBody []byte) ([]byte, *dto.Usage, bool) {
	if info == nil || info.ChannelMeta == nil {
		return nil, nil, false
	}
	return convertResponseViaRelaykit(ctx, info, upstreamBody)
}

// convertResponseViaRelaykit 是非流式响应转换的 config-free 核心（特性开关已由调用方校验）。
// info 与 info.ChannelMeta 必须非空。
func convertResponseViaRelaykit(ctx context.Context, info *common.RelayInfo, upstreamBody []byte) ([]byte, *dto.Usage, bool) {
	// 响应转换方向：上游格式 → 客户端格式（与请求相反）
	upstream := helper.ProviderNativeFormat(info.ChannelMeta.ChannelType)
	clientFormat := info.GetOriginalClientFormat()
	converterID := relaykitResponseConverterID(upstream, clientFormat)
	if converterID == "" {
		return nil, nil, false
	}

	spec, ok := relayconvert.LookupTextConverter(converterID)
	if !ok || spec.Resp.Convert == nil {
		return nil, nil, false
	}

	// 解析上游响应体为对应 DTO（Claude → dto.ClaudeResponse；Gemini → dto.GeminiChatResponse；
	// Dify → dto.DifyBlockingResponse；Ollama → dto.OllamaChatResponse）。
	// Coze 上游始终为 SSE（非流式客户端也走流式），宿主已缓冲整段 SSE，直接把原始 []byte 交给转换器解析。
	var upstreamResp any
	switch upstream {
	case constant.RelayFormatClaude:
		var claudeResp dto.ClaudeResponse
		if err := json.Unmarshal(upstreamBody, &claudeResp); err != nil {
			g.Log().Warningf(ctx, "[relaykit] parse Claude response failed, fallback to legacy: %v", err)
			return nil, nil, false
		}
		upstreamResp = &claudeResp
	case constant.RelayFormatGemini:
		var geminiResp dto.GeminiChatResponse
		if err := json.Unmarshal(upstreamBody, &geminiResp); err != nil {
			g.Log().Warningf(ctx, "[relaykit] parse Gemini response failed, fallback to legacy: %v", err)
			return nil, nil, false
		}
		upstreamResp = &geminiResp
	case constant.RelayFormatCoze:
		// 原始缓冲 SSE 体，由 CozeToOpenAIResponseConverter 解析
		upstreamResp = upstreamBody
	case constant.RelayFormatDify:
		var difyResp dto.DifyBlockingResponse
		if err := json.Unmarshal(upstreamBody, &difyResp); err != nil {
			g.Log().Warningf(ctx, "[relaykit] parse Dify response failed, fallback to legacy: %v", err)
			return nil, nil, false
		}
		upstreamResp = &difyResp
	case constant.RelayFormatOllama:
		var ollamaResp dto.OllamaChatResponse
		if err := json.Unmarshal(upstreamBody, &ollamaResp); err != nil {
			g.Log().Warningf(ctx, "[relaykit] parse Ollama response failed, fallback to legacy: %v", err)
			return nil, nil, false
		}
		upstreamResp = &ollamaResp
	case constant.RelayFormatOpenAI:
		// P1-B：openai 上游 → claude/gemini 客户端（ChatCompletionResponse → 客户端格式）
		var chatResp dto.ChatCompletionResponse
		if err := json.Unmarshal(upstreamBody, &chatResp); err != nil {
			g.Log().Warningf(ctx, "[relaykit] parse chat response failed, fallback to legacy: %v", err)
			return nil, nil, false
		}
		upstreamResp = &chatResp
	default:
		return nil, nil, false
	}

	start := time.Now()
	converted, usage, err := spec.Resp.Convert(ctx, info, upstreamResp)
	duration := time.Since(start)
	monitor.TrackConverterCall(converterID, string(upstream), string(clientFormat), duration, err)
	if err != nil {
		g.Log().Warningf(ctx, "[relaykit] convert response failed (converter=%s), fallback to legacy: %v", converterID, err)
		return nil, nil, false
	}

	out, err := json.Marshal(converted)
	if err != nil {
		g.Log().Warningf(ctx, "[relaykit] marshal converted response failed (converter=%s), fallback to legacy: %v", converterID, err)
		return nil, nil, false
	}

	return out, usage, true
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

// relaykitResponseConverterID 根据 (上游原生格式, 客户端格式) 返回响应转换器 ID。
// 返回空串表示没有匹配的 relaykit 响应转换器（调用方回退旧路径）。
func relaykitResponseConverterID(upstream, clientFormat constant.RelayFormat) string {
	if upstream == clientFormat {
		return "" // 同格式无需转换
	}
	switch {
	case upstream == constant.RelayFormatClaude && clientFormat == constant.RelayFormatOpenAI:
		return relayconvert.ConverterOpenAIChatToClaudeMessages // 同一个 spec，响应侧是反向（Claude→OpenAI）
	case upstream == constant.RelayFormatGemini && clientFormat == constant.RelayFormatOpenAI:
		return relayconvert.ConverterOpenAIChatToGeminiContent // 同理，响应侧是 Gemini→OpenAI
	case upstream == constant.RelayFormatCoze && clientFormat == constant.RelayFormatOpenAI:
		return relayconvert.ConverterOpenAIChatToCoze // 响应侧是 Coze→OpenAI
	case upstream == constant.RelayFormatDify && clientFormat == constant.RelayFormatOpenAI:
		return relayconvert.ConverterOpenAIChatToDify // 响应侧是 Dify→OpenAI
	case upstream == constant.RelayFormatOllama && clientFormat == constant.RelayFormatOpenAI:
		return relayconvert.ConverterOpenAIChatToOllama // 响应侧是 Ollama→OpenAI
	// P1-B：openai 上游 → claude/gemini 客户端（Claude Code/Gemini 客户端打 openai 兼容渠道）
	case upstream == constant.RelayFormatOpenAI && clientFormat == constant.RelayFormatClaude:
		return relayconvert.ConverterClaudeMessagesToOpenAIChat // spec A，Resp 侧反向（openai→claude）
	case upstream == constant.RelayFormatOpenAI && clientFormat == constant.RelayFormatGemini:
		return relayconvert.ConverterGeminiContentToOpenAIChat // spec B，Resp 侧反向（openai→gemini）
	default:
		return ""
	}
}
