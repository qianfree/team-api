// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlTenantModels is the golang structure for table mdl_tenant_models.
type MdlTenantModels struct {
	Id                int64       `json:"id"                  orm:"id"                  description:"主键ID"`                                            // 主键ID
	TenantId          int64       `json:"tenant_id"           orm:"tenant_id"           description:"租户ID"`                                            // 租户ID
	ModelId           int64       `json:"model_id"            orm:"model_id"            description:"模型ID"`                                            // 模型ID
	Enabled           bool        `json:"enabled"             orm:"enabled"             description:"是否启用（禁用后该租户无法调用此模型）"`                             // 是否启用（禁用后该租户无法调用此模型）
	CustomInputPrice  float64     `json:"custom_input_price"  orm:"custom_input_price"  description:"租户自定义输入价格（NULL 表示使用默认定价）"`                        // 租户自定义输入价格（NULL 表示使用默认定价）
	CustomOutputPrice float64     `json:"custom_output_price" orm:"custom_output_price" description:"租户自定义输出价格（NULL 表示使用默认定价）"`                        // 租户自定义输出价格（NULL 表示使用默认定价）
	Multiplier        float64     `json:"multiplier"          orm:"multiplier"          description:"租户价格倍率（VIP 折扣等，最终价格 = 基础价格 × 模型倍率 × 租户倍率）"`       // 租户价格倍率（VIP 折扣等，最终价格 = 基础价格 × 模型倍率 × 租户倍率）
	CreatedAt         *gtime.Time `json:"created_at"          orm:"created_at"          description:"创建时间"`                                            // 创建时间
	UpdatedAt         *gtime.Time `json:"updated_at"          orm:"updated_at"          description:"更新时间"`                                            // 更新时间
	BillingMode       string      `json:"billing_mode"        orm:"billing_mode"        description:"覆盖模型计费方式（NULL表示跟随模型默认）"`                          // 覆盖模型计费方式（NULL表示跟随模型默认）
	PerRequestPrice   float64     `json:"per_request_price"   orm:"per_request_price"   description:"按次计费单价（覆盖模型默认，仅 billing_mode = per_request 时有效）"` // 按次计费单价（覆盖模型默认，仅 billing_mode = per_request 时有效）
	DiscountRatio     float64     `json:"discount_ratio"      orm:"discount_ratio"      description:"折扣比例（如0.8表示八折，NULL表示不打折，优先于 multiplier 使用）"`      // 折扣比例（如0.8表示八折，NULL表示不打折，优先于 multiplier 使用）
	MaxConcurrency    int         `json:"max_concurrency"     orm:"max_concurrency"     description:"单模型并发上限（NULL表示不限制）"`                              // 单模型并发上限（NULL表示不限制）
	ChannelScope      string      `json:"channel_scope"       orm:"channel_scope"       description:"渠道范围覆盖（NULL跟随租户默认，[]表示全部，数组表示指定渠道ID）"`            // 渠道范围覆盖（NULL跟随租户默认，[]表示全部，数组表示指定渠道ID）
}
