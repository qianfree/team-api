package relaykit_bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/logic/monitor"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
)

// relaykit →Responses 方向的响应侧桥接（Claude/openai chat 上游 × 非流式/流式）。
//
// 与 stream.go/response.go 的差异：客户端格式为 Responses（异构 SSE 事件，非
// ChatCompletionStreamResponse chunk），需要独立的流式桥接入口与 usage 提取逻辑；
// 写出格式为 `event: <type>\ndata: <json>\n\n`（对齐 openai.EmitResponsesSSE），
// 且不写 [DONE]（Responses 客户端以 response.completed 收尾，不期待 [DONE]）。
//
// ===================== 已知差异清单（P0/P1-R 收编，评审时逐条确认） =====================
//  1. 计费口径：旧路径返回上游原生口径（Claude 方向 input 不含缓存），本桥接统一为
//     OpenAI 口径（input 含缓存，CacheIncludedInPrompt=true 由计费侧扣减）——金额等价、
//     明细口径变化；B 方向（ChatViaResponses）非流式旧路径漏设该标志的自相矛盾已顺带修正。
//  2. transferredTextLen（流式中断兜底估算）含 reasoning delta——旧路径 A 方向仅计纯文本，
//     仅影响「上游无 usage 且流中断」的兜底计费（略高估）。
//  3. SetFirstResponseTime 时机：首事件发出时（旧路径为首个 data 行，含解析失败行）。
//  4. 响应对象为 typed DTO：恒含 prompt/conversation null 两键（旧路径 map 不含；
//     官方 Responses API 本就含这两个 null 字段，语义等价）。
//  5. completed_at 与 created_at 差恒 0（确定性修复项；旧路径为 +1 或真实耗时）。
//  6. 多工具 done 事件与 completed output 按登记顺序、重复 finish 不重复发 done
//     （确定性修复项；旧路径 map 遍历顺序随机且可能重复）。
//  7. usage details 键恒存在（含零值）——codex 严格解析必需，2026-08-18 修复后固化。

// relaykitResponsesResponseConverterID 返回 X 上游 → Responses 客户端的响应转换器 ID。
// 返回空串表示无匹配（调用方回退旧路径）。info 参与 openai 上游的 responses 能力守卫：
// 上游原生支持 Responses 时不转换（adaptor 走原样直连 + 后处理）。
func relaykitResponsesResponseConverterID(info *common.RelayInfo, upstream, clientFormat constant.RelayFormat) string {
	if clientFormat != constant.RelayFormatResponses {
		return ""
	}
	switch upstream {
	case constant.RelayFormatClaude:
		// 复用 responses→claude 链 spec 的 Resp 侧（方向相反，先例同 ConverterOpenAIChatToClaudeMessages）
		return relayconvert.ConverterOpenAIResponsesToClaudeMessages
	case constant.RelayFormatOpenAI:
		if !info.ChannelMeta.UpstreamSpeaksResponses() {
			return relayconvert.ConverterOpenAIResponsesToOpenAIChat
		}
		return ""
	default:
		return ""
	}
}

// parseUpstreamResponse 按上游格式解析响应体（多态入参，供非流式转换）。
// 解析失败返回 nil（调用方回退旧路径）。
func parseUpstreamResponse(ctx context.Context, upstream constant.RelayFormat, body []byte) any {
	switch upstream {
	case constant.RelayFormatClaude:
		var claudeResp dto.ClaudeResponse
		if err := json.Unmarshal(body, &claudeResp); err != nil {
			g.Log().Warningf(ctx, "[relaykit] parse Claude response failed, fallback to legacy: %v", err)
			return nil
		}
		return &claudeResp
	case constant.RelayFormatOpenAI:
		var chatResp dto.ChatCompletionResponse
		if err := json.Unmarshal(body, &chatResp); err != nil {
			g.Log().Warningf(ctx, "[relaykit] parse chat response failed, fallback to legacy: %v", err)
			return nil
		}
		return &chatResp
	default:
		return nil
	}
}

// TryConvertResponsesResponseViaRelaykit 尝试用 relaykit 转换器将上游非流式响应体
// （Claude 或 OpenAI Chat 上游）转换为 Responses 格式。成功返回 (响应体, 计费 usage, true)；
// 失败返回 (nil, nil, false)，调用方回退旧内联转换逻辑。
func TryConvertResponsesResponseViaRelaykit(ctx context.Context, info *common.RelayInfo, upstreamBody []byte) ([]byte, *common.Usage, bool) {
	if info == nil || info.ChannelMeta == nil {
		return nil, nil, false
	}
	upstream := helper.ProviderNativeFormat(info.ChannelMeta.ChannelType)
	clientFormat := info.GetOriginalClientFormat()
	converterID := relaykitResponsesResponseConverterID(info, upstream, clientFormat)
	if converterID == "" {
		return nil, nil, false
	}

	spec, ok := relayconvert.LookupTextConverter(converterID)
	if !ok || spec.Resp.Convert == nil {
		return nil, nil, false
	}

	upstreamResp := parseUpstreamResponse(ctx, upstream, upstreamBody)
	if upstreamResp == nil {
		return nil, nil, false
	}

	start := time.Now()
	converted, _, err := spec.Resp.Convert(ctx, info, upstreamResp)
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

	usage := usageFromResponsesBody(out)
	return out, usage, true
}

// TryConvertChatViaResponsesResponseViaRelaykit 尝试用 relaykit 转换器将 Responses 上游
// 非流式响应体转换为 chat 格式（ChatViaResponses 渠道的响应侧）。
// 门控 info.UseResponsesAPI 而非 (upstream, clientFormat) 格式路由——该场景两者同为
// openai，与「无需转换」同形（请求侧桥接 UseResponsesAPI 特判的同款先例）。
// 成功返回 (响应体, 计费 usage, true)；失败返回 (nil, nil, false)，调用方回退旧逻辑。
func TryConvertChatViaResponsesResponseViaRelaykit(ctx context.Context, info *common.RelayInfo, upstreamBody []byte) ([]byte, *common.Usage, bool) {
	if info == nil || info.ChannelMeta == nil || !info.UseResponsesAPI {
		return nil, nil, false
	}
	clientFormat := info.GetOriginalClientFormat()
	if clientFormat != constant.RelayFormatOpenAI {
		return nil, nil, false
	}

	spec, ok := relayconvert.LookupTextConverter(relayconvert.ConverterOpenAIChatToOpenAIResponses)
	if !ok || spec.Resp.Convert == nil {
		return nil, nil, false
	}

	var responsesResp dto.OpenAIResponsesResponse
	if err := json.Unmarshal(upstreamBody, &responsesResp); err != nil {
		g.Log().Warningf(ctx, "[relaykit] parse responses body failed, fallback to legacy: %v", err)
		return nil, nil, false
	}

	start := time.Now()
	converted, _, err := spec.Resp.Convert(ctx, info, &responsesResp)
	duration := time.Since(start)
	monitor.TrackConverterCall(relayconvert.ConverterOpenAIChatToOpenAIResponses,
		string(constant.RelayFormatResponses), string(clientFormat), duration, err)
	if err != nil {
		g.Log().Warningf(ctx, "[relaykit] convert responses→chat failed, fallback to legacy: %v", err)
		return nil, nil, false
	}

	out, err := json.Marshal(converted)
	if err != nil {
		g.Log().Warningf(ctx, "[relaykit] marshal chat body failed, fallback to legacy: %v", err)
		return nil, nil, false
	}

	// 计费 usage 从转换后的 chat 体提取（CacheIncludedInPrompt=true——顺带修正 legacy
	// 非流式侧不设该标志、与流式侧 responsesUsageToCommon 自相矛盾的口径）
	usage, _ := UsageFromConvertedChatResponse(out)
	return out, usage, true
}

// usageFromResponsesBody 从转换后的 Responses 响应体提取计费 usage。
// 转换器写入的 usage 统一为 OpenAI 口径（prompt_tokens 已含缓存，cached 为其子集），
// 故置 CacheIncludedInPrompt 让计费按明细扣减缓存部分，避免双重计费。
func usageFromResponsesBody(body []byte) *common.Usage {
	var resp dto.OpenAIResponsesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return &common.Usage{CacheIncludedInPrompt: true}
	}
	usage := &common.Usage{CacheIncludedInPrompt: true}
	if resp.Usage == nil {
		return usage
	}
	usage.PromptTokens = resp.Usage.InputTokens
	usage.CompletionTokens = resp.Usage.OutputTokens
	usage.TotalTokens = resp.Usage.TotalTokens
	if d := resp.Usage.InputTokensDetails; d != nil {
		usage.PromptTokensDetails = &common.TokenDetails{
			CachedTokens:     d.CachedTokens,
			CacheWriteTokens: d.CacheWriteTokens,
			TextTokens:       d.TextTokens,
			AudioTokens:      d.AudioTokens,
			ImageTokens:      d.ImageTokens,
		}
	}
	if d := resp.Usage.OutputTokenDetails; d != nil {
		usage.CompletionTokenDetails = &common.TokenDetails{
			TextTokens:               d.TextTokens,
			AudioTokens:              d.AudioTokens,
			ReasoningTokens:          d.ReasoningTokens,
			AcceptedPredictionTokens: d.AcceptedPredictionTokens,
			RejectedPredictionTokens: d.RejectedPredictionTokens,
		}
	}
	return usage
}

// TryConvertResponsesStreamViaRelaykit 尝试用 relaykit 流式转换器将 Claude SSE 转为 Responses SSE。
//
// 返回：
//   - usage：计费用量（OpenAI 口径，CacheIncludedInPrompt=true）
//   - ok：false 表示未接管（写入前放弃），调用方回退旧 handleStreamToResponses；
//     true 表示已接管（SSE 头可能已提交，不可再回退）
//   - err：接管后发生的上游/中断错误（透传给调用方，语义对齐旧路径的返回错误）；
//     未接管时恒为 nil
func TryConvertResponsesStreamViaRelaykit(ctx context.Context, info *common.RelayInfo, upstreamBody io.Reader, writer http.ResponseWriter) (*common.Usage, bool, error) {
	if info == nil || info.ChannelMeta == nil {
		return nil, false, nil
	}
	upstream := helper.ProviderNativeFormat(info.ChannelMeta.ChannelType)
	clientFormat := info.GetOriginalClientFormat()
	if relaykitResponsesResponseConverterID(info, upstream, clientFormat) == "" {
		return nil, false, nil
	}

	fn, streamID, ok := relayconvert.LookupStreamConverter(types.RelayFormat(upstream), types.RelayFormat(clientFormat))
	if !ok {
		return nil, false, nil
	}

	// 设置 SSE 头 + 并发安全 writer + 保活 ping（与旧 handleStreamToResponses 一致）
	helper.SetEventStreamHeaders(writer)
	safeWriter := helper.NewSafeWriter(writer)
	defer helper.PingTicker(safeWriter, 15*time.Second)()

	capturedUsage := &common.Usage{CacheIncludedInPrompt: true}
	var (
		firstEvent         bool
		sentCompleted      bool // 是否已发出 response.completed
		transferredTextLen int   // 已转发的文本/思考内容长度，供流中断输出估算
	)

	// emitResponsesEvent 写出一个 Responses SSE 事件，并提取 usage / 记录首字节时间。
	emitResponsesEvent := func(event *dto.ResponsesStreamEvent) error {
		if !firstEvent {
			firstEvent = true
			info.SetFirstResponseTime()
		}
		if data, ok := event.Data.(map[string]any); ok {
			switch event.Type {
			case "response.output_text.delta", "response.reasoning_summary_text.delta":
				if delta, ok := data["delta"].(string); ok {
					transferredTextLen += len(delta)
				}
			case "response.completed":
				sentCompleted = true
				if resp, ok := data["response"].(*dto.OpenAIResponsesResponse); ok && resp.Usage != nil {
					capturedUsage.PromptTokens = resp.Usage.InputTokens
					capturedUsage.CompletionTokens = resp.Usage.OutputTokens
					capturedUsage.TotalTokens = resp.Usage.TotalTokens
					if d := resp.Usage.InputTokensDetails; d != nil {
						capturedUsage.PromptTokensDetails = &common.TokenDetails{
							CachedTokens:     d.CachedTokens,
							CacheWriteTokens: d.CacheWriteTokens,
							AudioTokens:      d.AudioTokens,
						}
					}
				}
			}
		}
		data, err := json.Marshal(event.Data)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(safeWriter, "event: %s\ndata: %s\n\n", event.Type, string(data))
		safeWriter.Flush()
		return err
	}

	setEndReason := func(reason common.StreamEndReason, err error) {
		if info.StreamStatus != nil {
			info.StreamStatus.SetEndReason(reason, err)
		}
	}

	// 兜底估算：completed 携带的输出 token 缺失时按已转发文本估算
	finalizeUsage := func() {
		if capturedUsage.CompletionTokens == 0 && transferredTextLen > 0 {
			capturedUsage.CompletionTokens = helper.EstimateStreamOutputTokens(info, transferredTextLen)
			capturedUsage.TotalTokens = capturedUsage.PromptTokens + capturedUsage.CompletionTokens
		}
	}

	start := time.Now()
	err := fn(ctx, info, upstreamBody, func(chunk any) error {
		event, ok := chunk.(*dto.ResponsesStreamEvent)
		if !ok {
			return nil // 忽略非预期类型
		}
		return emitResponsesEvent(event)
	})
	duration := time.Since(start)
	monitor.TrackConverterCall(streamID, string(upstream), string(clientFormat), duration, err)

	if err != nil {
		if ctx.Err() != nil {
			// 客户端断开 / 上下文取消：客户端已不可达，不再写事件
			setEndReason(common.StreamEndReasonClientGone, ctx.Err())
			helper.ApplyInterruptedUsageFallback(info, capturedUsage, transferredTextLen)
			return capturedUsage, true, common.ErrStreamInterrupted
		}

		// 首个事件前 Header 尚未 flush（SetEventStreamHeaders 只 Set 未提交），
		// 可显式 WriteHeader 复刻 legacy 的状态码与错误体透传
		var embeddedErr *types.EmbeddedUpstreamError
		var mismatchErr = errors.Is(err, types.ErrProtocolMismatch)
		if mismatchErr || errors.As(err, &embeddedErr) {
			g.Log().Warningf(ctx, "[relaykit] convert responses stream failed (converter=%s): %v", streamID, err)
			if !firstEvent {
				safeWriter.Header().Set("Content-Type", "application/json")
				if mismatchErr {
					// 假成功防护：上游流不是预期协议格式，写 legacy 原文错误体（502）
					errBody, _ := json.Marshal(map[string]any{
						"error": map[string]any{
							"message": "upstream returned a non-chat-completions stream",
							"type":    "upstream_error",
							"param":   nil,
							"code":    "upstream_protocol_mismatch",
						},
					})
					safeWriter.WriteHeader(http.StatusBadGateway)
					_, _ = safeWriter.Write(errBody)
				} else {
					// SSE 内嵌上游错误：透传原文（对齐 legacy 透传行为，保留上游错误码结构）
					safeWriter.WriteHeader(http.StatusOK)
					_, _ = safeWriter.Write(embeddedErr.Body)
				}
				safeWriter.Flush()
			}
			finalizeUsage()
			setEndReason(common.StreamEndReasonError, err)
			status := http.StatusBadGateway
			body := err.Error()
			if embeddedErr != nil {
				status = http.StatusOK
				body = string(embeddedErr.Body)
			}
			upstreamErr := constant.NewUpstreamError(status, body, nil)
			if !firstEvent {
				upstreamErr.ResponseWritten = true
			}
			return capturedUsage, true, upstreamErr
		}

		g.Log().Warningf(ctx, "[relaykit] convert responses stream failed (converter=%s): %v", streamID, err)
		// 对齐旧路径：首个事件前出错则写 Responses 兼容错误体（SSE 头已提交，状态码已固定 200）
		if !firstEvent {
			errBody, _ := json.Marshal(map[string]any{
				"error": map[string]any{
					"message": err.Error(),
					"type":    "upstream_error",
					"param":   nil,
					"code":    "upstream_stream_error",
				},
			})
			safeWriter.Header().Set("Content-Type", "application/json")
			_, _ = safeWriter.Write(errBody)
			safeWriter.Flush()
		}
		finalizeUsage()
		setEndReason(common.StreamEndReasonError, err)
		upstreamErr := constant.NewUpstreamError(http.StatusBadGateway, err.Error(), nil)
		if !firstEvent {
			upstreamErr.ResponseWritten = true
		}
		return capturedUsage, true, upstreamErr
	}

	finalizeUsage()
	if !sentCompleted {
		setEndReason(common.StreamEndReasonError, fmt.Errorf("stream ended without response.completed"))
	} else if info.StreamStatus.GetEndReason() == "" {
		setEndReason(common.StreamEndReasonDone, nil)
	}
	return capturedUsage, true, nil
}
