// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysPluginsDao is the data access object for the table sys_plugins.
type SysPluginsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  SysPluginsColumns  // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// SysPluginsColumns defines and stores column names for the table sys_plugins.
type SysPluginsColumns struct {
	Id        string //
	Name      string // 插件唯一标识，如 email-report
	Label     string // 显示名称
	Version   string // 当前安装版本
	Status    string // 状态：registered=已注册, installed=已安装, enabled=已启用, disabled=已禁用, error=异常
	Category  string // 分类：relay=代理扩展, middleware=中间件, billing=计费, notification=通知, extension=通用扩展
	Config    string // 插件全局配置（JSON）
	ErrorMsg  string // 异常信息
	CreatedAt string //
	UpdatedAt string //
}

// sysPluginsColumns holds the columns for the table sys_plugins.
var sysPluginsColumns = SysPluginsColumns{
	Id:        "id",
	Name:      "name",
	Label:     "label",
	Version:   "version",
	Status:    "status",
	Category:  "category",
	Config:    "config",
	ErrorMsg:  "error_msg",
	CreatedAt: "created_at",
	UpdatedAt: "updated_at",
}

// NewSysPluginsDao creates and returns a new DAO object for table data access.
func NewSysPluginsDao(handlers ...gdb.ModelHandler) *SysPluginsDao {
	return &SysPluginsDao{
		group:    "default",
		table:    "sys_plugins",
		columns:  sysPluginsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysPluginsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysPluginsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysPluginsDao) Columns() SysPluginsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysPluginsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SysPluginsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SysPluginsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
