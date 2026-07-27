// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// OrdPromoCodes is the golang structure for table ord_promo_codes.
type OrdPromoCodes struct {
	Id            int64           `json:"id"             orm:"id"             description:""`                                    //
	Code          string          `json:"code"           orm:"code"           description:"优惠码文本（唯一）"`                           // 优惠码文本（唯一）
	Name          string          `json:"name"           orm:"name"           description:""`                                    //
	Type          string          `json:"type"           orm:"type"           description:"类型：percentage（折扣百分比）/ fixed（立减固定金额）"` // 类型：percentage（折扣百分比）/ fixed（立减固定金额）
	DiscountValue decimal.Decimal `json:"discount_value" orm:"discount_value" description:"折扣值（百分比 0-100，立减为金额）"`                // 折扣值（百分比 0-100，立减为金额）
	MinAmount     decimal.Decimal `json:"min_amount"     orm:"min_amount"     description:"最低订单金额"`                              // 最低订单金额
	MaxDiscount   decimal.Decimal `json:"max_discount"   orm:"max_discount"   description:"最大折扣金额（0=不限）"`                        // 最大折扣金额（0=不限）
	TotalCount    int             `json:"total_count"    orm:"total_count"    description:""`                                    //
	UsedCount     int             `json:"used_count"     orm:"used_count"     description:""`                                    //
	PerUserLimit  int             `json:"per_user_limit" orm:"per_user_limit" description:""`                                    //
	ValidFrom     *gtime.Time     `json:"valid_from"     orm:"valid_from"     description:""`                                    //
	ValidTo       *gtime.Time     `json:"valid_to"       orm:"valid_to"       description:""`                                    //
	PlanIds       []int64         `json:"plan_ids"       orm:"plan_ids"       description:"适用套餐ID数组（NULL=全部）"`                   // 适用套餐ID数组（NULL=全部）
	Status        string          `json:"status"         orm:"status"         description:""`                                    //
	CreatedAt     *gtime.Time     `json:"created_at"     orm:"created_at"     description:""`                                    //
	UpdatedAt     *gtime.Time     `json:"updated_at"     orm:"updated_at"     description:""`                                    //
}
