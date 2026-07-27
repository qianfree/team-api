// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// OrdOrders is the golang structure for table ord_orders.
type OrdOrders struct {
	Id             int64           `json:"id"               orm:"id"               description:"主键ID"`                                                                   // 主键ID
	OrderNo        string          `json:"order_no"         orm:"order_no"         description:"订单号（唯一，格式 ORD + 时间戳 + 随机数）"`                                             // 订单号（唯一，格式 ORD + 时间戳 + 随机数）
	TenantId       int64           `json:"tenant_id"        orm:"tenant_id"        description:"租户ID"`                                                                   // 租户ID
	UserId         int64           `json:"user_id"          orm:"user_id"          description:"下单用户ID"`                                                                 // 下单用户ID
	OrderType      string          `json:"order_type"       orm:"order_type"       description:"订单类型：new_plan（新购）/ renew（续费）/ upgrade（升级）/ downgrade（降级）/ recharge（充值）"` // 订单类型：new_plan（新购）/ renew（续费）/ upgrade（升级）/ downgrade（降级）/ recharge（充值）
	PlanId         int64           `json:"plan_id"          orm:"plan_id"          description:"套餐ID（充值订单时为 NULL）"`                                                      // 套餐ID（充值订单时为 NULL）
	Amount         decimal.Decimal `json:"amount"           orm:"amount"           description:"原始金额"`                                                                   // 原始金额
	DiscountAmount decimal.Decimal `json:"discount_amount"  orm:"discount_amount"  description:"优惠金额"`                                                                   // 优惠金额
	FinalAmount    decimal.Decimal `json:"final_amount"     orm:"final_amount"     description:"最终金额"`                                                                   // 最终金额
	Currency       string          `json:"currency"         orm:"currency"         description:"货币（订单层一律 CNY）"`                                                          // 货币（订单层一律 CNY）
	PaymentChannel string          `json:"payment_channel"  orm:"payment_channel"  description:"支付渠道"`                                                                   // 支付渠道
	PaymentMethod  string          `json:"payment_method"   orm:"payment_method"   description:"支付方式描述"`                                                                 // 支付方式描述
	PaymentNo      string          `json:"payment_no"       orm:"payment_no"       description:"第三方支付流水号"`                                                               // 第三方支付流水号
	Status         string          `json:"status"           orm:"status"           description:"订单状态"`                                                                   // 订单状态
	PaidAt         *gtime.Time     `json:"paid_at"          orm:"paid_at"          description:"支付时间"`                                                                   // 支付时间
	FulfilledAt    *gtime.Time     `json:"fulfilled_at"     orm:"fulfilled_at"     description:"履约完成时间"`                                                                 // 履约完成时间
	ExpiredAt      *gtime.Time     `json:"expired_at"       orm:"expired_at"       description:"过期时间（未支付 30 分钟后自动过期）"`                                                   // 过期时间（未支付 30 分钟后自动过期）
	CancelledAt    *gtime.Time     `json:"cancelled_at"     orm:"cancelled_at"     description:"取消时间"`                                                                   // 取消时间
	RelatedOrderId int64           `json:"related_order_id" orm:"related_order_id" description:"关联订单ID（退款时指向原始订单）"`                                                      // 关联订单ID（退款时指向原始订单）
	Description    string          `json:"description"      orm:"description"      description:"订单描述"`                                                                   // 订单描述
	CreatedAt      *gtime.Time     `json:"created_at"       orm:"created_at"       description:"创建时间"`                                                                   // 创建时间
	UpdatedAt      *gtime.Time     `json:"updated_at"       orm:"updated_at"       description:"更新时间"`                                                                   // 更新时间
}
