// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// BilUsageDaily is the golang structure of table bil_usage_daily for DAO operations like Where/Data.
type BilUsageDaily struct {
	g.Meta               `orm:"table:bil_usage_daily, do:true"`
	Id                   any         //
	StatDate             *gtime.Time // 统计日期（按 created_at 的日期分桶）
	TenantId             any         // 租户ID
	ProjectId            any         // 项目ID（0=无项目/个人Key）
	ModelName            any         // 模型名称
	ChannelId            any         // 渠道ID（0=无渠道）
	Status               any         // 请求状态：success/error/timeout/cancelled
	RequestCount         any         // 请求数（COUNT(*)）
	InputTokens          any         // 输入Token合计
	OutputTokens         any         // 输出Token合计
	TotalCost            any         // 客户侧成本合计（USD，源自 bil_usage_logs.total_cost）
	AccountCost          any         // 上游账户成本合计（USD，源自 bil_usage_logs.account_cost，用于利润分析）
	SumLatencyMs         any         // 总延迟合计（ms，源自 bil_usage_logs.latency_ms；视图按 SUM/COUNT 求均值）
	SumFirstTokenMs      any         // 首Token延迟合计（ms，源自 bil_usage_logs.first_token_ms；视图按 SUM/COUNT 求均值）
	CreatedAt            *gtime.Time //
	UpdatedAt            *gtime.Time //
	CacheCreationTokens  any         // 缓存创建 token 数（Claude cache_creation，SUM 聚合）
	CacheReadTokens      any         // 缓存命中读取 token 数（Claude cache_read / OpenAI cached_tokens，SUM 聚合）
	CacheHitRequestCount any         // 命中缓存的请求数（明细 cache_read_tokens>0 计 1，含失败状态行）
}
