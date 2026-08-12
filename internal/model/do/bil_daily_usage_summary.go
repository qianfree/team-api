// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// BilDailyUsageSummary is the golang structure of table bil_daily_usage_summary for DAO operations like Where/Data.
type BilDailyUsageSummary struct {
	g.Meta        `orm:"table:bil_daily_usage_summary, do:true"`
	Id            any         //
	TenantId      any         //
	Date          *gtime.Time //
	TotalRequests any         //
	TotalTokens   any         //
	TotalCost     any         //
	CreatedAt     *gtime.Time //
	UpdatedAt     *gtime.Time //
}
