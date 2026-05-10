// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// BilMonthlyUsageSummary is the golang structure for table bil_monthly_usage_summary.
type BilMonthlyUsageSummary struct {
	Id            int64       `json:"id"             orm:"id"             description:""` //
	TenantId      int64       `json:"tenant_id"      orm:"tenant_id"      description:""` //
	Month         *gtime.Time `json:"month"          orm:"month"          description:""` //
	TotalRequests int64       `json:"total_requests" orm:"total_requests" description:""` //
	TotalTokens   int64       `json:"total_tokens"   orm:"total_tokens"   description:""` //
	TotalCost     float64     `json:"total_cost"     orm:"total_cost"     description:""` //
	CreatedAt     *gtime.Time `json:"created_at"     orm:"created_at"     description:""` //
	UpdatedAt     *gtime.Time `json:"updated_at"     orm:"updated_at"     description:""` //
}
