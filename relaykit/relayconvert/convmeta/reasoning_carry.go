package convmeta

// ReasoningCarry 宿主可选能力：思考文本跨轮携带（进程内状态）。
//
// 背景：DeepSeek 等 thinking 上游要求多轮工具调用把上一轮的 reasoning_content
// 随 assistant 历史传回，否则 400（"must be passed back to the API"）。标准回传
// 通道是 Responses 的 reasoning 项 encrypted_content（网关自编自解），但部分
// responses 客户端（ai-sdk 系）重建历史时把 reasoning 项整个剥掉，回传链在客户端
// 断裂。兜底：网关在响应侧按本轮工具调用 call_id 存下思考文本，下一轮请求转换
// 按 function_call 的 call_id 捞回。
//
// 转换器对 Meta 做类型断言获取本接口；未实现时静默跳过（仅靠 encrypted_content 回传）。
// 宿主端由 *relaycommon.RelayInfo 实现（进程内 TTL 缓存）。
type ReasoningCarry interface {
	// StoreReasoningForCalls 把本轮思考文本按工具调用 call_id 存入（每个 id 一份副本）。
	StoreReasoningForCalls(callIDs []string, reasoning string)
	// LookupReasoningForCalls 按 call_id 查思考文本，命中任一即返回；无命中返回空串。
	LookupReasoningForCalls(callIDs []string) string
}
