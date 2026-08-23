package relaykit_bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
)

// relaykit 流式响应转换器接入桥接层。
//
// 设计要点：
//   - 特性开关已移除（relaykit 常开）：relaykit 在其覆盖的转换方向上始终优先。
//   - 只替换「格式转换」这一步；SSE 帧化、保活 ping、[DONE] 收尾（openai 客户端）、StreamStatus 由本层负责。
//   - 任何「写入前」的放弃（无 ChannelMeta、同格式、无匹配转换器）都返回 ok=false，
//     调用方回退到旧 handleStreamToOpenAI 代码路径。
//   - 一旦 SetEventStreamHeaders 之后（开始写 chunk）即不可回退：
//     转换中途失败由本层按客户端格式写入结束事件（openai 客户端另补 [DONE]）+ 设置 end reason 后返回 ok=true。
//   - 转换耗时与成败通过 monitor.TrackConverterCall 记录，供 dashboard 观测。

// TryConvertStreamViaRelaykit 尝试用 relaykit 流式转换器将上游 SSE 流转换为客户端格式。
//
// 返回：
//   - usage：从最后一个带 Usage 的 chunk 提取的用量（无则零值 Usage）
//   - ok：true 表示已接管响应（成功或优雅失败），调用方应直接返回，不再走旧路径；
//     false 表示未接管（无 ChannelMeta / 同格式 / 无匹配转换器），调用方回退旧路径。
//
// 注意：未接管时在任何 I/O 之前即返回 false，旧路径行为零变化。
func TryConvertStreamViaRelaykit(ctx context.Context, info *common.RelayInfo, upstreamBody io.Reader, writer http.ResponseWriter) (*common.Usage, bool) {
	if info == nil || info.ChannelMeta == nil {
		return nil, false
	}
	return convertStreamViaRelaykit(ctx, info, upstreamBody, writer)
}

// convertStreamViaRelaykit 流式转换核心：不读取特性开关配置（由公开入口保证），
// 抽离出来便于单测（参照 internal/logic/relay 中 isChannelInProviders 的纯函数抽离手法）。
func convertStreamViaRelaykit(ctx context.Context, info *common.RelayInfo, upstreamBody io.Reader, writer http.ResponseWriter) (*common.Usage, bool) {
	clientFormat := info.GetOriginalClientFormat()
	upstream := bridgeUpstreamFormat(info, clientFormat)
	if upstream == clientFormat {
		return nil, false // 同格式无需转换
	}

	fn, converterID, ok := relayconvert.LookupStreamConverter(types.RelayFormat(upstream), types.RelayFormat(clientFormat))
	if !ok {
		return nil, false // 无匹配的 relaykit 流式转换器
	}

	// 设置 SSE 头 + 并发安全 writer + 保活 ping（与旧 handleStreamToOpenAI 一致）
	helper.SetEventStreamHeaders(writer)
	safeWriter := helper.NewSafeWriter(writer)
	defer helper.PingTicker(safeWriter, 15*time.Second)()

	// relaykit 转换器输出的 usage 统一为 OpenAI 口径：prompt_tokens 已含缓存
	//（Gemini 的 cached ⊆ promptTokenCount；Claude 转换时已做 input+cache_read+cache_creation 加法）。
	// 置 CacheIncludedInPrompt 让计费按明细扣减缓存部分，避免「input 全价 + cache 价」双重计费
	capturedUsage := &common.Usage{CacheIncludedInPrompt: true}
	var (
		gotFinish          bool // 转换器是否已产出带 finish_reason 的结束 chunk
		firstChunk         bool
		transferredTextLen int // 已转发的文本/思考/工具内容长度，供无 usage 与流中断的输出估算
	)

	// claudeClient / geminiClient：P2 扩展的客户端格式分派（openai 上游 → claude/gemini 客户端）。
	// 收尾约定：claude 客户端不写 [DONE]（message_stop 由转换器发，terminal 补发亦为 claude 事件）；
	// gemini 客户端不写 [DONE]（官方 Gemini SSE 无此哨兵，转换器以带 finishReason 的尾 chunk 收尾）；
	// openai 客户端写 data: [DONE]。
	claudeClient := clientFormat == constant.RelayFormatClaude
	geminiClient := clientFormat == constant.RelayFormatGemini

	// writeDoneMarker 按客户端格式写收尾标记
	writeDoneMarker := func() {
		if claudeClient || geminiClient {
			// Claude 以 message_stop 收尾；Gemini SSE（alt=sse）官方协议无 [DONE] 哨兵，
			// 直连路径也仅在上游发了 [DONE] 时才转发——两者均不补发
			return
		}
		_ = helper.WriteSSEData(safeWriter, "[DONE]")
	}

	// writeTerminalForClient 在转换器未产出结束事件时按客户端格式补发终止事件
	writeTerminalForClient := func() {
		if claudeClient {
			// 补 message_delta(stop) + message_stop，保证 Claude 客户端正常收尾
			stop := "end_turn"
			_, _ = fmt.Fprintf(safeWriter, "event: message_delta\ndata: %s\n\n",
				fmt.Sprintf(`{"type":"message_delta","delta":{"stop_reason":%q}}`, stop))
			_, _ = fmt.Fprintf(safeWriter, "event: message_stop\ndata: %s\n\n", `{"type":"message_stop"}`)
			safeWriter.Flush()
			return
		}
		if geminiClient {
			// 补 Gemini 格式收尾 chunk（candidates 带 finishReason=STOP，
			// 形状与 OpenAIToGeminiStreamConverter 的尾 chunk 一致）
			terminal := &dto.GeminiChatResponse{
				Candidates: []dto.GeminiCandidate{{
					Content:      &dto.GeminiContent{Role: "model", Parts: []dto.GeminiPart{}},
					FinishReason: "STOP",
				}},
			}
			data, _ := json.Marshal(terminal)
			_ = helper.WriteSSEData(safeWriter, string(data))
			return
		}
		// openai：补 chat 终止 chunk（[DONE] 由 writeDoneMarker 统一写）
		stop := "stop"
		terminal := &dto.ChatCompletionStreamResponse{
			ID:      fmt.Sprintf("chatcmpl-%s", info.RequestID),
			Object:  "chat.completion.chunk",
			Model:   info.OriginModelName,
			Choices: []dto.StreamChoice{{Index: 0, FinishReason: &stop}},
		}
		data, _ := json.Marshal(terminal)
		_ = helper.WriteSSEData(safeWriter, string(data))
	}

	// chunkWriter：按 chunk 类型三态分派——
	//   *dto.ChatCompletionStreamResponse（openai 客户端，现状）→ data: 行；
	//   *dto.ClaudeStreamEvent（claude 客户端，P2）→ event: + data: 行，message_delta 提取计费 usage；
	//   *dto.GeminiChatResponse（gemini 客户端，P2）→ data: 行，UsageMetadata 提取计费 usage。
	chunkWriter := func(chunk any) error {
		switch typed := chunk.(type) {
		case *dto.ClaudeStreamEvent:
			if !firstChunk {
				firstChunk = true
				info.SetFirstResponseTime()
			}
			if typed == nil || typed.Data == nil {
				return nil
			}
			if typed.Type == "message_delta" {
				// message_delta 携带 Claude 扣减口径 usage——还原为 OpenAI 计费口径
				//（prompt = input + cache_read，含 cache 子集，CacheIncludedInPrompt 已置）
				if u := typed.Data.Usage; u != nil {
					capturedUsage.PromptTokens = u.InputTokens + u.CacheReadInputTokens
					capturedUsage.CompletionTokens = u.OutputTokens
					capturedUsage.TotalTokens = capturedUsage.PromptTokens + capturedUsage.CompletionTokens
					if capturedUsage.PromptTokensDetails == nil {
						capturedUsage.PromptTokensDetails = &common.TokenDetails{}
					}
					capturedUsage.PromptTokensDetails.CachedTokens = u.CacheReadInputTokens
				}
				gotFinish = true
			}
			if typed.Type == "content_block_delta" && typed.Data.Delta != nil {
				if typed.Data.Delta.Text != nil {
					transferredTextLen += len(*typed.Data.Delta.Text)
				}
				if typed.Data.Delta.Thinking != nil {
					transferredTextLen += len(*typed.Data.Delta.Thinking)
				}
			}
			dataJSON, err := json.Marshal(typed.Data)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(safeWriter, "event: %s\ndata: %s\n\n", typed.Type, string(dataJSON))
			safeWriter.Flush()
			return err
		case *dto.GeminiChatResponse:
			if !firstChunk {
				firstChunk = true
				info.SetFirstResponseTime()
			}
			if typed == nil {
				return nil
			}
			if um := typed.UsageMetadata; um != nil {
				// Gemini 口径还原 OpenAI 计费口径：prompt 含 cached 子集；
				// completion = candidates + thoughts（2f0cc01 扣减的逆运算）
				capturedUsage.PromptTokens = um.PromptTokenCount
				capturedUsage.CompletionTokens = um.CandidatesTokenCount + um.ThoughtsTokenCount
				capturedUsage.TotalTokens = um.TotalTokenCount
				if capturedUsage.PromptTokensDetails == nil {
					capturedUsage.PromptTokensDetails = &common.TokenDetails{}
				}
				capturedUsage.PromptTokensDetails.CachedTokens = um.CachedContentTokenCount
				if capturedUsage.CompletionTokenDetails == nil {
					capturedUsage.CompletionTokenDetails = &common.TokenDetails{}
				}
				capturedUsage.CompletionTokenDetails.ReasoningTokens = um.ThoughtsTokenCount
			}
			for _, cand := range typed.Candidates {
				if cand.FinishReason != "" && cand.FinishReason != "FINISH_REASON_UNSPECIFIED" {
					gotFinish = true
				}
				if cand.Content != nil {
					for _, p := range cand.Content.Parts {
						transferredTextLen += len(p.Text)
					}
				}
			}
			dataJSON, err := json.Marshal(typed)
			if err != nil {
				return err
			}
			return helper.WriteSSEData(safeWriter, string(dataJSON))
		}
		streamChunk, ok := chunk.(*dto.ChatCompletionStreamResponse)
		if !ok {
			return nil // 忽略非预期类型
		}
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
			for _, tc := range choice.Delta.ToolCalls {
				// 工具名仅随首个 chunk 发出（转换器按 callID 去重），参数为增量——
				// 纯工具调用流的文本长度为 0，不计入则估算兜底恒为 0
				transferredTextLen += len(tc.Function.Name) + len(tc.Function.Arguments)
			}
		}
		data, err := json.Marshal(streamChunk)
		if err != nil {
			return err
		}
		return helper.WriteSSEData(safeWriter, string(data))
	}

	setEndReason := func(reason common.StreamEndReason, err error) {
		if info.StreamStatus != nil {
			info.StreamStatus.SetEndReason(reason, err)
		}
	}

	// 兜底估算（须在 setEndReason 之后调用——中断口径 2 字符/token 依赖结束原因已落位）：
	// 上游未返回 usage 时按已转发内容估算输出 token；流中断时输入 token 用请求侧估算值补齐。
	// 与宿主各流式路径（openai/stream.go、HandleResponsesStreamToChat）同口径。
	finalizeUsage := func() {
		if capturedUsage.TotalTokens == 0 && transferredTextLen > 0 {
			capturedUsage.CompletionTokens = helper.EstimateStreamOutputTokens(info, transferredTextLen)
			capturedUsage.TotalTokens = capturedUsage.PromptTokens + capturedUsage.CompletionTokens
		}
		helper.ApplyInterruptedUsageFallback(info, capturedUsage, transferredTextLen)
	}

	start := time.Now()
	err := fn(ctx, info, upstreamBody, chunkWriter)
	duration := time.Since(start)
	common.TrackConverterCall(converterID, string(upstream), string(clientFormat), duration, err)

	if err != nil {
		if ctx.Err() != nil {
			// 客户端断开 / 上下文取消：客户端已不可达，不写收尾标记
			setEndReason(common.StreamEndReasonClientGone, ctx.Err())
			finalizeUsage()
			return capturedUsage, true
		}
		g.Log().Warningf(ctx, "[relaykit] convert stream failed (converter=%s): %v", converterID, err)
		if !gotFinish {
			writeTerminalForClient()
		}
		writeDoneMarker()
		setEndReason(common.StreamEndReasonError, err)
		finalizeUsage()
		return capturedUsage, true
	}

	if !gotFinish {
		writeTerminalForClient()
	}
	writeDoneMarker()
	setEndReason(common.StreamEndReasonDone, nil)
	finalizeUsage()
	return capturedUsage, true
}
