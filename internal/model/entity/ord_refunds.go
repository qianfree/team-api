// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// OrdRefunds is the golang structure for table ord_refunds.
type OrdRefunds struct {
	Id              int64           `json:"id"                orm:"id"                description:"主键ID"`       // 主键ID
	OrderId         int64           `json:"order_id"          orm:"order_id"          description:"关联订单ID"`     // 关联订单ID
	TenantId        int64           `json:"tenant_id"         orm:"tenant_id"         description:"租户ID"`       // 租户ID
	Amount          decimal.Decimal `json:"amount"            orm:"amount"            description:"退款金额"`       // 退款金额
	Reason          string          `json:"reason"            orm:"reason"            description:"退款原因"`       // 退款原因
	Status          string          `json:"status"            orm:"status"            description:"退款状态"`       // 退款状态
	PaymentChannel  string          `json:"payment_channel"   orm:"payment_channel"   description:"原支付渠道"`      // 原支付渠道
	PaymentRefundId string          `json:"payment_refund_id" orm:"payment_refund_id" description:"第三方退款流水号"`   // 第三方退款流水号
	ApprovedBy      int64           `json:"approved_by"       orm:"approved_by"       description:"审批人（管理员ID）"` // 审批人（管理员ID）
	ApprovedAt      *gtime.Time     `json:"approved_at"       orm:"approved_at"       description:"审批时间"`       // 审批时间
	CreatedAt       *gtime.Time     `json:"created_at"        orm:"created_at"        description:"创建时间"`       // 创建时间
	UpdatedAt       *gtime.Time     `json:"updated_at"        orm:"updated_at"        description:"更新时间"`       // 更新时间
}
