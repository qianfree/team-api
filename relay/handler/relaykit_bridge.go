package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/logic/monitor"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relaykit/relayconvert"

	// blank import 触发内置转换器注册（register.init() 调用 RegisterTextConverter）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
)

// relaykit 请求转换器接入桥接层。
//
// 设计要点：
//   - 特性开关已移除（relaykit 常开）：relaykit 在其覆盖的转换方向上始终优先。
//   - 只替换「格式转换」这一步；转换后仍复用旧路径的系统提示词注入 / 参数改写 / 字段清理。
//   - 任何失败（无匹配转换器、解析失败、转换失败、同格式、Ollama 非 chat 模式）都返回 ok=false，
//     调用方回退到 adaptor.ConvertRequest 旧代码路径，保证请求不因 relaykit 中断。
//   - 转换耗时与成败通过 monitor.TrackConverterCall 记录，供 dashboard 观测。

// relaykitRequestConverterID 根据 (RelayInfo, 客户端入站格式, 上游原生格式, RelayMode) 返回
// 已注册的请求转换器 ID。返回空串表示没有匹配的 relaykit 转换器（调用方回退旧路径）。
// Ollama 仅注册了 chat 路径转换器，其 generate/embedding 模式返回空串以回退旧 adaptor。
func relaykitRequestConverterID(info *common.RelayInfo, inbound, upstream constant.RelayFormat, relayMode int) string {
	// chat 客户端桥接到 Responses 上游（ChatViaResponses 渠道）：inbound 与 upstream 同为
	// openai（ProviderNativeFormat 不反映 responses 能力），必须在「同格式早退」之前判定，
	// 对应旧路径 openai adaptor 的 UseResponsesAPI 分支
	if info.UseResponsesAPI && constant.RelayMode(relayMode) == constant.RelayModeChatCompletions &&
		inbound == constant.RelayFormatOpenAI && upstream == constant.RelayFormatOpenAI {
		return relayconvert.ConverterOpenAIChatToOpenAIResponses
	}
	// Responses 入站（Responses/ResponsesCompact 模式均映射 responses 格式）
	if inbound == constant.RelayFormatResponses {
		switch {
		case upstream == constant.RelayFormatOpenAI && !info.ChannelMeta.UpstreamSpeaksResponses():
			// chat-only 上游：Responses → OpenAI Chat。
			// 上游原生支持 Responses 时不转换（adaptor 走原样直连 + 后处理）
			return relayconvert.ConverterOpenAIResponsesToOpenAIChat
		case upstream == constant.RelayFormatClaude:
			// Responses → Claude Messages（链式：responses→openai→claude）
			return relayconvert.ConverterOpenAIResponsesToClaudeMessages
		default:
			return ""
		}
	}
	if inbound == upstream {
		return "" // 同格式无需转换（这类请求本就走 passthrough）
	}
	switch {
	case inbound == constant.RelayFormatOpenAI && upstream == constant.RelayFormatClaude:
		return relayconvert.ConverterOpenAIChatToClaudeMessages
	case inbound == constant.RelayFormatOpenAI && upstream == constant.RelayFormatGemini:
		return relayconvert.ConverterOpenAIChatToGeminiContent
	case inbound == constant.RelayFormatOpenAI && upstream == constant.RelayFormatCoze:
		return relayconvert.ConverterOpenAIChatToCoze
	case inbound == constant.RelayFormatOpenAI && upstream == constant.RelayFormatDify:
		return relayconvert.ConverterOpenAIChatToDify
	case inbound == constant.RelayFormatOpenAI && upstream == constant.RelayFormatOllama:
		// 仅 chat 路径迁移；generate（completions）/ embedding 回退旧 adaptor
		if constant.RelayMode(relayMode) != constant.RelayModeChatCompletions {
			return ""
		}
		return relayconvert.ConverterOpenAIChatToOllama
	default:
		return ""
	}
}

// tryConvertRequestViaRelaykit 尝试用 relaykit 转换器转换请求体。
// 成功返回 (转换后的 io.Reader, true)；无匹配 / 解析或转换失败返回 (nil, false)。
//
// 结构与流式桥接（relay/relaykit_bridge/stream.go）对称：nil 守卫留在公开入口，
// 真正的转换逻辑抽到 config-free 的 convertRequestViaRelaykit 核心以便单测直接覆盖。
func tryConvertRequestViaRelaykit(ctx context.Context, info *common.RelayInfo, body []byte) (io.Reader, bool) {
	if info == nil || info.ChannelMeta == nil {
		return nil, false
	}
	return convertRequestViaRelaykit(ctx, info, body)
}

// convertRequestViaRelaykit 是请求转换的 config-free 核心（特性开关已由调用方校验）。
// info 与 info.ChannelMeta 必须非空。
func convertRequestViaRelaykit(ctx context.Context, info *common.RelayInfo, body []byte) (io.Reader, bool) {
	inbound := info.InboundFormat
	upstream := helper.ProviderNativeFormat(info.ChannelMeta.ChannelType)
	converterID := relaykitRequestConverterID(info, inbound, upstream, info.RelayMode)
	if converterID == "" {
		return nil, false
	}

	// 经请求注册表查找（支持直接转换器与链式 spec；链式 spec 的 Req.Convert 为 nil
	// 由 ExecuteRequestConverter 逐跳执行）
	reqSpec, ok := relayconvert.LookupRequestConverter(converterID)
	if !ok {
		return nil, false
	}

	// 按入站格式解析请求体（转换器入参类型契约由此保证）
	parsed, ok := parseInboundRequest(ctx, inbound, body)
	if !ok {
		return nil, false
	}

	// Responses 入站专属预处理：
	//   - 有状态请求（previous_response_id）回退旧路径，由 legacy ConvertResponsesToOpenAI
	//     返回哨兵错误驱动 failover（relaykit 转换器不做此检查）；
	//   - stash 请求快照，供响应侧合成 Responses 格式时 echo 请求参数（对应 legacy 行为）。
	if inbound == constant.RelayFormatResponses {
		responsesReq := parsed.(*dto.OpenAIResponsesRequest)
		if responsesReq.PreviousResponseID != "" {
			return nil, false
		}
		info.ResponsesRequest = responsesReq
	}

	start := time.Now()
	converted, err := relayconvert.ExecuteRequestConverter(ctx, reqSpec, info, parsed)
	duration := time.Since(start)
	monitor.TrackConverterCall(converterID, string(inbound), string(upstream), duration, err)
	if err != nil {
		g.Log().Warningf(ctx, "[relaykit] convert request failed (converter=%s), fallback to legacy: %v", converterID, err)
		return nil, false
	}

	out, err := json.Marshal(converted)
	if err != nil {
		g.Log().Warningf(ctx, "[relaykit] marshal converted request failed (converter=%s), fallback to legacy: %v", converterID, err)
		return nil, false
	}

	return bytes.NewReader(out), true
}

// parseInboundRequest 按入站格式将请求体解析为对应 DTO 指针。
// 解析失败记 Warning 并返回 false（调用方回退旧路径）。
func parseInboundRequest(ctx context.Context, inbound constant.RelayFormat, body []byte) (any, bool) {
	switch inbound {
	case constant.RelayFormatOpenAI:
		var openaiReq dto.GeneralOpenAIRequest
		if err := json.Unmarshal(body, &openaiReq); err != nil {
			g.Log().Warningf(ctx, "[relaykit] parse inbound request failed, fallback to legacy: %v", err)
			return nil, false
		}
		return &openaiReq, true
	case constant.RelayFormatResponses:
		var responsesReq dto.OpenAIResponsesRequest
		if err := json.Unmarshal(body, &responsesReq); err != nil {
			g.Log().Warningf(ctx, "[relaykit] parse inbound responses request failed, fallback to legacy: %v", err)
			return nil, false
		}
		return &responsesReq, true
	default:
		return nil, false
	}
}
