// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TskTasksDao is the data access object for the table tsk_tasks.
type TskTasksDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  TskTasksColumns    // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// TskTasksColumns defines and stores column names for the table tsk_tasks.
type TskTasksColumns struct {
	Id           string // 主键ID
	Name         string // 任务名称（如 "发送邮件"、"日对账"、"数据导出"）
	Handler      string // Handler 函数路径（用于任务路由）
	Status       string // 状态：pending（待执行）/ running（执行中）/ succeeded（成功）/ failed（失败）/ cancelled（已取消）
	Payload      string // 任务输入参数（JSONB）
	Result       string // 任务执行结果（JSONB）
	MaxRetries   string // 最大重试次数
	RetryCount   string // 已重试次数
	StartedAt    string // 开始执行时间
	FinishedAt   string // 执行完成时间
	ScheduledAt  string // 计划执行时间（用于定时任务）
	ErrorMessage string // 失败时的错误信息
	CreatedAt    string // 创建时间
	UpdatedAt    string // 更新时间
}

// tskTasksColumns holds the columns for the table tsk_tasks.
var tskTasksColumns = TskTasksColumns{
	Id:           "id",
	Name:         "name",
	Handler:      "handler",
	Status:       "status",
	Payload:      "payload",
	Result:       "result",
	MaxRetries:   "max_retries",
	RetryCount:   "retry_count",
	StartedAt:    "started_at",
	FinishedAt:   "finished_at",
	ScheduledAt:  "scheduled_at",
	ErrorMessage: "error_message",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewTskTasksDao creates and returns a new DAO object for table data access.
func NewTskTasksDao(handlers ...gdb.ModelHandler) *TskTasksDao {
	return &TskTasksDao{
		group:    "default",
		table:    "tsk_tasks",
		columns:  tskTasksColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TskTasksDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TskTasksDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TskTasksDao) Columns() TskTasksColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TskTasksDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TskTasksDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TskTasksDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
