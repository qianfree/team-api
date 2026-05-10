// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TskTaskLogsDao is the data access object for the table tsk_task_logs.
type TskTaskLogsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  TskTaskLogsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// TskTaskLogsColumns defines and stores column names for the table tsk_task_logs.
type TskTaskLogsColumns struct {
	Id        string // 主键ID
	TaskId    string // 关联任务ID
	Level     string // 日志级别：info（信息）/ warn（警告）/ error（错误）
	Message   string // 日志内容
	CreatedAt string // 创建时间
	UpdatedAt string // 更新时间
}

// tskTaskLogsColumns holds the columns for the table tsk_task_logs.
var tskTaskLogsColumns = TskTaskLogsColumns{
	Id:        "id",
	TaskId:    "task_id",
	Level:     "level",
	Message:   "message",
	CreatedAt: "created_at",
	UpdatedAt: "updated_at",
}

// NewTskTaskLogsDao creates and returns a new DAO object for table data access.
func NewTskTaskLogsDao(handlers ...gdb.ModelHandler) *TskTaskLogsDao {
	return &TskTaskLogsDao{
		group:    "default",
		table:    "tsk_task_logs",
		columns:  tskTaskLogsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TskTaskLogsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TskTaskLogsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TskTaskLogsDao) Columns() TskTaskLogsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TskTaskLogsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TskTaskLogsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TskTaskLogsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
