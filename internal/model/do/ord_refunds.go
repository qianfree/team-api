// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OrdRefunds is the golang structure of table ord_refunds for DAO operations like Where/Data.
type OrdRefunds struct {
	g.Meta          `orm:"table:ord_refunds, do:true"`
	Id              any         // 主键ID
	OrderId         any         // 关联订单ID
	TenantId        any         // 租户ID
	Amount          any         // 退款金额
	Reason          any         // 退款原因
	Status          any         // 退款状态
	PaymentChannel  any         // 原支付渠道
	PaymentRefundId any         // 第三方退款流水号
	ApprovedBy      any         // 审批人（管理员ID）
	ApprovedAt      *gtime.Time // 审批时间
	CreatedAt       *gtime.Time // 创建时间
	UpdatedAt       *gtime.Time // 更新时间
}
