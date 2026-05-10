// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysAdminRolePermsDao is the data access object for the table sys_admin_role_perms.
type SysAdminRolePermsDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  SysAdminRolePermsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// SysAdminRolePermsColumns defines and stores column names for the table sys_admin_role_perms.
type SysAdminRolePermsColumns struct {
	Id              string // 主键ID
	AdminUserId     string // 关联的管理员用户ID
	PermissionPoint string // 权限点标识（如 tenant:create、channel:edit）
	CreatedAt       string // 创建时间
	UpdatedAt       string // 更新时间
}

// sysAdminRolePermsColumns holds the columns for the table sys_admin_role_perms.
var sysAdminRolePermsColumns = SysAdminRolePermsColumns{
	Id:              "id",
	AdminUserId:     "admin_user_id",
	PermissionPoint: "permission_point",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewSysAdminRolePermsDao creates and returns a new DAO object for table data access.
func NewSysAdminRolePermsDao(handlers ...gdb.ModelHandler) *SysAdminRolePermsDao {
	return &SysAdminRolePermsDao{
		group:    "default",
		table:    "sys_admin_role_perms",
		columns:  sysAdminRolePermsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysAdminRolePermsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysAdminRolePermsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysAdminRolePermsDao) Columns() SysAdminRolePermsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysAdminRolePermsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SysAdminRolePermsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SysAdminRolePermsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
