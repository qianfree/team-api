package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
	"github.com/qianfree/team-api/relaykit/relayconvert"

	// blank import 触发内置转换器注册（register.init() 调用 RegisterTextConverter）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
)

// relaykit 请求转换器接入桥接层。
//
// 设计要点：
//   - 特性开关已移除（relaykit 常开）：relaykit 在其覆盖的转换方向上始终优先。
//   - 只替换「格式转换」这一步；转换后仍复用旧路径的系统提示词注入 / 参数改写 / 字段清理。
//   - X→openai(chat) 方向（claude/gemini/responses 入站）一律不在本层路由：接管点在共享
//     函数 openai.ConvertToOpenAI 内部，由各 adaptor 调用——保证 adaptor 的定制后处理
//     （deepseek thinking 注入、zhipu GLM 裁剪等）在任何入站格式下都照常执行。
//   - 无匹配（同格式 / Ollama 非 chat 模式等）返回 ok=false，调用方回退 adaptor.ConvertRequest
//     ——legacy 转换器已收割，该回退只覆盖原生直通与 adaptor 本地模式（Ollama generate/embedding）。
//     已匹配方向的解析/转换失败一律返回 error 显式报错，不回退（问题经 monitor.TrackConverterCall 暴露）。
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
	// Responses 入站（Responses/ResponsesCompact 模式均映射 responses 格式）。
	// ⚠️ responses→openai(chat-only 上游) 方向严禁在此路由：接管点在共享函数
	// openai.ConvertToOpenAI 内部（同 claude/gemini→openai 的既有约定）——handler 层
	// 接管会整体跳过 adaptor.ConvertRequest，deepseek injectThinkingParams、zhipu
	// applyGLMCompatibility、ali top_p 夹取、xai/baidu_v2 后缀注入等定制后处理全部失效。
	// Responses 原生上游（UpstreamSpeaksResponses）同样不路由，adaptor 走原样直连。
	if inbound == constant.RelayFormatResponses {
		switch {
		case upstream == constant.RelayFormatClaude:
			// Responses → Claude Messages（链式：responses→openai→claude）
			return relayconvert.ConverterOpenAIResponsesToClaudeMessages
		case upstream == constant.RelayFormatGemini:
			// Responses → Gemini（链式：responses→openai→gemini）
			return relayconvert.ConverterOpenAIResponsesToGemini
		default:
			return ""
		}
	}
	if inbound == upstream {
		return "" // 同格式无需转换（这类请求本就走 passthrough）
	}
	switch {
	// ⚠️ claude/gemini 入站 → openai 上游方向严禁在此路由：该方向的 relaykit 接管
	// 在共享函数 openai.ConvertToOpenAI 内部（relay/relaykit_bridge/request.go），
	// 在 handler 层接管会跳过 20+ 个 openai 兼容 adaptor 的定制后处理。
	case inbound == constant.RelayFormatGemini && upstream == constant.RelayFormatClaude:
		// Gemini → Claude Messages（链式：gemini→openai→claude）
		return relayconvert.ConverterGeminiContentToClaudeMessages
	case inbound == constant.RelayFormatClaude && upstream == constant.RelayFormatGemini:
		// Claude → Gemini（链式：claude→openai→gemini）
		return relayconvert.ConverterClaudeMessagesToGeminiContent
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
// 成功返回 (转换后的 io.Reader, true, nil)；无匹配返回 (nil, false, nil)，调用方回退
// adaptor 原生直通与本地模式；已匹配方向的解析/转换失败返回 error（显式报错不回退）；
// 有状态 responses 请求命中非 Responses 原生转换方向时返回哨兵错误
// ErrStatefulResponsesUnsupported（驱动调度 FSM 换渠道，不走 adaptor 旧路径）。
//
// 结构与流式桥接（relay/relaykit_bridge/stream.go）对称：nil 守卫留在公开入口，
// 真正的转换逻辑抽到 config-free 的 convertRequestViaRelaykit 核心以便单测直接覆盖。
func tryConvertRequestViaRelaykit(ctx context.Context, info *common.RelayInfo, body []byte) (io.Reader, bool, error) {
	if info == nil || info.ChannelMeta == nil {
		return nil, false, nil
	}
	return convertRequestViaRelaykit(ctx, info, body)
}

// convertRequestViaRelaykit 是请求转换的 config-free 核心（特性开关已由调用方校验）。
// info 与 info.ChannelMeta 必须非空。
func convertRequestViaRelaykit(ctx context.Context, info *common.RelayInfo, body []byte) (io.Reader, bool, error) {
	inbound := info.InboundFormat
	upstream := helper.ProviderNativeFormat(info.ChannelMeta.ChannelType)
	converterID := relaykitRequestConverterID(info, inbound, upstream, info.RelayMode)
	if converterID == "" {
		return nil, false, nil
	}

	// 经请求注册表查找（支持直接转换器与链式 spec；链式 spec 的 Req.Convert 为 nil
	// 由 ExecuteRequestConverter 逐跳执行）。路由返回的 ID 必然已注册——未命中说明
	// 路由表与注册表失配，显式报错而非静默回退
	reqSpec, ok := relayconvert.LookupRequestConverter(converterID)
	if !ok {
		return nil, false, fmt.Errorf("[relaykit] converter %q not registered (inbound=%s, upstream=%s)", converterID, inbound, upstream)
	}

	// 按入站格式解析请求体（转换器入参类型契约由此保证）。畸形请求体直接报错。
	// 解析与 Responses 预处理（哨兵 + stash）走共享入口（relaykit_bridge/inbound.go），
	// 与共享桥同源——哨兵错误经本函数返回，由 relay_handler 的哨兵判定点驱动 FSM 换渠道
	parsed, err := relaykit_bridge.ParseInboundRequest(ctx, inbound, body)
	if err != nil {
		return nil, false, fmt.Errorf("[relaykit] parse %s inbound request failed (converter=%s): %w", inbound, converterID, err)
	}
	if inbound == constant.RelayFormatResponses {
		if err := relaykit_bridge.PrepareResponsesInbound(info, parsed.(*dto.OpenAIResponsesRequest)); err != nil {
			return nil, false, err
		}
	}

	start := time.Now()
	converted, err := relayconvert.ExecuteRequestConverter(ctx, reqSpec, info, parsed)
	duration := time.Since(start)
	common.TrackConverterCall(converterID, string(inbound), string(upstream), duration, err)
	if err != nil {
		return nil, false, fmt.Errorf("[relaykit] convert request failed (converter=%s, %s→%s): %w", converterID, inbound, upstream, err)
	}

	out, err := json.Marshal(converted)
	if err != nil {
		return nil, false, fmt.Errorf("[relaykit] marshal converted request failed (converter=%s): %w", converterID, err)
	}

	return bytes.NewReader(out), true, nil
}
