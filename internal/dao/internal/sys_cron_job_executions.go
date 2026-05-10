// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysCronJobExecutionsDao is the data access object for the table sys_cron_job_executions.
type SysCronJobExecutionsDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  SysCronJobExecutionsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// SysCronJobExecutionsColumns defines and stores column names for the table sys_cron_job_executions.
type SysCronJobExecutionsColumns struct {
	Id           string // 主键ID
	JobName      string // 任务名称（代码中定义的唯一标识）
	Status       string // 执行状态：succeeded/failed
	StartedAt    string // 开始执行时间
	FinishedAt   string // 执行完成时间
	DurationMs   string // 执行耗时（毫秒）
	ErrorMessage string // 错误消息（仅失败时有值）
	TriggeredBy  string // 触发方式：auto（自动调度）/ manual（手动触发）
	CreatedAt    string // 创建时间
	UpdatedAt    string // 更新时间
}

// sysCronJobExecutionsColumns holds the columns for the table sys_cron_job_executions.
var sysCronJobExecutionsColumns = SysCronJobExecutionsColumns{
	Id:           "id",
	JobName:      "job_name",
	Status:       "status",
	StartedAt:    "started_at",
	FinishedAt:   "finished_at",
	DurationMs:   "duration_ms",
	ErrorMessage: "error_message",
	TriggeredBy:  "triggered_by",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewSysCronJobExecutionsDao creates and returns a new DAO object for table data access.
func NewSysCronJobExecutionsDao(handlers ...gdb.ModelHandler) *SysCronJobExecutionsDao {
	return &SysCronJobExecutionsDao{
		group:    "default",
		table:    "sys_cron_job_executions",
		columns:  sysCronJobExecutionsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysCronJobExecutionsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysCronJobExecutionsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysCronJobExecutionsDao) Columns() SysCronJobExecutionsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysCronJobExecutionsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SysCronJobExecutionsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SysCronJobExecutionsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
