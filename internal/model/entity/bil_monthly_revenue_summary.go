// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// BilMonthlyRevenueSummary is the golang structure for table bil_monthly_revenue_summary.
type BilMonthlyRevenueSummary struct {
	Id               int64       `json:"id"                orm:"id"                description:""` //
	Month            *gtime.Time `json:"month"             orm:"month"             description:""` //
	TotalRecharge    float64     `json:"total_recharge"    orm:"total_recharge"    description:""` //
	TotalConsumption float64     `json:"total_consumption" orm:"total_consumption" description:""` //
	NetRevenue       float64     `json:"net_revenue"       orm:"net_revenue"       description:""` //
	CreatedAt        *gtime.Time `json:"created_at"        orm:"created_at"        description:""` //
	UpdatedAt        *gtime.Time `json:"updated_at"        orm:"updated_at"        description:""` //
}
