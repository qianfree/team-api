// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// BilUsageDaily is the golang structure for table bil_usage_daily.
type BilUsageDaily struct {
	Id              int64           `json:"id"                 orm:"id"                 description:""`                                                                  //
	StatDate        *gtime.Time     `json:"stat_date"          orm:"stat_date"          description:"统计日期（按 created_at 的日期分桶）"`                                          // 统计日期（按 created_at 的日期分桶）
	TenantId        int64           `json:"tenant_id"          orm:"tenant_id"          description:"租户ID"`                                                              // 租户ID
	ProjectId       int64           `json:"project_id"         orm:"project_id"         description:"项目ID（0=无项目/个人Key）"`                                                 // 项目ID（0=无项目/个人Key）
	ModelName       string          `json:"model_name"         orm:"model_name"         description:"模型名称"`                                                              // 模型名称
	ChannelId       int64           `json:"channel_id"         orm:"channel_id"         description:"渠道ID（0=无渠道）"`                                                       // 渠道ID（0=无渠道）
	Status          string          `json:"status"             orm:"status"             description:"请求状态：success/error/timeout/cancelled"`                              // 请求状态：success/error/timeout/cancelled
	RequestCount    int             `json:"request_count"      orm:"request_count"      description:"请求数（COUNT(*)）"`                                                     // 请求数（COUNT(*)）
	InputTokens     int64           `json:"input_tokens"       orm:"input_tokens"       description:"输入Token合计"`                                                         // 输入Token合计
	OutputTokens    int64           `json:"output_tokens"      orm:"output_tokens"      description:"输出Token合计"`                                                         // 输出Token合计
	TotalCost       decimal.Decimal `json:"total_cost"         orm:"total_cost"         description:"客户侧成本合计（USD，源自 bil_usage_logs.total_cost）"`                         // 客户侧成本合计（USD，源自 bil_usage_logs.total_cost）
	AccountCost     decimal.Decimal `json:"account_cost"       orm:"account_cost"       description:"上游账户成本合计（USD，源自 bil_usage_logs.account_cost，用于利润分析）"`               // 上游账户成本合计（USD，源自 bil_usage_logs.account_cost，用于利润分析）
	SumLatencyMs    int64           `json:"sum_latency_ms"     orm:"sum_latency_ms"     description:"总延迟合计（ms，源自 bil_usage_logs.latency_ms；视图按 SUM/COUNT 求均值）"`          // 总延迟合计（ms，源自 bil_usage_logs.latency_ms；视图按 SUM/COUNT 求均值）
	SumFirstTokenMs int64           `json:"sum_first_token_ms" orm:"sum_first_token_ms" description:"首Token延迟合计（ms，源自 bil_usage_logs.first_token_ms；视图按 SUM/COUNT 求均值）"` // 首Token延迟合计（ms，源自 bil_usage_logs.first_token_ms；视图按 SUM/COUNT 求均值）
	CreatedAt       *gtime.Time     `json:"created_at"         orm:"created_at"         description:""`                                                                  //
	UpdatedAt       *gtime.Time     `json:"updated_at"         orm:"updated_at"         description:""`                                                                  //
}
