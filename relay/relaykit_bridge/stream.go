package relaykit_bridge

import (
	"context"
	"encoding/json"
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
	upstream := helper.ProviderNativeFormat(info.ChannelMeta.ChannelType)
	clientFormat := info.GetOriginalClientFormat()
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

	// Gemini 的 promptTokenCount 已含 cachedContentTokenCount（cached 为其子集），
	// 置 CacheIncludedInPrompt 让计费扣减缓存部分避免双重计费；
	// Claude 的 input_tokens 与缓存桶独立，保持默认 false 由缓存部分单独计价
	capturedUsage := &common.Usage{CacheIncludedInPrompt: upstream == constant.RelayFormatGemini}
	var (
		gotFinish          bool // 转换器是否已产出带 finish_reason 的结束 chunk
		firstChunk         bool
		transferredTextLen int // 已转发的文本/思考内容长度，供流中断输出估算
	)

	// chunkWriter：将转换器产出的 *dto.ChatCompletionStreamResponse 序列化为 SSE 写出，
	// 并提取 Usage / 记录首字节时间 / 追踪是否已发结束 chunk。
	chunkWriter := func(chunk any) error {
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
		}
		data, err := json.Marshal(streamChunk)
		if err != nil {
			return err
		}
		return helper.WriteSSEData(safeWriter, string(data))
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
		data, _ := json.Marshal(terminal)
		_ = helper.WriteSSEData(safeWriter, string(data))
	}

	setEndReason := func(reason common.StreamEndReason, err error) {
		if info.StreamStatus != nil {
			info.StreamStatus.SetEndReason(reason, err)
		}
	}

	start := time.Now()
	err := fn(ctx, info, upstreamBody, chunkWriter)
	duration := time.Since(start)
	monitor.TrackConverterCall(converterID, string(upstream), string(clientFormat), duration, err)

	if err != nil {
		if ctx.Err() != nil {
			// 客户端断开 / 上下文取消：客户端已不可达，不写 [DONE]
			setEndReason(common.StreamEndReasonClientGone, ctx.Err())
			// 流中断计费兜底：输出缺失按已转发文本 2 字符/token 估算，输入用请求侧估算值补齐
			helper.ApplyInterruptedUsageFallback(info, capturedUsage, transferredTextLen)
			return capturedUsage, true
		}
		g.Log().Warningf(ctx, "[relaykit] convert stream failed (converter=%s): %v", converterID, err)
		if !gotFinish {
			writeTerminal()
		}
		_ = helper.WriteSSEData(safeWriter, "[DONE]")
		setEndReason(common.StreamEndReasonError, err)
		return capturedUsage, true
	}

	if !gotFinish {
		writeTerminal()
	}
	_ = helper.WriteSSEData(safeWriter, "[DONE]")
	setEndReason(common.StreamEndReasonDone, nil)
	return capturedUsage, true
}
