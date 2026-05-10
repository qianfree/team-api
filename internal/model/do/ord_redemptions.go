// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OrdRedemptions is the golang structure of table ord_redemptions for DAO operations like Where/Data.
type OrdRedemptions struct {
	g.Meta       `orm:"table:ord_redemptions, do:true"`
	Id           any         //
	Code         any         //
	Type         any         // 类型：quota（额度）/ plan（套餐时长）/ duration（时长天数）
	Value        any         //
	PlanId       any         //
	DurationDays any         //
	MaxUses      any         //
	UsedCount    any         //
	RedeemedBy   any         //
	RedeemedAt   *gtime.Time //
	ExpiresAt    *gtime.Time //
	Status       any         //
	BatchNo      any         // 批次号（批量生成时，便于管理）
	CreatedAt    *gtime.Time //
	UpdatedAt    *gtime.Time //
}
