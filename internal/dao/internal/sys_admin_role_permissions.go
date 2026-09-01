// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysAdminRolePermissionsDao is the data access object for the table sys_admin_role_permissions.
type SysAdminRolePermissionsDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  SysAdminRolePermissionsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// SysAdminRolePermissionsColumns defines and stores column names for the table sys_admin_role_permissions.
type SysAdminRolePermissionsColumns struct {
	Id              string // 主键ID
	RoleId          string // 角色ID（逻辑关联 sys_admin_roles.id，无外键，删除角色时由业务层级联清理）
	PermissionPoint string // 权限点标识（如 tenant:create、channel:edit）
	CreatedAt       string // 创建时间
	UpdatedAt       string // 更新时间
}

// sysAdminRolePermissionsColumns holds the columns for the table sys_admin_role_permissions.
var sysAdminRolePermissionsColumns = SysAdminRolePermissionsColumns{
	Id:              "id",
	RoleId:          "role_id",
	PermissionPoint: "permission_point",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewSysAdminRolePermissionsDao creates and returns a new DAO object for table data access.
func NewSysAdminRolePermissionsDao(handlers ...gdb.ModelHandler) *SysAdminRolePermissionsDao {
	return &SysAdminRolePermissionsDao{
		group:    "default",
		table:    "sys_admin_role_permissions",
		columns:  sysAdminRolePermissionsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysAdminRolePermissionsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysAdminRolePermissionsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysAdminRolePermissionsDao) Columns() SysAdminRolePermissionsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysAdminRolePermissionsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SysAdminRolePermissionsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SysAdminRolePermissionsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
