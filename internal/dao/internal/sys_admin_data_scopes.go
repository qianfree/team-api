// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysAdminDataScopesDao is the data access object for the table sys_admin_data_scopes.
type SysAdminDataScopesDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  SysAdminDataScopesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// SysAdminDataScopesColumns defines and stores column names for the table sys_admin_data_scopes.
type SysAdminDataScopesColumns struct {
	Id          string // 主键ID
	AdminUserId string // 关联的管理员用户ID
	ScopeType   string // 范围类型：all（全部）/ tenant_group（租户组）/ tenant（指定租户）
	ScopeValue  string // 范围值（tenant_group时为组名，tenant时为租户ID列表，逗号分隔）
	CreatedAt   string // 创建时间
	UpdatedAt   string // 更新时间
}

// sysAdminDataScopesColumns holds the columns for the table sys_admin_data_scopes.
var sysAdminDataScopesColumns = SysAdminDataScopesColumns{
	Id:          "id",
	AdminUserId: "admin_user_id",
	ScopeType:   "scope_type",
	ScopeValue:  "scope_value",
	CreatedAt:   "created_at",
	UpdatedAt:   "updated_at",
}

// NewSysAdminDataScopesDao creates and returns a new DAO object for table data access.
func NewSysAdminDataScopesDao(handlers ...gdb.ModelHandler) *SysAdminDataScopesDao {
	return &SysAdminDataScopesDao{
		group:    "default",
		table:    "sys_admin_data_scopes",
		columns:  sysAdminDataScopesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysAdminDataScopesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysAdminDataScopesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysAdminDataScopesDao) Columns() SysAdminDataScopesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysAdminDataScopesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SysAdminDataScopesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SysAdminDataScopesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
