// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// PlnTenantPlans is the golang structure for table pln_tenant_plans.
type PlnTenantPlans struct {
	Id               int64       `json:"id"                orm:"id"                description:"主键ID"`                                                       // 主键ID
	TenantId         int64       `json:"tenant_id"         orm:"tenant_id"         description:"租户ID"`                                                       // 租户ID
	PlanId           int64       `json:"plan_id"           orm:"plan_id"           description:"套餐ID"`                                                       // 套餐ID
	Status           string      `json:"status"            orm:"status"            description:"状态：pending（待生效）/ active（生效中）/ expired（已过期）/ cancelled（已取消）"` // 状态：pending（待生效）/ active（生效中）/ expired（已过期）/ cancelled（已取消）
	StartAt          *gtime.Time `json:"start_at"          orm:"start_at"          description:"生效起始时间"`                                                     // 生效起始时间
	EndAt            *gtime.Time `json:"end_at"            orm:"end_at"            description:"到期时间"`                                                       // 到期时间
	CreatedAt        *gtime.Time `json:"created_at"        orm:"created_at"        description:"创建时间"`                                                       // 创建时间
	UpdatedAt        *gtime.Time `json:"updated_at"        orm:"updated_at"        description:"更新时间"`                                                       // 更新时间
	TotalCredits     float64     `json:"total_credits"     orm:"total_credits"     description:"总额度（USD）= credit_amount + bonus_amount"`                     // 总额度（USD）= credit_amount + bonus_amount
	RemainingCredits float64     `json:"remaining_credits" orm:"remaining_credits" description:"剩余额度（USD）"`                                                  // 剩余额度（USD）
	PaidCny          float64     `json:"paid_cny"          orm:"paid_cny"          description:"实付金额（CNY）"`                                                  // 实付金额（CNY）
	RefundedAt       *gtime.Time `json:"refunded_at"       orm:"refunded_at"       description:"退款时间"`                                                       // 退款时间
	OrderId          int64       `json:"order_id"          orm:"order_id"          description:"关联订单ID"`                                                     // 关联订单ID
}
