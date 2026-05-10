// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysOptionsDao is the data access object for the table sys_options.
type SysOptionsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  SysOptionsColumns  // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// SysOptionsColumns defines and stores column names for the table sys_options.
type SysOptionsColumns struct {
	Id          string // 主键ID
	Key         string // 配置键（唯一标识，如 site_name、register_enabled）
	Value       string // 配置值
	Description string // 配置说明
	Category    string // 配置分类（如 general、security、email、payment）
	IsPublic    string // 是否公开（前端可直接获取，如站点名称、注册开关）
	CreatedAt   string // 创建时间
	UpdatedAt   string // 更新时间
}

// sysOptionsColumns holds the columns for the table sys_options.
var sysOptionsColumns = SysOptionsColumns{
	Id:          "id",
	Key:         "key",
	Value:       "value",
	Description: "description",
	Category:    "category",
	IsPublic:    "is_public",
	CreatedAt:   "created_at",
	UpdatedAt:   "updated_at",
}

// NewSysOptionsDao creates and returns a new DAO object for table data access.
func NewSysOptionsDao(handlers ...gdb.ModelHandler) *SysOptionsDao {
	return &SysOptionsDao{
		group:    "default",
		table:    "sys_options",
		columns:  sysOptionsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysOptionsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysOptionsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysOptionsDao) Columns() SysOptionsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysOptionsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SysOptionsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SysOptionsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
