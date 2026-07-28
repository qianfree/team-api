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
	relaylogic "github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
)

// 阶段 4 Task4：relaykit 流式响应转换器接入桥接层。
//
// 设计要点（与 Task 2/3 一致）：
//   - 完全由特性开关控制（relaylogic.IsRelaykitEnabledForChannel），默认关闭 → 零运行时影响。
//   - 只替换「格式转换」这一步；SSE 帧化、保活 ping、[DONE] 收尾、StreamStatus 由本层负责。
//   - 任何「写入前」的放弃（开关关闭、同格式、无匹配转换器）都返回 ok=false，
//     调用方回退到旧 handleStreamToOpenAI 代码路径。
//   - 一旦 SetEventStreamHeaders 之后（开始写 chunk）即不可回退：
//     转换中途失败由本层写入结束 chunk + [DONE] + 设置 end reason 后返回 ok=true。
//   - 转换耗时与成败通过 monitor.TrackConverterCall 记录，供 dashboard 观测灰度效果。

// TryConvertStreamViaRelaykit 尝试用 relaykit 流式转换器将上游 SSE 流转换为客户端格式。
//
// 返回：
//   - usage：从最后一个带 Usage 的 chunk 提取的用量（无则零值 Usage）
//   - ok：true 表示已接管响应（成功或优雅失败），调用方应直接返回，不再走旧路径；
//     false 表示未接管（开关关闭 / 无 ChannelMeta / 同格式 / 无匹配转换器），调用方回退旧路径。
//
// 注意：开关关闭时在任何 I/O 之前即返回 false，旧路径行为零变化。
func TryConvertStreamViaRelaykit(ctx context.Context, info *common.RelayInfo, upstreamBody io.Reader, writer http.ResponseWriter) (*common.Usage, bool) {
	if info == nil || info.ChannelMeta == nil {
		return nil, false
	}

	providerKey := providerKeyForChannelType(info.ChannelMeta.ChannelType)
	if providerKey == "" || !relaylogic.IsRelaykitEnabledForChannel(ctx, providerKey) {
		return nil, false
	}

	return convertStreamViaRelaykit(ctx, info, upstreamBody, writer)
}

// convertStreamViaRelaykit 流式转换核心：不读取特性开关配置（由公开入口保证），
// 抽离出来便于单测（参照 internal/logic/relay 中 isChannelInProviders 的纯函数抽离手法）。
func convertStreamViaRelaykit(ctx context.Context, info *common.RelayInfo, upstreamBody io.Reader, writer http.ResponseWriter) (*common.Usage, bool) {
	upstream := providerNativeFormat(info.ChannelMeta.ChannelType)
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

	capturedUsage := &common.Usage{}
	var (
		gotFinish  bool // 转换器是否已产出带 finish_reason 的结束 chunk
		firstChunk bool
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
		}
		if len(streamChunk.Choices) > 0 && streamChunk.Choices[0].FinishReason != nil {
			gotFinish = true
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
