// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MdlTenantModelsDao is the data access object for the table mdl_tenant_models.
type MdlTenantModelsDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  MdlTenantModelsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// MdlTenantModelsColumns defines and stores column names for the table mdl_tenant_models.
type MdlTenantModelsColumns struct {
	Id              string // 主键ID
	TenantId        string // 租户ID
	ModelId         string // 模型ID
	Enabled         string // 是否启用（禁用后该租户无法调用此模型）
	Multiplier      string // 租户价格倍率（VIP 折扣）。作为 tenant_multiplier 参与计费，当前实际生效公式 = 基础价格 × 租户乘数 × 时段乘数；模型乘数为预留能力（mdl_models 无 multiplier 字段、computeCost 未接入），bil_records.model_multiplier 快照恒为 1.0。倍率来源优先级：discount_ratio > multiplier > 租户等级 price_multiplier（级别兜底仅在前两者均未设置、且级别倍率 <1.0 时生效）
	CreatedAt       string // 创建时间
	UpdatedAt       string // 更新时间
	DiscountRatio   string // 折扣比例（如0.8表示八折，NULL表示不打折，优先于 multiplier 使用）
	MaxConcurrency  string // 单模型并发上限（NULL表示不限制）
	ChannelScope    string // 渠道范围覆盖（NULL跟随租户默认，[]表示全部，数组表示指定渠道ID）
	CustomPricing   string // 定价覆盖 JSONB（PricingBlob 形状，唯一入口）：键存在=覆盖/缺失=继承；价格键+discount_ratio 互斥（一口价）。含旧 TenantPricingPatch 三键（prices/time_segments/param_multipliers）同形兼容
	PriceNote       string // 价格说明覆盖（仅管理后台可见），NULL 表示继承平台
	DiscountLabel   string // 折扣标签覆盖（对外展示），NULL 表示继承平台
	PriceChangeNote string // 价格调整说明覆盖（对外展示），NULL 表示继承平台
}

// mdlTenantModelsColumns holds the columns for the table mdl_tenant_models.
var mdlTenantModelsColumns = MdlTenantModelsColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	ModelId:         "model_id",
	Enabled:         "enabled",
	Multiplier:      "multiplier",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
	DiscountRatio:   "discount_ratio",
	MaxConcurrency:  "max_concurrency",
	ChannelScope:    "channel_scope",
	CustomPricing:   "custom_pricing",
	PriceNote:       "price_note",
	DiscountLabel:   "discount_label",
	PriceChangeNote: "price_change_note",
}

// NewMdlTenantModelsDao creates and returns a new DAO object for table data access.
func NewMdlTenantModelsDao(handlers ...gdb.ModelHandler) *MdlTenantModelsDao {
	return &MdlTenantModelsDao{
		group:    "default",
		table:    "mdl_tenant_models",
		columns:  mdlTenantModelsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MdlTenantModelsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MdlTenantModelsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MdlTenantModelsDao) Columns() MdlTenantModelsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MdlTenantModelsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MdlTenantModelsDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *MdlTenantModelsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
