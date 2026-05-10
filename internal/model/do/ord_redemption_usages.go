// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OrdRedemptionUsages is the golang structure of table ord_redemption_usages for DAO operations like Where/Data.
type OrdRedemptionUsages struct {
	g.Meta        `orm:"table:ord_redemption_usages, do:true"`
	Id            any         // 主键ID
	RedemptionId  any         // 关联兑换码ID
	TenantId      any         // 使用兑换码的租户ID
	UserId        any         // 执行兑换操作的用户ID
	Type          any         // 兑换类型：quota / plan / duration
	Value         any         // 兑换面值（quota类型为金额，plan/duration为0）
	TransactionId any         // 关联的交易流水ID（仅quota类型有值）
	CreatedAt     *gtime.Time // 创建时间
	UpdatedAt     *gtime.Time // 更新时间
}
