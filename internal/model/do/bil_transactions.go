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
	Type         any         // 类型：consume（消费）/ recharge（充值）/ adjust（调整）/ pre_deduct（预扣，已废弃）/ settle（结算，已废弃）/ refund（退款，已废弃）/ freeze（冻结，已废弃）/ unfreeze（解冻，已废弃）
	Amount       any         // 变动金额（正数=收入，负数=支出）
	BalanceAfter any         // 变动后总余额
	FrozenAfter  any         // 变动后冻结余额
	RelatedId    any         // 关联业务ID（如计费记录ID、订单ID等）
	RelatedType  any         // 关联业务类型：billing_record / order / refund / adjustment / redemption
	Description  any         // 交易描述
	CreatedAt    *gtime.Time // 创建时间
	UpdatedAt    *gtime.Time // 更新时间
	UserId       any         // 关联用户ID（consume 类型为实际消费用户，recharge 类型为操作用户，adjust 类型为空）
	RequestId    any         // 关联请求ID（consume 类型对应 API 调用的 request_id，其他类型为空）
	ModelName    any         // 关联模型名（consume 类型为调用的模型名，其他类型为空）
}
