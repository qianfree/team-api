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
//     使用带明细的 UsageWithDetails，缓存读/写 token 明细不丢失（计费按明细扣减缓存价）；
//   - ToolCalls 非空：宿主捕获为服务端/云端工具调用计数（工具名 → 次数，多次出现
//     同名工具取最大值——厂商累计报数替换语义）。Usage 结构体没有工具计数的容身之处
//     （OpenAI 协议无此字段），经本字段旁路透出供按次附加费计费。
type StreamEvent struct {
	Event     string
	Data      any
	Usage     *dto.UsageWithDetails
	ToolCalls map[string]int
}
