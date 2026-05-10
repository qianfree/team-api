// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OrdPromoCodes is the golang structure of table ord_promo_codes for DAO operations like Where/Data.
type OrdPromoCodes struct {
	g.Meta        `orm:"table:ord_promo_codes, do:true"`
	Id            any         //
	Code          any         // 优惠码文本（唯一）
	Name          any         //
	Type          any         // 类型：percentage（折扣百分比）/ fixed（立减固定金额）
	DiscountValue any         // 折扣值（百分比 0-100，立减为金额）
	MinAmount     any         // 最低订单金额
	MaxDiscount   any         // 最大折扣金额（0=不限）
	TotalCount    any         //
	UsedCount     any         //
	PerUserLimit  any         //
	ValidFrom     *gtime.Time //
	ValidTo       *gtime.Time //
	PlanIds       []int64     // 适用套餐ID数组（NULL=全部）
	Status        any         //
	CreatedAt     *gtime.Time //
	UpdatedAt     *gtime.Time //
}
