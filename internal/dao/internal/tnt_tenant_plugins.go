// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TntTenantPluginsDao is the data access object for the table tnt_tenant_plugins.
type TntTenantPluginsDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  TntTenantPluginsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// TntTenantPluginsColumns defines and stores column names for the table tnt_tenant_plugins.
type TntTenantPluginsColumns struct {
	Id         string //
	TenantId   string //
	PluginName string // 插件标识
	Enabled    string // 是否启用
	Config     string // 租户级配置覆盖（JSON），优先级高于全局配置
	CreatedAt  string //
	UpdatedAt  string //
}

// tntTenantPluginsColumns holds the columns for the table tnt_tenant_plugins.
var tntTenantPluginsColumns = TntTenantPluginsColumns{
	Id:         "id",
	TenantId:   "tenant_id",
	PluginName: "plugin_name",
	Enabled:    "enabled",
	Config:     "config",
	CreatedAt:  "created_at",
	UpdatedAt:  "updated_at",
}

// NewTntTenantPluginsDao creates and returns a new DAO object for table data access.
func NewTntTenantPluginsDao(handlers ...gdb.ModelHandler) *TntTenantPluginsDao {
	return &TntTenantPluginsDao{
		group:    "default",
		table:    "tnt_tenant_plugins",
		columns:  tntTenantPluginsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TntTenantPluginsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TntTenantPluginsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TntTenantPluginsDao) Columns() TntTenantPluginsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TntTenantPluginsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TntTenantPluginsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TntTenantPluginsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
