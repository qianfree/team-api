// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AudAiAnalysisJobsDao is the data access object for the table aud_ai_analysis_jobs.
type AudAiAnalysisJobsDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  AudAiAnalysisJobsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// AudAiAnalysisJobsColumns defines and stores column names for the table aud_ai_analysis_jobs.
type AudAiAnalysisJobsColumns struct {
	Id           string //
	ReportId     string // 完成后回填的报告ID
	TenantId     string // 分析目标租户ID
	TargetUserId string // 分析目标用户ID（0=租户全员）
	Source       string // 任务来源：manual=手动 / patrol=定时巡查
	TimeStart    string // 分析窗口起点
	TimeEnd      string // 分析窗口终点
	Status       string // 状态：pending/running/completed/failed
	Progress     string // 进度百分比（已处理单元批次/总批次）
	ErrorMsg     string // 失败原因
	CreatedBy    string // 触发人管理员ID（patrol 为 0）
	CreatedAt    string //
	UpdatedAt    string //
}

// audAiAnalysisJobsColumns holds the columns for the table aud_ai_analysis_jobs.
var audAiAnalysisJobsColumns = AudAiAnalysisJobsColumns{
	Id:           "id",
	ReportId:     "report_id",
	TenantId:     "tenant_id",
	TargetUserId: "target_user_id",
	Source:       "source",
	TimeStart:    "time_start",
	TimeEnd:      "time_end",
	Status:       "status",
	Progress:     "progress",
	ErrorMsg:     "error_msg",
	CreatedBy:    "created_by",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewAudAiAnalysisJobsDao creates and returns a new DAO object for table data access.
func NewAudAiAnalysisJobsDao(handlers ...gdb.ModelHandler) *AudAiAnalysisJobsDao {
	return &AudAiAnalysisJobsDao{
		group:    "default",
		table:    "aud_ai_analysis_jobs",
		columns:  audAiAnalysisJobsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AudAiAnalysisJobsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AudAiAnalysisJobsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AudAiAnalysisJobsDao) Columns() AudAiAnalysisJobsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AudAiAnalysisJobsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AudAiAnalysisJobsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AudAiAnalysisJobsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
