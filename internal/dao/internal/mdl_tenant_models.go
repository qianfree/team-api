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
	Id                       string // 主键ID
	TenantId                 string // 租户ID
	ModelId                  string // 模型ID
	Enabled                  string // 是否启用（禁用后该租户无法调用此模型）
	CustomInputPrice         string // 租户自定义输入价格（NULL 表示使用默认定价）
	CustomOutputPrice        string // 租户自定义输出价格（NULL 表示使用默认定价）
	Multiplier               string // 租户价格倍率（VIP 折扣等，最终价格 = 基础价格 × 模型倍率 × 租户倍率）
	CreatedAt                string // 创建时间
	UpdatedAt                string // 更新时间
	BillingMode              string // 覆盖模型计费方式（NULL表示跟随模型默认）
	PerRequestPrice          string // 按次计费单价（覆盖模型默认，仅 billing_mode = per_request 时有效）
	DiscountRatio            string // 折扣比例（如0.8表示八折，NULL表示不打折，优先于 multiplier 使用）
	MaxConcurrency           string // 单模型并发上限（NULL表示不限制）
	ChannelScope             string // 渠道范围覆盖（NULL跟随租户默认，[]表示全部，数组表示指定渠道ID）
	CustomCacheReadPrice     string // 自定义缓存读取价格（$/1M token），NULL 表示使用基础定价
	CustomCacheCreationPrice string // 自定义缓存创建价格（$/1M token），NULL 表示使用基础定价
	CustomPricingTiers       string // 自定义阶梯定价（JSONB 数组），格式: [{"min_tokens":0,"max_tokens":100000,"input_price":0.5,"output_price":1.5,"cache_read_price":0.1,"cache_creation_price":0.2}]
}

// mdlTenantModelsColumns holds the columns for the table mdl_tenant_models.
var mdlTenantModelsColumns = MdlTenantModelsColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	ModelId:                  "model_id",
	Enabled:                  "enabled",
	CustomInputPrice:         "custom_input_price",
	CustomOutputPrice:        "custom_output_price",
	Multiplier:               "multiplier",
	CreatedAt:                "created_at",
	UpdatedAt:                "updated_at",
	BillingMode:              "billing_mode",
	PerRequestPrice:          "per_request_price",
	DiscountRatio:            "discount_ratio",
	MaxConcurrency:           "max_concurrency",
	ChannelScope:             "channel_scope",
	CustomCacheReadPrice:     "custom_cache_read_price",
	CustomCacheCreationPrice: "custom_cache_creation_price",
	CustomPricingTiers:       "custom_pricing_tiers",
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
