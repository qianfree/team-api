package relaykit_bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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

// TryConvertInboundToOpenAIChat 尝试用 relaykit 转换器将 claude/gemini/responses 入站
// 请求体转换为 OpenAI Chat 格式。成功返回 (转换后字节, true, nil)；未覆盖的入站格式返回
// (nil, false, nil)；已覆盖方向的解析/转换失败返回 error（调用方 ConvertToOpenAI 直接
// 透传报错，不回退——legacy 回退已收割）。
// 有状态 responses 请求（previous_response_id）命中非 Responses 原生上游时返回哨兵错误
// ErrStatefulResponsesUnsupported（wrap），经 ConvertToOpenAI → adaptor → convertRequestBody
// 透传至 relay_handler 的哨兵判定点驱动 FSM 换渠道——与 handler 桥（claude/gemini 链）
// 的同构语义。
//
// responses 入站 × 一切非链式上游（openai 兼容 chat-only/ollama/coze/dify 等）的 r2o
// 转换均在此完成（handler 桥的 responses 路由只保留 claude/gemini 链式方向）——
// 接管点在 adaptor 层使各 compat adaptor 的定制后处理（deepseek injectThinkingParams、
// zhipu applyGLMCompatibility 等）照常执行（legacy ConvertResponsesToOpenAI 的收编）。
func TryConvertInboundToOpenAIChat(ctx context.Context, info *common.RelayInfo, body []byte) ([]byte, bool, error) {
	if info == nil || info.ChannelMeta == nil {
		return nil, false, nil
	}

	var converterID string
	switch info.InboundFormat {
	case constant.RelayFormatClaude:
		converterID = relayconvert.ConverterClaudeMessagesToOpenAIChat
	case constant.RelayFormatGemini:
		converterID = relayconvert.ConverterGeminiContentToOpenAIChat
	case constant.RelayFormatResponses:
		converterID = relayconvert.ConverterOpenAIResponsesToOpenAIChat
	default:
		return nil, false, nil
	}

	spec, ok := relayconvert.LookupRequestConverter(converterID)
	if !ok {
		return nil, false, fmt.Errorf("[relaykit] converter %q not registered", converterID)
	}

	// 解析与 Responses 预处理（哨兵 + stash）走共享入口（inbound.go），与 handler 桥同源
	parsed, err := ParseInboundRequest(ctx, info.InboundFormat, body)
	if err != nil {
		return nil, false, fmt.Errorf("[relaykit] parse %s inbound request failed: %w", info.InboundFormat, err)
	}
	if info.InboundFormat == constant.RelayFormatResponses {
		if err := PrepareResponsesInbound(info, parsed.(*dto.OpenAIResponsesRequest)); err != nil {
			return nil, false, err
		}
	}

	start := time.Now()
	converted, err := relayconvert.ExecuteRequestConverter(ctx, spec, info, parsed)
	duration := time.Since(start)
	monitor.TrackConverterCall(converterID, string(info.InboundFormat), string(constant.RelayFormatOpenAI), duration, err)
	if err != nil {
		return nil, false, fmt.Errorf("[relaykit] convert inbound request failed (converter=%s): %w", converterID, err)
	}

	out, err := json.Marshal(converted)
	if err != nil {
		return nil, false, fmt.Errorf("[relaykit] marshal converted request failed (converter=%s): %w", converterID, err)
	}
	return out, true, nil
}
