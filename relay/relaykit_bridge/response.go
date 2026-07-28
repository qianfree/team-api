package relaykit_bridge

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/logic/monitor"
	relaylogic "github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
)

// TryConvertResponseViaRelaykit 尝试用 relaykit 转换器转换非流式响应。
// 成功返回 (转换后的响应体, Usage, true)；开关关闭 / 无匹配 / 转换失败返回 (nil, nil, false)。
func TryConvertResponseViaRelaykit(ctx context.Context, info *common.RelayInfo, upstreamBody []byte) ([]byte, *dto.Usage, bool) {
	if info == nil || info.ChannelMeta == nil {
		return nil, nil, false
	}

	providerKey := providerKeyForChannelType(info.ChannelMeta.ChannelType)
	if providerKey == "" || !relaylogic.IsRelaykitEnabledForChannel(ctx, providerKey) {
		return nil, nil, false
	}

	// 响应转换方向：上游格式 → 客户端格式（与请求相反）
	upstream := providerNativeFormat(info.ChannelMeta.ChannelType)
	clientFormat := info.GetOriginalClientFormat()
	converterID := relaykitResponseConverterID(upstream, clientFormat)
	if converterID == "" {
		return nil, nil, false
	}

	spec, ok := relayconvert.LookupTextConverter(converterID)
	if !ok || spec.Resp.Convert == nil {
		return nil, nil, false
	}

	// 解析上游响应体为对应 DTO（Claude → dto.ClaudeResponse；Gemini → dto.GeminiChatResponse）
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
	default:
		return ""
	}
}

// providerKeyForChannelType 将渠道 ProviderType 映射为 relaykit 特性开关白名单使用的供应商 key。
func providerKeyForChannelType(channelType int) string {
	switch constant.ProviderType(channelType) {
	case constant.ProviderClaude:
		return "claude"
	case constant.ProviderGemini:
		return "gemini"
	case constant.ProviderOpenAI:
		return "openai"
	default:
		return ""
	}
}

// providerNativeFormat 返回供应商的原生协议格式（passthrough.go 同名函数的桥接版本）
func providerNativeFormat(providerType int) constant.RelayFormat {
	switch constant.ProviderType(providerType) {
	case constant.ProviderClaude:
		return constant.RelayFormatClaude
	case constant.ProviderGemini:
		return constant.RelayFormatGemini
	default:
		return constant.RelayFormatOpenAI
	}
}
