// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// BilMonthlyRevenueSummary is the golang structure of table bil_monthly_revenue_summary for DAO operations like Where/Data.
type BilMonthlyRevenueSummary struct {
	g.Meta           `orm:"table:bil_monthly_revenue_summary, do:true"`
	Id               any         //
	Month            *gtime.Time //
	TotalRecharge    any         //
	TotalConsumption any         //
	NetRevenue       any         //
	CreatedAt        *gtime.Time //
	UpdatedAt        *gtime.Time //
}
