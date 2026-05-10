// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// OrdRedemptions is the golang structure for table ord_redemptions.
type OrdRedemptions struct {
	Id           int64       `json:"id"            orm:"id"            description:""`                                         //
	Code         string      `json:"code"          orm:"code"          description:""`                                         //
	Type         string      `json:"type"          orm:"type"          description:"类型：quota（额度）/ plan（套餐时长）/ duration（时长天数）"` // 类型：quota（额度）/ plan（套餐时长）/ duration（时长天数）
	Value        float64     `json:"value"         orm:"value"         description:""`                                         //
	PlanId       int64       `json:"plan_id"       orm:"plan_id"       description:""`                                         //
	DurationDays int         `json:"duration_days" orm:"duration_days" description:""`                                         //
	MaxUses      int         `json:"max_uses"      orm:"max_uses"      description:""`                                         //
	UsedCount    int         `json:"used_count"    orm:"used_count"    description:""`                                         //
	RedeemedBy   int64       `json:"redeemed_by"   orm:"redeemed_by"   description:""`                                         //
	RedeemedAt   *gtime.Time `json:"redeemed_at"   orm:"redeemed_at"   description:""`                                         //
	ExpiresAt    *gtime.Time `json:"expires_at"    orm:"expires_at"    description:""`                                         //
	Status       string      `json:"status"        orm:"status"        description:""`                                         //
	BatchNo      string      `json:"batch_no"      orm:"batch_no"      description:"批次号（批量生成时，便于管理）"`                          // 批次号（批量生成时，便于管理）
	CreatedAt    *gtime.Time `json:"created_at"    orm:"created_at"    description:""`                                         //
	UpdatedAt    *gtime.Time `json:"updated_at"    orm:"updated_at"    description:""`                                         //
}
