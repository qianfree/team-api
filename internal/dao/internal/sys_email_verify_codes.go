// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysEmailVerifyCodesDao is the data access object for the table sys_email_verify_codes.
type SysEmailVerifyCodesDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  SysEmailVerifyCodesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// SysEmailVerifyCodesColumns defines and stores column names for the table sys_email_verify_codes.
type SysEmailVerifyCodesColumns struct {
	Id        string // 主键ID
	Email     string // 目标邮箱地址
	Code      string // 验证码（6位数字）
	Purpose   string // 用途：register（注册）/ reset_password（重置密码）/ change_email（更换邮箱）
	ExpiresAt string // 过期时间（10分钟有效）
	UsedAt    string // 使用时间（NULL表示未使用）
	CreatedAt string // 创建时间
	UpdatedAt string // 更新时间
}

// sysEmailVerifyCodesColumns holds the columns for the table sys_email_verify_codes.
var sysEmailVerifyCodesColumns = SysEmailVerifyCodesColumns{
	Id:        "id",
	Email:     "email",
	Code:      "code",
	Purpose:   "purpose",
	ExpiresAt: "expires_at",
	UsedAt:    "used_at",
	CreatedAt: "created_at",
	UpdatedAt: "updated_at",
}

// NewSysEmailVerifyCodesDao creates and returns a new DAO object for table data access.
func NewSysEmailVerifyCodesDao(handlers ...gdb.ModelHandler) *SysEmailVerifyCodesDao {
	return &SysEmailVerifyCodesDao{
		group:    "default",
		table:    "sys_email_verify_codes",
		columns:  sysEmailVerifyCodesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysEmailVerifyCodesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysEmailVerifyCodesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysEmailVerifyCodesDao) Columns() SysEmailVerifyCodesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysEmailVerifyCodesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SysEmailVerifyCodesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SysEmailVerifyCodesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
