// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// BilDailyRevenueSummary is the golang structure of table bil_daily_revenue_summary for DAO operations like Where/Data.
type BilDailyRevenueSummary struct {
	g.Meta           `orm:"table:bil_daily_revenue_summary, do:true"`
	Id               any         //
	Date             *gtime.Time //
	TotalRecharge    any         //
	TotalConsumption any         //
	NetRevenue       any         //
	NewOrders        any         //
	PaidOrders       any         //
	CreatedAt        *gtime.Time //
	UpdatedAt        *gtime.Time //
}
