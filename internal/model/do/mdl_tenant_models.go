// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// MdlTenantModels is the golang structure of table mdl_tenant_models for DAO operations like Where/Data.
type MdlTenantModels struct {
	g.Meta          `orm:"table:mdl_tenant_models, do:true"`
	Id              any              // 主键ID
	TenantId        any              // 租户ID
	ModelId         any              // 模型ID
	Enabled         any              // 是否启用（禁用后该租户无法调用此模型）
	Multiplier      any              // 租户价格倍率（VIP 折扣）。作为 tenant_multiplier 参与计费，当前实际生效公式 = 基础价格 × 租户乘数 × 时段乘数；模型乘数为预留能力（mdl_models 无 multiplier 字段、computeCost 未接入），bil_records.model_multiplier 快照恒为 1.0。倍率来源优先级：discount_ratio > multiplier > 租户等级 price_multiplier（级别兜底仅在前两者均未设置、且级别倍率 <1.0 时生效）
	CreatedAt       *gtime.Time      // 创建时间
	UpdatedAt       *gtime.Time      // 更新时间
	DiscountRatio   *decimal.Decimal // 折扣比例（如0.8表示八折，NULL表示不打折，优先于 multiplier 使用）
	MaxConcurrency  any              // 单模型并发上限（NULL表示不限制）
	ChannelScope    any              // 渠道范围覆盖（NULL跟随租户默认，[]表示全部，数组表示指定渠道ID）
	CustomPricing   any              // 定价覆盖 JSONB（PricingBlob 形状，唯一入口）：键存在=覆盖/缺失=继承；价格键+discount_ratio 互斥（一口价）。含旧 TenantPricingPatch 三键（prices/time_segments/param_multipliers）同形兼容
	PriceNote       any              // 价格说明覆盖（仅管理后台可见），NULL 表示继承平台
	DiscountLabel   any              // 折扣标签覆盖（对外展示），NULL 表示继承平台
	PriceChangeNote any              // 价格调整说明覆盖（对外展示），NULL 表示继承平台
}
