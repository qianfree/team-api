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
	g.Meta             `orm:"table:pln_plans, do:true"`
	Id                 any         // 主键ID
	Name               any         // 套餐显示名称
	Identifier         any         // 套餐唯一标识（free/basic/pro/enterprise）
	Description        any         // 套餐描述（面向用户的营销文案）
	MonthlyPrice       any         // 月度价格（CNY）
	YearlyPrice        any         // 年度价格（CNY，通常为月价×10）
	Status             any         // 状态：active（上架）/ archived（下架）
	MonthlyQuotaTokens any         // 每月 Token 配额（0=不限）
	AllowedModels      []string    // 允许使用的模型列表（NULL=全部，空数组=无）
	IsRecommended      any         // 是否推荐
	SortOrder          any         // 排序权重（数字越小越靠前）
	CreatedAt          *gtime.Time // 创建时间
	UpdatedAt          *gtime.Time // 更新时间
}
