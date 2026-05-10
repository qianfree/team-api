// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// BilDailyUsageSummary is the golang structure for table bil_daily_usage_summary.
type BilDailyUsageSummary struct {
	Id            int64       `json:"id"             orm:"id"             description:""` //
	TenantId      int64       `json:"tenant_id"      orm:"tenant_id"      description:""` //
	Date          *gtime.Time `json:"date"           orm:"date"           description:""` //
	TotalRequests int64       `json:"total_requests" orm:"total_requests" description:""` //
	TotalTokens   int64       `json:"total_tokens"   orm:"total_tokens"   description:""` //
	TotalCost     float64     `json:"total_cost"     orm:"total_cost"     description:""` //
	CreatedAt     *gtime.Time `json:"created_at"     orm:"created_at"     description:""` //
	UpdatedAt     *gtime.Time `json:"updated_at"     orm:"updated_at"     description:""` //
}
