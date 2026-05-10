// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// BilDailyRevenueSummary is the golang structure for table bil_daily_revenue_summary.
type BilDailyRevenueSummary struct {
	Id               int64       `json:"id"                orm:"id"                description:""` //
	Date             *gtime.Time `json:"date"              orm:"date"              description:""` //
	TotalRecharge    float64     `json:"total_recharge"    orm:"total_recharge"    description:""` //
	TotalConsumption float64     `json:"total_consumption" orm:"total_consumption" description:""` //
	NetRevenue       float64     `json:"net_revenue"       orm:"net_revenue"       description:""` //
	NewOrders        int         `json:"new_orders"        orm:"new_orders"        description:""` //
	PaidOrders       int         `json:"paid_orders"       orm:"paid_orders"       description:""` //
	CreatedAt        *gtime.Time `json:"created_at"        orm:"created_at"        description:""` //
	UpdatedAt        *gtime.Time `json:"updated_at"        orm:"updated_at"        description:""` //
}
