// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysAdminRolesDao is the data access object for the table sys_admin_roles.
type SysAdminRolesDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  SysAdminRolesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// SysAdminRolesColumns defines and stores column names for the table sys_admin_roles.
type SysAdminRolesColumns struct {
	Id          string // 主键ID
	Code        string // 角色标识（小写字母/数字/下划线，创建后不可修改，是权限缓存与审计日志的稳定标识；保留字 super_admin 不可占用）
	Name        string // 角色显示名称（可修改）
	Description string // 角色说明，在分配界面展示
	IsBuiltin   string // 是否系统预置：仅用于标识来源与支撑「恢复默认权限」，不构成删除保护（预置角色同样可改可删）
	IsEnabled   string // 是否启用：禁用后该角色的权限对全部关联用户立即失效，无需解除关联
	Sort        string // 排序权重（升序）
	CreatedAt   string // 创建时间
	UpdatedAt   string // 更新时间
}

// sysAdminRolesColumns holds the columns for the table sys_admin_roles.
var sysAdminRolesColumns = SysAdminRolesColumns{
	Id:          "id",
	Code:        "code",
	Name:        "name",
	Description: "description",
	IsBuiltin:   "is_builtin",
	IsEnabled:   "is_enabled",
	Sort:        "sort",
	CreatedAt:   "created_at",
	UpdatedAt:   "updated_at",
}

// NewSysAdminRolesDao creates and returns a new DAO object for table data access.
func NewSysAdminRolesDao(handlers ...gdb.ModelHandler) *SysAdminRolesDao {
	return &SysAdminRolesDao{
		group:    "default",
		table:    "sys_admin_roles",
		columns:  sysAdminRolesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysAdminRolesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysAdminRolesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysAdminRolesDao) Columns() SysAdminRolesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysAdminRolesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SysAdminRolesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SysAdminRolesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
