// 工具调用计费的宿主侧导出（计费旁路计量）。
//
// grounding / server_tool_use 的解析实现位于 internal/shared（转换器共用），
// 宿主渠道层（relay/channel/*）不能导入 internal 包，经本文件导出薄包装。
// 价格查找与附加费计算在宿主计费层（internal/logic/billing/tool_pricing.go）。
package relayconvert

import (
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/shared"
)

// GeminiGroundingSearchRequests 从单个 candidate.groundingMetadata 提取联网搜索次数
// （webSearchQueries 条数；缺失但有引用来源按 1 次；无 grounding 为 0）。
// 供宿主渠道层在原生透传路径统计工具调用计数（计费 key: google_search）
func GeminiGroundingSearchRequests(meta any) int {
	if c := shared.ParseGeminiGrounding(meta); c != nil {
		return c.SearchRequestCount()
	}
	return 0
}
