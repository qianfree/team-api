// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// BilWallets is the golang structure for table bil_wallets.
type BilWallets struct {
	Id               int64       `json:"id"                orm:"id"                description:"主键ID"`                                          // 主键ID
	TenantId         int64       `json:"tenant_id"         orm:"tenant_id"         description:"租户ID（每个租户一个钱包）"`                                // 租户ID（每个租户一个钱包）
	Balance          float64     `json:"balance"           orm:"balance"           description:"总余额"`                                           // 总余额
	FrozenBalance    float64     `json:"frozen_balance"    orm:"frozen_balance"    description:"冻结余额（支付中/退款中，可用余额 = balance - frozen_balance）"` // 冻结余额（支付中/退款中，可用余额 = balance - frozen_balance）
	WarningThreshold float64     `json:"warning_threshold" orm:"warning_threshold" description:"余额预警线（低于此值触发通知）"`                               // 余额预警线（低于此值触发通知）
	Currency         string      `json:"currency"          orm:"currency"          description:"货币（USD）"`                                       // 货币（USD）
	CreatedAt        *gtime.Time `json:"created_at"        orm:"created_at"        description:"创建时间"`                                          // 创建时间
	UpdatedAt        *gtime.Time `json:"updated_at"        orm:"updated_at"        description:"更新时间"`                                          // 更新时间
}
