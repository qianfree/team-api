// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysAdminUserRolesDao is the data access object for the table sys_admin_user_roles.
type SysAdminUserRolesDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  SysAdminUserRolesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// SysAdminUserRolesColumns defines and stores column names for the table sys_admin_user_roles.
type SysAdminUserRolesColumns struct {
	Id          string // 主键ID
	AdminUserId string // 管理员用户ID（逻辑关联 sys_admin_users.id，无外键）
	RoleId      string // 角色ID（逻辑关联 sys_admin_roles.id，无外键）
	CreatedAt   string // 创建时间
	UpdatedAt   string // 更新时间
}

// sysAdminUserRolesColumns holds the columns for the table sys_admin_user_roles.
var sysAdminUserRolesColumns = SysAdminUserRolesColumns{
	Id:          "id",
	AdminUserId: "admin_user_id",
	RoleId:      "role_id",
	CreatedAt:   "created_at",
	UpdatedAt:   "updated_at",
}

// NewSysAdminUserRolesDao creates and returns a new DAO object for table data access.
func NewSysAdminUserRolesDao(handlers ...gdb.ModelHandler) *SysAdminUserRolesDao {
	return &SysAdminUserRolesDao{
		group:    "default",
		table:    "sys_admin_user_roles",
		columns:  sysAdminUserRolesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysAdminUserRolesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysAdminUserRolesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysAdminUserRolesDao) Columns() SysAdminUserRolesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysAdminUserRolesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SysAdminUserRolesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SysAdminUserRolesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
