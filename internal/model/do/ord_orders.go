// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OrdOrders is the golang structure of table ord_orders for DAO operations like Where/Data.
type OrdOrders struct {
	g.Meta         `orm:"table:ord_orders, do:true"`
	Id             any         // 主键ID
	OrderNo        any         // 订单号（唯一，格式 ORD + 时间戳 + 随机数）
	TenantId       any         // 租户ID
	UserId         any         // 下单用户ID
	OrderType      any         // 订单类型：new_plan（新购）/ renew（续费）/ upgrade（升级）/ downgrade（降级）/ recharge（充值）
	PlanId         any         // 套餐ID（充值订单时为 NULL）
	Amount         any         // 原始金额
	DiscountAmount any         // 优惠金额
	FinalAmount    any         // 最终金额
	Currency       any         // 货币
	PaymentChannel any         // 支付渠道
	PaymentMethod  any         // 支付方式描述
	PaymentNo      any         // 第三方支付流水号
	Status         any         // 订单状态
	PaidAt         *gtime.Time // 支付时间
	FulfilledAt    *gtime.Time // 履约完成时间
	ExpiredAt      *gtime.Time // 过期时间（未支付 30 分钟后自动过期）
	CancelledAt    *gtime.Time // 取消时间
	RelatedOrderId any         // 关联订单ID（退款时指向原始订单）
	Description    any         // 订单描述
	CreatedAt      *gtime.Time // 创建时间
	UpdatedAt      *gtime.Time // 更新时间
}
