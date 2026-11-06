// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AudAiAnalysisFindingsDao is the data access object for the table aud_ai_analysis_findings.
type AudAiAnalysisFindingsDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  AudAiAnalysisFindingsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// AudAiAnalysisFindingsColumns defines and stores column names for the table aud_ai_analysis_findings.
type AudAiAnalysisFindingsColumns struct {
	Id          string //
	ReportId    string // 所属报告ID（逻辑关联 aud_ai_analysis_reports.id）
	UnitType    string // 单元类型：session=会话/内容链单元 / standalone=独立请求
	SessionKey  string // 关联的会话稳定键（sk:source:hash，本身即哈希无敏感明文；独立单元为空）
	RequestId   string // 关联的请求唯一ID（证据所在轮次，回链审计日志详情）
	AnchorLogId string // 会话锚点日志ID（用于调 LoadConversationView 还原完整会话）
	Dimension   string // 判定维度：compliance=合规 / leakage=泄露
	Severity    string // 严重度：high/medium/low
	Category    string // 细分类目：phishing/fraud/malware/illegal_content/evasion/credential_leak/source_code_leak/pii_leak/internal_doc_leak/other
	Evidence    string // 证据片段（原文摘录，落库前经 MaskSensitiveData 脱敏）
	Explanation string // AI 判定理由
	CreatedAt   string //
}

// audAiAnalysisFindingsColumns holds the columns for the table aud_ai_analysis_findings.
var audAiAnalysisFindingsColumns = AudAiAnalysisFindingsColumns{
	Id:          "id",
	ReportId:    "report_id",
	UnitType:    "unit_type",
	SessionKey:  "session_key",
	RequestId:   "request_id",
	AnchorLogId: "anchor_log_id",
	Dimension:   "dimension",
	Severity:    "severity",
	Category:    "category",
	Evidence:    "evidence",
	Explanation: "explanation",
	CreatedAt:   "created_at",
}

// NewAudAiAnalysisFindingsDao creates and returns a new DAO object for table data access.
func NewAudAiAnalysisFindingsDao(handlers ...gdb.ModelHandler) *AudAiAnalysisFindingsDao {
	return &AudAiAnalysisFindingsDao{
		group:    "default",
		table:    "aud_ai_analysis_findings",
		columns:  audAiAnalysisFindingsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AudAiAnalysisFindingsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AudAiAnalysisFindingsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AudAiAnalysisFindingsDao) Columns() AudAiAnalysisFindingsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AudAiAnalysisFindingsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AudAiAnalysisFindingsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AudAiAnalysisFindingsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
