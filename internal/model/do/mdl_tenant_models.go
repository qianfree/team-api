// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlTenantModels is the golang structure of table mdl_tenant_models for DAO operations like Where/Data.
type MdlTenantModels struct {
	g.Meta            `orm:"table:mdl_tenant_models, do:true"`
	Id                any         // 主键ID
	TenantId          any         // 租户ID
	ModelId           any         // 模型ID
	Enabled           any         // 是否启用（禁用后该租户无法调用此模型）
	CustomInputPrice  any         // 租户自定义输入价格（NULL 表示使用默认定价）
	CustomOutputPrice any         // 租户自定义输出价格（NULL 表示使用默认定价）
	Multiplier        any         // 租户价格倍率（VIP 折扣等，最终价格 = 基础价格 × 模型倍率 × 租户倍率）
	CreatedAt         *gtime.Time // 创建时间
	UpdatedAt         *gtime.Time // 更新时间
	BillingMode       any         // 覆盖模型计费方式（NULL表示跟随模型默认）
	PerRequestPrice   any         // 按次计费单价（覆盖模型默认，仅 billing_mode = per_request 时有效）
	DiscountRatio     any         // 折扣比例（如0.8表示八折，NULL表示不打折，优先于 multiplier 使用）
	MaxConcurrency    any         // 单模型并发上限（NULL表示不限制）
	ChannelScope      any         // 渠道范围覆盖（NULL跟随租户默认，[]表示全部，数组表示指定渠道ID）
}
