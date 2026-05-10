// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// BilTransactions is the golang structure of table bil_transactions for DAO operations like Where/Data.
type BilTransactions struct {
	g.Meta       `orm:"table:bil_transactions, do:true"`
	Id           any         // 主键ID
	TenantId     any         // 租户ID
	WalletId     any         // 关联钱包ID
	Type         any         // 类型：recharge（充值）/ pre_deduct（预扣）/ settle（结算）/ refund（退款）/ adjust（调整）/ freeze（冻结）/ unfreeze（解冻）
	Amount       any         // 变动金额（正数=收入，负数=支出）
	BalanceAfter any         // 变动后总余额
	FrozenAfter  any         // 变动后冻结余额
	RelatedId    any         // 关联业务ID（如计费记录ID、订单ID等）
	RelatedType  any         // 关联业务类型：billing_record / order / refund / adjustment / redemption
	Description  any         // 交易描述
	CreatedAt    *gtime.Time // 创建时间
	UpdatedAt    *gtime.Time // 更新时间
}
