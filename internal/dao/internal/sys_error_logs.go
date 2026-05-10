// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysErrorLogsDao is the data access object for the table sys_error_logs.
type SysErrorLogsDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  SysErrorLogsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// SysErrorLogsColumns defines and stores column names for the table sys_error_logs.
type SysErrorLogsColumns struct {
	Id           string // 主键
	RequestId    string // 请求ID，用于链路追踪
	ErrorCode    string // 错误码（HTTP状态码或GoFrame错误码）
	ErrorMessage string // 错误消息
	StackTrace   string // 错误堆栈
	HttpMethod   string // HTTP请求方法
	RequestPath  string // 请求路径
	RequestBody  string // 请求体摘要（截断）
	Source       string // 错误来源：api/panic/cron/background
	Resolved     string // 是否已处理
	ResolvedBy   string // 处理人ID
	ResolvedAt   string // 处理时间
	CreatedAt    string // 创建时间
}

// sysErrorLogsColumns holds the columns for the table sys_error_logs.
var sysErrorLogsColumns = SysErrorLogsColumns{
	Id:           "id",
	RequestId:    "request_id",
	ErrorCode:    "error_code",
	ErrorMessage: "error_message",
	StackTrace:   "stack_trace",
	HttpMethod:   "http_method",
	RequestPath:  "request_path",
	RequestBody:  "request_body",
	Source:       "source",
	Resolved:     "resolved",
	ResolvedBy:   "resolved_by",
	ResolvedAt:   "resolved_at",
	CreatedAt:    "created_at",
}

// NewSysErrorLogsDao creates and returns a new DAO object for table data access.
func NewSysErrorLogsDao(handlers ...gdb.ModelHandler) *SysErrorLogsDao {
	return &SysErrorLogsDao{
		group:    "default",
		table:    "sys_error_logs",
		columns:  sysErrorLogsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysErrorLogsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysErrorLogsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysErrorLogsDao) Columns() SysErrorLogsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysErrorLogsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SysErrorLogsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SysErrorLogsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
