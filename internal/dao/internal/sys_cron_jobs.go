// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysCronJobsDao is the data access object for the table sys_cron_jobs.
type SysCronJobsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  SysCronJobsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// SysCronJobsColumns defines and stores column names for the table sys_cron_jobs.
type SysCronJobsColumns struct {
	Id               string //
	JobName          string // 任务名称（代码中定义的唯一标识）
	Schedule         string // cron 表达式
	LastStatus       string // 最近一次执行状态：succeeded/failed
	LastStartedAt    string // 最近一次开始执行时间
	LastFinishedAt   string // 最近一次执行完成时间
	LastDurationMs   string // 最近一次执行耗时（毫秒）
	LastErrorMessage string // 最近一次错误信息（仅失败时有值）
	LastTriggeredBy  string // 最近一次触发方式：auto/manual
	TotalRuns        string // 累计执行次数
	TotalFailures    string // 累计失败次数
	CreatedAt        string //
	UpdatedAt        string //
}

// sysCronJobsColumns holds the columns for the table sys_cron_jobs.
var sysCronJobsColumns = SysCronJobsColumns{
	Id:               "id",
	JobName:          "job_name",
	Schedule:         "schedule",
	LastStatus:       "last_status",
	LastStartedAt:    "last_started_at",
	LastFinishedAt:   "last_finished_at",
	LastDurationMs:   "last_duration_ms",
	LastErrorMessage: "last_error_message",
	LastTriggeredBy:  "last_triggered_by",
	TotalRuns:        "total_runs",
	TotalFailures:    "total_failures",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
}

// NewSysCronJobsDao creates and returns a new DAO object for table data access.
func NewSysCronJobsDao(handlers ...gdb.ModelHandler) *SysCronJobsDao {
	return &SysCronJobsDao{
		group:    "default",
		table:    "sys_cron_jobs",
		columns:  sysCronJobsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysCronJobsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysCronJobsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysCronJobsDao) Columns() SysCronJobsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysCronJobsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SysCronJobsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SysCronJobsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
