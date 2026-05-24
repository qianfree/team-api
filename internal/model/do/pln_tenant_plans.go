// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// PlnTenantPlans is the golang structure of table pln_tenant_plans for DAO operations like Where/Data.
type PlnTenantPlans struct {
	g.Meta           `orm:"table:pln_tenant_plans, do:true"`
	Id               any         // 主键ID
	TenantId         any         // 租户ID
	PlanId           any         // 套餐ID
	Status           any         // 状态：pending（待生效）/ active（生效中）/ expired（已过期）/ cancelled（已取消）
	StartAt          *gtime.Time // 生效起始时间
	EndAt            *gtime.Time // 到期时间
	CreatedAt        *gtime.Time // 创建时间
	UpdatedAt        *gtime.Time // 更新时间
	TotalCredits     any         // 总额度（USD）= credit_amount + bonus_amount
	RemainingCredits any         // 剩余额度（USD）
	PaidCny          any         // 实付金额（CNY）
	RefundedAt       *gtime.Time // 退款时间
	OrderId          any         // 关联订单ID
}
