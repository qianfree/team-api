// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// BilWallets is the golang structure of table bil_wallets for DAO operations like Where/Data.
type BilWallets struct {
	g.Meta           `orm:"table:bil_wallets, do:true"`
	Id               any         // 主键ID
	TenantId         any         // 租户ID（每个租户一个钱包）
	Balance          any         // 总余额
	FrozenBalance    any         // 冻结余额（支付中/退款中，可用余额 = balance - frozen_balance）
	WarningThreshold any         // 余额预警线（低于此值触发通知）
	Currency         any         // 货币（USD）
	CreatedAt        *gtime.Time // 创建时间
	UpdatedAt        *gtime.Time // 更新时间
}
