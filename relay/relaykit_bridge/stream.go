package relaykit_bridge

import (
	"context"
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
)

// relaykit 流式响应转换器接入桥接层。
//
// 设计要点：
//   - 特性开关已移除（relaykit 常开）：relaykit 在其覆盖的转换方向上始终优先。
//   - 只替换「格式转换」这一步；SSE 帧化、保活 ping、[DONE] 收尾、StreamStatus 由本层负责。
//   - 任何「写入前」的放弃（无 ChannelMeta、同格式、无匹配转换器）都返回 ok=false，
//     调用方回退到旧 handleStreamToOpenAI 代码路径。
//   - 一旦 SetEventStreamHeaders 之后（开始写 chunk）即不可回退：
//     转换中途失败由本层写入结束 chunk + [DONE] + 设置 end reason 后返回 ok=true。
//   - 转换耗时与成败通过 monitor.TrackConverterCall 记录，供 dashboard 观测。

// TryConvertStreamViaRelaykit 尝试用 relaykit 流式转换器将上游 SSE 流转换为客户端格式。
//
// 返回值语义（三态）：
//   - ok=false, err=nil：未接管（nil info / 无 ChannelMeta / 同格式 / 无匹配转换器），
//     在任何 I/O 之前返回，调用方按自身策略回退或 hard-fail；
//   - ok=true, err=nil：转换成功，流已正常收尾；
//   - ok=true, err!=nil：已接管且 SSE 已写出，但流以错误结束。err 已带 ResponseWritten
//     语义（HTTP 头与 200 状态码既已提交，上层不得重写响应体、也不得换渠道重试），
//     调用方应原样 `return usage, err` 让 relay_handler 按「已送达 + 失败」记账与上报调度。
//
// usage 恒为从最后一个带 Usage 的 chunk 提取的用量（客户端断开时已叠加中断兜底估算）。
func TryConvertStreamViaRelaykit(ctx context.Context, info *common.RelayInfo, upstreamBody io.Reader, writer http.ResponseWriter) (*common.Usage, bool, error) {
	if info == nil || info.ChannelMeta == nil {
		return nil, false, nil
	}
	return convertStreamViaRelaykit(ctx, info, upstreamBody, writer)
}

// convertStreamViaRelaykit 流式转换核心：不读取特性开关配置（由公开入口保证），
// 抽离出来便于单测（参照 internal/logic/relay 中 isChannelInProviders 的纯函数抽离手法）。
func convertStreamViaRelaykit(ctx context.Context, info *common.RelayInfo, upstreamBody io.Reader, writer http.ResponseWriter) (*common.Usage, bool, error) {
	upstream := EffectiveUpstreamFormat(info)
	clientFormat := info.GetOriginalClientFormat()
	if upstream == clientFormat {
		return nil, false, nil // 同格式无需转换
	}

	fn, converterID, ok := relayconvert.LookupStreamConverter(KitFormat(upstream), KitFormat(clientFormat))
	if !ok {
		return nil, false, nil // 无匹配的 relaykit 流式转换器
	}

	// 收尾语义按客户端格式区分（与各方向旧实现一致）：
	//   - OpenAI 客户端：SSE 流以 data: [DONE] 结束，另需保证存在带
	//     finish_reason 的终止 chunk（转换器未产出时由本层补发）；
	//   - Gemini 客户端：真实 Gemini API 无 [DONE] 哨兵、流自然结束即收尾；
	//     官方 SDK（@google/genai）对每个 data 帧做 JSON.parse，写 [DONE] 会直接抛
	//     SyntaxError 导致客户端报错（gemini 原生透传路径也从不写 [DONE]，口径一致）；
	//   - Claude / Responses 客户端：终止语义由事件本身承载（message_stop /
	//     response.completed），不写 [DONE]，补帧也无意义；转换失败时分别补发
	//     error 事件 / response.failed（见错误分支）。
	needsDone := clientFormat == constant.RelayFormatOpenAI
	needsTerminalChunk := clientFormat == constant.RelayFormatOpenAI

	// 设置 SSE 头 + 并发安全 writer + 保活 ping（与旧 handleStreamToOpenAI 一致）
	helper.SetEventStreamHeaders(writer)
	safeWriter := helper.NewSafeWriter(writer)
	defer helper.PingTicker(safeWriter, 15*time.Second)()

	// relaykit 转换器输出的 usage 统一为 OpenAI 口径：prompt_tokens 已含缓存
	//（Gemini 的 cached ⊆ promptTokenCount；Claude 转换时已做 input+cache_read+cache_creation 加法）。
	// 置 CacheIncludedInPrompt 让计费按明细扣减缓存部分，避免「input 全价 + cache 价」双重计费
	capturedUsage := &common.Usage{CacheIncludedInPrompt: true}
	var (
		gotFinish           bool // 转换器是否已产出带 finish_reason 的结束 chunk
		gotResponseTerminal bool // Responses 客户端方向：转换器是否已产出终止态事件（completed/incomplete/failed）
		gotClaudeTerminal   bool // Claude 客户端方向：转换器是否已产出 message_stop
		firstChunk          bool
		writeFailed         bool // 写客户端已失败（连接不可达），客户端断开的可靠信号
		transferredTextLen  int  // 已转发的文本/思考内容长度，供流中断输出估算
	)

	// writeChatChunk：OpenAI chat chunk 的写出与状态追踪（Usage 提取、finish 追踪、
	// 中断估算文本长度）。池化缓冲 + 绑定 encoder：序列化与拼帧零分配，整帧单次写出
	//（帧原子，ping 插不进去）。
	writeChatChunk := func(streamChunk *dto.ChatCompletionStreamResponse) error {
		if !firstChunk {
			firstChunk = true
			info.SetFirstResponseTime()
		}
		if streamChunk.Usage != nil {
			capturedUsage.PromptTokens = streamChunk.Usage.PromptTokens
			capturedUsage.CompletionTokens = streamChunk.Usage.CompletionTokens
			capturedUsage.TotalTokens = streamChunk.Usage.TotalTokens
			capturedUsage.CacheCreationTokens = convertedCacheCreationTokens(streamChunk.Usage.PromptTokensDetails)
			capturedUsage.PromptTokensDetails = common.DtoTokenDetailsToCommon(streamChunk.Usage.PromptTokensDetails)
			capturedUsage.CompletionTokenDetails = common.DtoTokenDetailsToCommon(streamChunk.Usage.CompletionTokenDetails)
		}
		if len(streamChunk.Choices) > 0 && streamChunk.Choices[0].FinishReason != nil {
			gotFinish = true
		}
		for _, choice := range streamChunk.Choices {
			if text, ok := choice.Delta.Content.(string); ok {
				transferredTextLen += len(text)
			}
			if choice.Delta.ReasoningContent != nil {
				transferredTextLen += len(*choice.Delta.ReasoningContent)
			}
		}
		if err := helper.WriteSSEDataJSON(safeWriter, streamChunk); err != nil {
			// 序列化失败与写客户端失败必须分开：前者是转换器产出了不可序列化的 chunk，
			// 后者才说明客户端已不可达（writeFailed 驱动上层按流中断结算、不再补写 [DONE]）
			if !errors.Is(err, helper.ErrSSEPayloadMarshal) {
				writeFailed = true
			}
			return err
		}
		return nil
	}

	// chunkWriter：按 chunk 类型分派写出。
	//   - *dto.ChatCompletionStreamResponse：OpenAI 客户端方向，序列化为纯 data 帧；
	//   - *relayconvert.StreamEvent：非 OpenAI 客户端方向的通用封装，Event 非空写事件帧
	//     （Claude/Responses 风格），为空写纯 data 帧（Gemini 风格），Usage 非空即捕获。
	//     ToolCalls（服务端工具调用计数，计费旁路）先于 Data==nil 早退捕获——
	//     OpenAI 方向的转换器会发 Data 为 nil 的纯侧信道事件。
	chunkWriter := func(chunk any) error {
		switch c := chunk.(type) {
		case *dto.ChatCompletionStreamResponse:
			return writeChatChunk(c)
		case *relayconvert.StreamEvent:
			if c == nil {
				return nil
			}
			// 侧信道捕获先于 Data==nil 早退：转换器会发 Data 为空的纯侧信道事件
			//（服务端工具调用计数；流被掐断时已拿到的 usage），这些事件不写出但必须记账
			captureToolCalls(capturedUsage, c.ToolCalls)
			if c.Usage != nil {
				captureDtoUsage(capturedUsage, c.Usage)
			}
			if c.Data == nil {
				return nil
			}
			if !firstChunk {
				firstChunk = true
				info.SetFirstResponseTime()
			}
			switch c.Event {
			case "response.failed", "response.completed", "response.incomplete":
				gotResponseTerminal = true
			case "message_stop":
				gotClaudeTerminal = true
			}
			// 中断估算所需的已转发文本长度：此前只在 OpenAI chunk 路径累计，Claude / Responses
			// 客户端方向流中断时输出一律按 0 估算、只计输入
			transferredTextLen += streamEventTextLen(c)
			var err error
			if c.Event != "" {
				err = helper.WriteSSEEventJSON(safeWriter, c.Event, c.Data)
			} else {
				err = helper.WriteSSEDataJSON(safeWriter, c.Data)
			}
			if err != nil {
				if !errors.Is(err, helper.ErrSSEPayloadMarshal) {
					writeFailed = true
				}
				return err
			}
			return nil
		default:
			return nil // 忽略非预期类型
		}
	}

	// writeTerminal 在转换器未产出结束 chunk 时补发一个终止 chunk，保证客户端正常收尾。
	writeTerminal := func() {
		stop := "stop"
		terminal := &dto.ChatCompletionStreamResponse{
			ID:      fmt.Sprintf("chatcmpl-%s", info.RequestID),
			Object:  "chat.completion.chunk",
			Model:   info.OriginModelName,
			Choices: []dto.StreamChoice{{Index: 0, FinishReason: &stop}},
		}
		_ = helper.WriteSSEDataJSON(safeWriter, terminal)
	}

	setEndReason := func(reason common.StreamEndReason, err error) {
		if info.StreamStatus != nil {
			info.StreamStatus.SetEndReason(reason, err)
		}
	}

	// 上游空闲超时：转换器用 bufio.Scanner 阻塞读 body，无法被打断；用 IdleTimeoutReader 在
	// 两帧间隔超过 streaming_timeout_seconds 时关闭 body 解锁读取。此前本路径没有任何超时
	//（流式 http.Client 无 Timeout、请求 ctx 无 deadline），上游假死时客户端被 ping 维持着无限挂起。
	if rc, ok := upstreamBody.(io.ReadCloser); ok {
		idle := helper.NewIdleTimeoutReader(rc, helper.StreamIdleTimeout(info))
		defer idle.Stop()
		upstreamBody = idle
	}

	start := time.Now()
	err := fn(ctx, info, upstreamBody, chunkWriter)
	duration := time.Since(start)
	monitor.TrackConverterCall(converterID, string(upstream), string(clientFormat), duration, err)

	if err != nil {
		// 客户端断开判定：ctx 已取消，或已出现过写客户端失败。后者兜住「写失败先于
		// ctx 取消被观察到」的竞态与断开信号经代理链路延迟传导的场景——此前仅凭
		// ctx.Err() 区分，这类情况会被误判为转换失败：跳过中断计费兜底、向死连接
		// 补写 [DONE]，最终按 0 token 成功结算。
		if writeFailed || ctx.Err() != nil {
			endErr := ctx.Err()
			if endErr == nil {
				endErr = err
			}
			// 客户端已不可达，不写 terminal/[DONE]
			setEndReason(common.StreamEndReasonClientGone, endErr)
			// 流中断计费兜底：输出缺失按已转发文本 2 字符/token 估算，输入用请求侧估算值补齐
			helper.ApplyInterruptedUsageFallback(info, capturedUsage, transferredTextLen)
			return capturedUsage, true, common.ErrStreamInterrupted
		}
		g.Log().Warningf(ctx, "[relaykit] convert stream failed (converter=%s): %v", converterID, err)
		// 结束原因按错误性质区分：上游空闲超时记 timeout（属流中断，按已转发部分结算）；
		// 其余（上游 error 事件 / 流被掐断 / 畸形流）记 error
		switch {
		case errors.Is(err, helper.ErrStreamIdleTimeout):
			setEndReason(common.StreamEndReasonTimeout, err)
			helper.ApplyInterruptedUsageFallback(info, capturedUsage, transferredTextLen)
		case errors.Is(err, relayconvert.ErrUpstreamStreamTruncated):
			// 上游未发完成信号即 EOF：客户端已收到部分输出，属流中断，按已转发部分结算
			setEndReason(common.StreamEndReasonUpstreamTruncated, err)
			helper.ApplyInterruptedUsageFallback(info, capturedUsage, transferredTextLen)
		default:
			setEndReason(common.StreamEndReasonError, err)
		}
		// 向客户端补发协议原生的终止事件。HTTP 200 + SSE 头早已提交，这是客户端能收到的
		// 唯一失败信号；缺了它，客户端要么把半截回答当完整答案（Claude Code 无 error 事件
		// 即视作连接自然结束），要么一直等终止态而挂起（codex）。
		switch clientFormat {
		case constant.RelayFormatOpenAI:
			// chat 流式协议没有独立的错误事件，约定是 data 行携带 error 对象（OpenAI 官方中途失败
			// 即如此，各语言 SDK 据此抛错）。此前补发 finish_reason=stop 的终止 chunk 等于假装正常结束。
			if !gotFinish {
				_ = helper.WriteSSEDataJSON(safeWriter, map[string]any{
					"error": map[string]any{
						"message": err.Error(),
						"type":    "upstream_error",
						"code":    chatStreamErrorCode(err),
					},
				})
			}
			_ = helper.WriteSSEData(safeWriter, "[DONE]")
		case constant.RelayFormatResponses:
			// 转换器已自行收尾的（如 Gemini 安全拦截发 completed 拒答）不得追加第二终止事件
			if !gotResponseTerminal {
				_ = helper.WriteSSEEventJSON(safeWriter, "response.failed", map[string]any{
					"type": "response.failed",
					"response": map[string]any{
						"id":     fmt.Sprintf("resp_%s", info.RequestID),
						"object": "response",
						"status": "failed",
						"error": map[string]any{
							"code":    "upstream_error",
							"message": err.Error(),
						},
					},
				})
			}
		case constant.RelayFormatClaude:
			// Anthropic 流式协议的错误事件：{"type":"error","error":{"type":...,"message":...}}。
			// 此前 Claude 客户端方向在转换失败时什么都不补，连接直接关闭。
			if !gotClaudeTerminal {
				_ = helper.WriteSSEEventJSON(safeWriter, "error", map[string]any{
					"type": "error",
					"error": map[string]any{
						"type":    claudeStreamErrorType(err),
						"message": err.Error(),
					},
				})
			}
		}
		return capturedUsage, true, streamConvertError(err)
	}

	if needsTerminalChunk && !gotFinish {
		writeTerminal()
	}
	if needsDone {
		_ = helper.WriteSSEData(safeWriter, "[DONE]")
	}
	setEndReason(common.StreamEndReasonDone, nil)
	return capturedUsage, true, nil
}

// streamConvertError 把转换器在流中途返回的业务错误映射为宿主错误类型。
//
// 一律置 ResponseWritten：走到这里 SetEventStreamHeaders 早已提交 200 与 SSE 头，
// 上层既不能改状态码、也不能换渠道重试（客户端已收到部分事件流）。
//
// 分类依据是错误性质而非状态码方便性：
//   - 内容安全拦截（ErrContentBlocked）是客户端提示词问题，映射为请求类错误，
//     避免正常渠道因用户违规内容被扣健康分/熔断；
//   - 上游空闲超时映射为 504，调度 FSM 按超时类故障处理；
//   - 其余（上游 error 事件、流被掐断、上游流畸形等）是上游故障，按 502 上游错误上报调度 FSM。
func streamConvertError(err error) error {
	if errors.Is(err, relayconvert.ErrContentBlocked) {
		reqErr := constant.NewRequestError(err.Error(), err)
		reqErr.ResponseWritten = true
		return reqErr
	}
	status := http.StatusBadGateway
	if errors.Is(err, helper.ErrStreamIdleTimeout) {
		status = http.StatusGatewayTimeout
	}
	upErr := constant.NewUpstreamError(status, err.Error(), err)
	upErr.ResponseWritten = true
	return upErr
}

// chatStreamErrorCode 转换中途失败时补发给 OpenAI chat 客户端的 error.code，便于客户端/排障区分成因。
func chatStreamErrorCode(err error) string {
	switch {
	case errors.Is(err, relayconvert.ErrUpstreamStreamTruncated):
		return "upstream_truncated"
	case errors.Is(err, helper.ErrStreamIdleTimeout):
		return "upstream_timeout"
	case errors.Is(err, relayconvert.ErrContentBlocked):
		return "content_filter"
	default:
		return "upstream_error"
	}
}

// claudeStreamErrorType 转换中途失败时补发给 Claude 客户端的 error.type。
// Anthropic SDK 按该字段分类与重试：内容拦截是请求问题（invalid_request_error），
// 其余上游故障用 api_error（SDK 视为服务端错误，可重试）。
func claudeStreamErrorType(err error) string {
	if errors.Is(err, relayconvert.ErrContentBlocked) {
		return "invalid_request_error"
	}
	return "api_error"
}

// streamEventTextLen 提取一帧 StreamEvent 中已转发给客户端的文本/思考增量长度，供流中断时
// 按 2 字符/token 估算输出用量。只认增量事件：Responses 的 output_text / reasoning_summary_text
// delta（map 负载）与 Claude 的 content_block_delta（text_delta / thinking_delta）；
// done / 终止事件携带的是全文重复，不计入。
func streamEventTextLen(c *relayconvert.StreamEvent) int {
	switch c.Event {
	case "response.output_text.delta", "response.reasoning_summary_text.delta":
		if m, ok := c.Data.(map[string]any); ok {
			if delta, ok := m["delta"].(string); ok {
				return len(delta)
			}
		}
	case "content_block_delta":
		if resp, ok := c.Data.(*dto.ClaudeResponse); ok && resp.Delta != nil {
			n := 0
			if resp.Delta.Text != nil {
				n += len(*resp.Delta.Text)
			}
			if resp.Delta.Thinking != nil {
				n += len(*resp.Delta.Thinking)
			}
			return n
		}
	}
	return 0
}

// captureDtoUsage 把 StreamEvent 携带的带明细 Usage 累计进宿主 Usage（后到覆盖先到）。
func captureDtoUsage(dst *common.Usage, u *dto.UsageWithDetails) {
	dst.PromptTokens = u.PromptTokens
	dst.CompletionTokens = u.CompletionTokens
	dst.TotalTokens = u.TotalTokens
	dst.CacheCreationTokens = convertedCacheCreationTokens(u.PromptTokensDetails)
	dst.PromptTokensDetails = common.DtoTokenDetailsToCommon(u.PromptTokensDetails)
	dst.CompletionTokenDetails = common.DtoTokenDetailsToCommon(u.CompletionTokenDetails)
}

// captureToolCalls 把转换器上报的服务端工具调用计数并入宿主 Usage。
// 同名工具取最大值：厂商累计报数（如 Claude server_tool_use）为替换语义，
// 流式多次上报时最终帧的累计值 ≥ 中间帧；协议层增量计数由转换器在内部
// 归一为最终总数后上报，max 同样成立（防止异常重发导致重复累加）
func captureToolCalls(dst *common.Usage, calls map[string]int) {
	if len(calls) == 0 {
		return
	}
	for tool, count := range calls {
		if count > dst.ToolCalls[tool] {
			if dst.ToolCalls == nil {
				dst.ToolCalls = make(map[string]int)
			}
			dst.ToolCalls[tool] = count
		}
	}
}
