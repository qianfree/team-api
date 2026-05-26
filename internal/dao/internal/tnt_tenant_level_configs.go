// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TntTenantLevelConfigsDao is the data access object for the table tnt_tenant_level_configs.
type TntTenantLevelConfigsDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  TntTenantLevelConfigsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// TntTenantLevelConfigsColumns defines and stores column names for the table tnt_tenant_level_configs.
type TntTenantLevelConfigsColumns struct {
	Id                          string //
	Level                       string // 等级号（1, 2, 3...）
	Name                        string // 等级名称
	CumulativeRechargeThreshold string // 累计充值阈值（USD），达到此值自动升级
	MaxMembers                  string // 该等级最大成员数
	MaxConcurrency              string // 该等级最大并发数，0=无限
	PriceMultiplier             string // 价格乘数（折扣，如 0.9=九折）
	SortOrder                   string // 排序权重
	CreatedAt                   string //
	UpdatedAt                   string //
}

// tntTenantLevelConfigsColumns holds the columns for the table tnt_tenant_level_configs.
var tntTenantLevelConfigsColumns = TntTenantLevelConfigsColumns{
	Id:                          "id",
	Level:                       "level",
	Name:                        "name",
	CumulativeRechargeThreshold: "cumulative_recharge_threshold",
	MaxMembers:                  "max_members",
	MaxConcurrency:              "max_concurrency",
	PriceMultiplier:             "price_multiplier",
	SortOrder:                   "sort_order",
	CreatedAt:                   "created_at",
	UpdatedAt:                   "updated_at",
}

// NewTntTenantLevelConfigsDao creates and returns a new DAO object for table data access.
func NewTntTenantLevelConfigsDao(handlers ...gdb.ModelHandler) *TntTenantLevelConfigsDao {
	return &TntTenantLevelConfigsDao{
		group:    "default",
		table:    "tnt_tenant_level_configs",
		columns:  tntTenantLevelConfigsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TntTenantLevelConfigsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TntTenantLevelConfigsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TntTenantLevelConfigsDao) Columns() TntTenantLevelConfigsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TntTenantLevelConfigsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TntTenantLevelConfigsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TntTenantLevelConfigsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
