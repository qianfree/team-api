// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// BilTransactions is the golang structure for table bil_transactions.
type BilTransactions struct {
	Id           int64       `json:"id"            orm:"id"            description:"主键ID"`                                                                                          // 主键ID
	TenantId     int64       `json:"tenant_id"     orm:"tenant_id"     description:"租户ID"`                                                                                          // 租户ID
	WalletId     int64       `json:"wallet_id"     orm:"wallet_id"     description:"关联钱包ID"`                                                                                        // 关联钱包ID
	Type         string      `json:"type"          orm:"type"          description:"类型：recharge（充值）/ pre_deduct（预扣）/ settle（结算）/ refund（退款）/ adjust（调整）/ freeze（冻结）/ unfreeze（解冻）"` // 类型：recharge（充值）/ pre_deduct（预扣）/ settle（结算）/ refund（退款）/ adjust（调整）/ freeze（冻结）/ unfreeze（解冻）
	Amount       float64     `json:"amount"        orm:"amount"        description:"变动金额（正数=收入，负数=支出）"`                                                                             // 变动金额（正数=收入，负数=支出）
	BalanceAfter float64     `json:"balance_after" orm:"balance_after" description:"变动后总余额"`                                                                                        // 变动后总余额
	FrozenAfter  float64     `json:"frozen_after"  orm:"frozen_after"  description:"变动后冻结余额"`                                                                                       // 变动后冻结余额
	RelatedId    int64       `json:"related_id"    orm:"related_id"    description:"关联业务ID（如计费记录ID、订单ID等）"`                                                                         // 关联业务ID（如计费记录ID、订单ID等）
	RelatedType  string      `json:"related_type"  orm:"related_type"  description:"关联业务类型：billing_record / order / refund / adjustment / redemption"`                              // 关联业务类型：billing_record / order / refund / adjustment / redemption
	Description  string      `json:"description"   orm:"description"   description:"交易描述"`                                                                                          // 交易描述
	CreatedAt    *gtime.Time `json:"created_at"    orm:"created_at"    description:"创建时间"`                                                                                          // 创建时间
	UpdatedAt    *gtime.Time `json:"updated_at"    orm:"updated_at"    description:"更新时间"`                                                                                          // 更新时间
}
