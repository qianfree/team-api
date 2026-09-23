// Package relayconvert — 通用流式事件封装。
//
// 早期流式转换器只服务「上游原生 → OpenAI 客户端」方向，chunkWriter 直接收
// *dto.ChatCompletionStreamResponse。反向与跨原生方向的输出是 Claude 事件流 /
// Gemini SSE / Responses 事件流，帧格式各异（是否带 event 名、结束语义），
// 因此引入 StreamEvent 封装：转换器负责产出「事件名 + 负载」，宿主桥接层负责
// SSE 帧化与 Usage 提取。
package relayconvert

import "github.com/qianfree/team-api/relaykit/dto"

// StreamEvent 一帧流式输出。
//
//   - Event 非空：写为事件帧（event: X\ndata: {json}\n\n），Claude / Responses 风格；
//   - Event 为空：写为纯数据帧（data: {json}\n\n），Gemini / OpenAI 风格；
//   - Usage 非空：宿主捕获为本次请求的用量（多次出现取最后一次）。
//     使用带明细的 UsageWithDetails，缓存读/写 token 明细不丢失（计费按明细扣减缓存价）。
//
// 输出为 OpenAI chat 格式时转换器仍可直接产出 *dto.ChatCompletionStreamResponse
// （宿主桥接层对两种 chunk 类型均兼容）。
type StreamEvent struct {
	Event string
	Data  any
	Usage *dto.UsageWithDetails
}
