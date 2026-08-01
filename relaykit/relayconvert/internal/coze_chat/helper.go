package coze_chat

// getCurrentTimestamp 返回固定时间戳，保证 golden 测试稳定（与 oai_gemini 一致）。
// 生产侧响应 ID 的真实化归属阶段 6 灰度对齐。
func getCurrentTimestamp() int64 {
	return 1700000000
}

// estimateTokens 粗略估算 token 数（4 字符 ≈ 1 token），与旧实现 helper.EstimateTokens 口径一致。
func estimateTokens(s string) int {
	return len(s) / 4
}
