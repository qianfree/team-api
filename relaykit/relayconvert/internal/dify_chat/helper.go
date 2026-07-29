package dify_chat

// getCurrentTimestamp 返回固定时间戳，保证 golden 测试稳定（与 oai_gemini 一致）。
// 生产侧响应 ID 的真实化归属阶段 6 灰度对齐。
func getCurrentTimestamp() int64 {
	return 1700000000
}
