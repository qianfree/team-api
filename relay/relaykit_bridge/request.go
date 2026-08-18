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
	"github.com/qianfree/team-api/relaykit/relayconvert"
)

// Claude/Gemini 入站 → OpenAI Chat 上游的请求侧接管入口（P1-A）。
//
// ⚠️ 接管位置约定（严禁在此处形成双通道）：
//   - claude/gemini 入站 → openai 上游方向的 relaykit 接管只发生在本入口，由共享函数
//     relay/channel/openai.ConvertToOpenAI 与 openai adaptor 的内联分支调用。
//     **严禁在 relay/handler 的 relaykitRequestConverterID 加同方向路由**——那会跳过
//     20+ 个 openai 兼容 adaptor 的定制后处理（volcengine 删 reasoning_effort、
//     tencent 参数截断等），造成行为回退。
//   - 后处理保持论断：各 adaptor 在 ConvertToOpenAI 之后消费的是转换输出的字节，
//     relaykit 输出同构 *dto.GeneralOpenAIRequest 的 marshal ⇒ 后处理照常执行，
//     本层无需吸收任何后处理逻辑（与 handler 层接管 Responses 方向的 P0 模式本质不同）。

// TryConvertInboundToOpenAIChat 尝试用 relaykit 转换器将 claude/gemini 入站请求体
// 转换为 OpenAI Chat 格式。成功返回 (转换后字节, true)；失败/未覆盖返回 (nil, false)，
// 调用方回退 legacy ConvertClaudeToOpenAI / ConvertGeminiToOpenAI。
func TryConvertInboundToOpenAIChat(ctx context.Context, info *common.RelayInfo, body []byte) ([]byte, bool) {
	if info == nil || info.ChannelMeta == nil {
		return nil, false
	}

	var converterID string
	switch info.InboundFormat {
	case constant.RelayFormatClaude:
		converterID = relayconvert.ConverterClaudeMessagesToOpenAIChat
	case constant.RelayFormatGemini:
		converterID = relayconvert.ConverterGeminiContentToOpenAIChat
	default:
		return nil, false
	}

	spec, ok := relayconvert.LookupRequestConverter(converterID)
	if !ok {
		return nil, false
	}

	var parsed any
	switch info.InboundFormat {
	case constant.RelayFormatClaude:
		var claudeReq dto.ClaudeRequest
		if err := json.Unmarshal(body, &claudeReq); err != nil {
			g.Log().Warningf(ctx, "[relaykit] parse claude request failed, fallback to legacy: %v", err)
			return nil, false
		}
		parsed = &claudeReq
	case constant.RelayFormatGemini:
		var geminiReq dto.GeminiChatRequest
		if err := json.Unmarshal(body, &geminiReq); err != nil {
			g.Log().Warningf(ctx, "[relaykit] parse gemini request failed, fallback to legacy: %v", err)
			return nil, false
		}
		parsed = &geminiReq
	}

	start := time.Now()
	converted, err := relayconvert.ExecuteRequestConverter(ctx, spec, info, parsed)
	duration := time.Since(start)
	monitor.TrackConverterCall(converterID, string(info.InboundFormat), string(constant.RelayFormatOpenAI), duration, err)
	if err != nil {
		g.Log().Warningf(ctx, "[relaykit] convert inbound request failed (converter=%s), fallback to legacy: %v", converterID, err)
		return nil, false
	}

	out, err := json.Marshal(converted)
	if err != nil {
		g.Log().Warningf(ctx, "[relaykit] marshal converted request failed (converter=%s), fallback to legacy: %v", converterID, err)
		return nil, false
	}
	return out, true
}
