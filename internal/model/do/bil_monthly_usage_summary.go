// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// BilMonthlyUsageSummary is the golang structure of table bil_monthly_usage_summary for DAO operations like Where/Data.
type BilMonthlyUsageSummary struct {
	g.Meta        `orm:"table:bil_monthly_usage_summary, do:true"`
	Id            any         //
	TenantId      any         //
	Month         *gtime.Time //
	TotalRequests any         //
	TotalTokens   any         //
	TotalCost     any         //
	CreatedAt     *gtime.Time //
	UpdatedAt     *gtime.Time //
}
