// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// PlnPlans is the golang structure for table pln_plans.
type PlnPlans struct {
	Id                  int64       `json:"id"                    orm:"id"                    description:"主键ID"`                              // 主键ID
	Name                string      `json:"name"                  orm:"name"                  description:"套餐显示名称"`                            // 套餐显示名称
	Identifier          string      `json:"identifier"            orm:"identifier"            description:"套餐唯一标识（free/basic/pro/enterprise）"` // 套餐唯一标识（free/basic/pro/enterprise）
	Description         string      `json:"description"           orm:"description"           description:"套餐描述（面向用户的营销文案）"`                   // 套餐描述（面向用户的营销文案）
	Price               float64     `json:"price"                 orm:"price"                 description:"套餐价格（CNY）"`                         // 套餐价格（CNY）
	Status              string      `json:"status"                orm:"status"                description:"状态：active（上架）/ archived（下架）"`       // 状态：active（上架）/ archived（下架）
	IsRecommended       bool        `json:"is_recommended"        orm:"is_recommended"        description:"是否推荐"`                              // 是否推荐
	SortOrder           int         `json:"sort_order"            orm:"sort_order"            description:"排序权重（数字越小越靠前）"`                     // 排序权重（数字越小越靠前）
	CreatedAt           *gtime.Time `json:"created_at"            orm:"created_at"            description:"创建时间"`                              // 创建时间
	UpdatedAt           *gtime.Time `json:"updated_at"            orm:"updated_at"            description:"更新时间"`                              // 更新时间
	CreditAmount        float64     `json:"credit_amount"         orm:"credit_amount"         description:"套餐包含的额度（USD）"`                      // 套餐包含的额度（USD）
	BonusAmount         float64     `json:"bonus_amount"          orm:"bonus_amount"          description:"赠送额度（USD）"`                         // 赠送额度（USD）
	ValidityDays        int         `json:"validity_days"         orm:"validity_days"         description:"有效天数，从激活时起算"`                       // 有效天数，从激活时起算
	PurchaseLimit       int         `json:"purchase_limit"        orm:"purchase_limit"        description:"限购数量，0=不限购"`                        // 限购数量，0=不限购
	PurchaseLimitPeriod string      `json:"purchase_limit_period" orm:"purchase_limit_period" description:"限购周期：lifetime/monthly/yearly"`      // 限购周期：lifetime/monthly/yearly
	Stock               int         `json:"stock"                 orm:"stock"                 description:"库存数量，NULL=不限"`                      // 库存数量，NULL=不限
	TotalPurchased      int         `json:"total_purchased"       orm:"total_purchased"       description:"累计购买次数"`                            // 累计购买次数
	AllowedModels       []string    `json:"allowed_models"        orm:"allowed_models"        description:"允许使用的模型列表，空数组=全部模型"`                // 允许使用的模型列表，空数组=全部模型
}
