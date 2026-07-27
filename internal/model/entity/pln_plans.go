// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// PlnPlans is the golang structure for table pln_plans.
type PlnPlans struct {
	Id                 int64           `json:"id"                   orm:"id"                   description:"主键ID"`                              // 主键ID
	Name               string          `json:"name"                 orm:"name"                 description:"套餐显示名称"`                            // 套餐显示名称
	Identifier         string          `json:"identifier"           orm:"identifier"           description:"套餐唯一标识（free/basic/pro/enterprise）"` // 套餐唯一标识（free/basic/pro/enterprise）
	Description        string          `json:"description"          orm:"description"          description:"套餐描述（面向用户的营销文案）"`                   // 套餐描述（面向用户的营销文案）
	MonthlyPrice       decimal.Decimal `json:"monthly_price"        orm:"monthly_price"        description:"月度价格（CNY）"`                         // 月度价格（CNY）
	YearlyPrice        decimal.Decimal `json:"yearly_price"         orm:"yearly_price"         description:"年度价格（CNY，通常为月价×10）"`                // 年度价格（CNY，通常为月价×10）
	Status             string          `json:"status"               orm:"status"               description:"状态：active（上架）/ archived（下架）"`       // 状态：active（上架）/ archived（下架）
	MonthlyQuotaTokens int64           `json:"monthly_quota_tokens" orm:"monthly_quota_tokens" description:"每月 Token 配额（0=不限）"`                 // 每月 Token 配额（0=不限）
	AllowedModels      []string        `json:"allowed_models"       orm:"allowed_models"       description:"允许使用的模型列表（NULL=全部，空数组=无）"`          // 允许使用的模型列表（NULL=全部，空数组=无）
	IsRecommended      bool            `json:"is_recommended"       orm:"is_recommended"       description:"是否推荐"`                              // 是否推荐
	SortOrder          int             `json:"sort_order"           orm:"sort_order"           description:"排序权重（数字越小越靠前）"`                     // 排序权重（数字越小越靠前）
	CreatedAt          *gtime.Time     `json:"created_at"           orm:"created_at"           description:"创建时间"`                              // 创建时间
	UpdatedAt          *gtime.Time     `json:"updated_at"           orm:"updated_at"           description:"更新时间"`                              // 更新时间
}
