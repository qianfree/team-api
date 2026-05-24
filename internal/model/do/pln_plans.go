// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// PlnPlans is the golang structure of table pln_plans for DAO operations like Where/Data.
type PlnPlans struct {
	g.Meta              `orm:"table:pln_plans, do:true"`
	Id                  any         // 主键ID
	Name                any         // 套餐显示名称
	Identifier          any         // 套餐唯一标识（free/basic/pro/enterprise）
	Description         any         // 套餐描述（面向用户的营销文案）
	Price               any         // 套餐价格（CNY）
	Status              any         // 状态：active（上架）/ archived（下架）
	IsRecommended       any         // 是否推荐
	SortOrder           any         // 排序权重（数字越小越靠前）
	CreatedAt           *gtime.Time // 创建时间
	UpdatedAt           *gtime.Time // 更新时间
	CreditAmount        any         // 套餐包含的额度（USD）
	BonusAmount         any         // 赠送额度（USD）
	ValidityDays        any         // 有效天数，从激活时起算
	PurchaseLimit       any         // 限购数量，0=不限购
	PurchaseLimitPeriod any         // 限购周期：lifetime/monthly/yearly
	Stock               any         // 库存数量，NULL=不限
	TotalPurchased      any         // 累计购买次数
	AllowedModels       []string    // 允许使用的模型列表，空数组=全部模型
}
