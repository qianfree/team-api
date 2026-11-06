// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AudAiAnalysisReportsDao is the data access object for the table aud_ai_analysis_reports.
type AudAiAnalysisReportsDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  AudAiAnalysisReportsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// AudAiAnalysisReportsColumns defines and stores column names for the table aud_ai_analysis_reports.
type AudAiAnalysisReportsColumns struct {
	Id              string //
	TenantId        string // 分析目标租户ID
	TargetUserId    string // 分析目标用户ID（0=租户全员）
	Source          string // 报告来源：manual=手动分析 / patrol=定时巡查
	TimeStart       string // 分析窗口起点
	TimeEnd         string // 分析窗口终点
	RiskLevel       string // 整体风险等级：high/medium/low/clean/unknown
	AuditLevel      string // 分析时数据实际的审计级别（数据完整性声明）
	LogCount        string // 覆盖的原始审计日志条数
	SessionCount    string // 会话/内容链单元数（去重后）
	StandaloneCount string // 独立单发单元数（无会话键）
	FindingCount    string // 发现总数（冗余计数，列表展示用）
	Summary         string // AI 生成的整体评估摘要（脱敏后）
	AiModel         string // 使用的分析模型（快照）
	AiTokensUsed    string // 分析消耗的总 token（快照）
	CreatedBy       string // 触发人管理员ID（manual 时有效，patrol 为 0）
	CreatedAt       string //
	UpdatedAt       string //
}

// audAiAnalysisReportsColumns holds the columns for the table aud_ai_analysis_reports.
var audAiAnalysisReportsColumns = AudAiAnalysisReportsColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	TargetUserId:    "target_user_id",
	Source:          "source",
	TimeStart:       "time_start",
	TimeEnd:         "time_end",
	RiskLevel:       "risk_level",
	AuditLevel:      "audit_level",
	LogCount:        "log_count",
	SessionCount:    "session_count",
	StandaloneCount: "standalone_count",
	FindingCount:    "finding_count",
	Summary:         "summary",
	AiModel:         "ai_model",
	AiTokensUsed:    "ai_tokens_used",
	CreatedBy:       "created_by",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewAudAiAnalysisReportsDao creates and returns a new DAO object for table data access.
func NewAudAiAnalysisReportsDao(handlers ...gdb.ModelHandler) *AudAiAnalysisReportsDao {
	return &AudAiAnalysisReportsDao{
		group:    "default",
		table:    "aud_ai_analysis_reports",
		columns:  audAiAnalysisReportsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AudAiAnalysisReportsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AudAiAnalysisReportsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AudAiAnalysisReportsDao) Columns() AudAiAnalysisReportsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AudAiAnalysisReportsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AudAiAnalysisReportsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AudAiAnalysisReportsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
