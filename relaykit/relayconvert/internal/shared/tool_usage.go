// 服务端工具调用计数提取（计费旁路计量）。
//
// 工具调用计费的计数经 StreamEvent.ToolCalls 侧信道透出（Usage 结构体无工具
// 计数字段，OpenAI 协议亦无对应报文），宿主桥接层捕获后随 Usage 结算。
// 价格查找与附加费计算在宿主计费层（internal/logic/billing/tool_pricing.go），
// 转换器只负责如实计数，与价格解耦。
package shared

import "github.com/qianfree/team-api/relaykit/dto"

// claudeWebSearchToolKey Claude server_tool_use.web_search_requests 对应的计费 key。
// geminiGoogleSearchToolKey Gemini grounding（googleSearch 工具）对应的计费 key。
// key 语义与宿主工具价格表（billing/tool_pricing.go）对齐
const (
	claudeWebSearchToolKey    = "web_search"
	geminiGoogleSearchToolKey = "google_search"
)

// ServerToolCallsFromClaudeUsage 从 Claude usage.server_tool_use 提取工具调用计数。
// web_fetch / code_execution 等 DTO 暂无对应字段，接入时在此补充映射。
// 返回 nil 表示无服务端工具调用（避免分配空 map）。
func ServerToolCallsFromClaudeUsage(u *dto.ClaudeUsage) map[string]int {
	if u == nil || u.ServerToolUse == nil {
		return nil
	}
	calls := make(map[string]int)
	if u.ServerToolUse.WebSearchRequests > 0 {
		calls[claudeWebSearchToolKey] = u.ServerToolUse.WebSearchRequests
	}
	if len(calls) == 0 {
		return nil
	}
	return calls
}

// GeminiSearchToolCalls 从 grounding 引用证据提取联网搜索调用计数。
// 次数语义与 GeminiToClaude 响应还原的 usage.server_tool_use.web_search_requests
// 一致（SearchRequestCount：webSearchQueries 条数，缺失但有来源按 1），
// 保证同一 Gemini 响应经不同客户端格式还原后计费口径相同。
func GeminiSearchToolCalls(c *GroundingCitations) map[string]int {
	if c == nil {
		return nil
	}
	if n := c.SearchRequestCount(); n > 0 {
		return map[string]int{geminiGoogleSearchToolKey: n}
	}
	return nil
}
