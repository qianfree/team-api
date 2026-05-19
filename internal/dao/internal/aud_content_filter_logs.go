// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AudContentFilterLogsDao is the data access object for the table aud_content_filter_logs.
type AudContentFilterLogsDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  AudContentFilterLogsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// AudContentFilterLogsColumns defines and stores column names for the table aud_content_filter_logs.
type AudContentFilterLogsColumns struct {
	Id              string // 主键ID
	TenantId        string // 租户ID
	UserId          string // 用户ID
	ApiKeyId        string // API Key ID
	RequestId       string // 请求唯一ID
	Method          string // HTTP 方法
	Path            string // 请求路径
	ClientIp        string // 客户端 IP
	FilterMode      string // 过滤模式：log / replace / block
	MatchedWords    string // 命中的敏感词列表（JSONB 数组）
	OriginalSnippet string // 原始请求体片段（截断存储，仅 replace 模式）
	Blocked         string // 是否被拦截（mode=block 时为 true）
	CreatedAt       string // 创建时间
	ProjectId       string // 项目ID
}

// audContentFilterLogsColumns holds the columns for the table aud_content_filter_logs.
var audContentFilterLogsColumns = AudContentFilterLogsColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	UserId:          "user_id",
	ApiKeyId:        "api_key_id",
	RequestId:       "request_id",
	Method:          "method",
	Path:            "path",
	ClientIp:        "client_ip",
	FilterMode:      "filter_mode",
	MatchedWords:    "matched_words",
	OriginalSnippet: "original_snippet",
	Blocked:         "blocked",
	CreatedAt:       "created_at",
	ProjectId:       "project_id",
}

// NewAudContentFilterLogsDao creates and returns a new DAO object for table data access.
func NewAudContentFilterLogsDao(handlers ...gdb.ModelHandler) *AudContentFilterLogsDao {
	return &AudContentFilterLogsDao{
		group:    "default",
		table:    "aud_content_filter_logs",
		columns:  audContentFilterLogsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AudContentFilterLogsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AudContentFilterLogsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AudContentFilterLogsDao) Columns() AudContentFilterLogsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AudContentFilterLogsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AudContentFilterLogsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AudContentFilterLogsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
