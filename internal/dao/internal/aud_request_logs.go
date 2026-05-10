// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AudRequestLogsDao is the data access object for the table aud_request_logs.
type AudRequestLogsDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  AudRequestLogsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// AudRequestLogsColumns defines and stores column names for the table aud_request_logs.
type AudRequestLogsColumns struct {
	Id                 string // 主键ID
	TenantId           string // 租户ID
	UserId             string // 用户ID
	ApiKeyId           string // 使用的 API Key ID
	RequestId          string // 请求唯一ID（关联全链路追踪）
	Method             string // HTTP 方法（GET/POST/PUT/DELETE）
	Path               string // 请求路径
	QueryParams        string // 查询参数（URL Query String）
	StatusCode         string // HTTP 响应状态码
	ClientIp           string // 客户端 IP
	UserAgent          string // 客户端 User-Agent
	RequestBody        string // 请求体（敏感字段脱敏后存储）
	ResponseBody       string // 响应体（截断后存储）
	LatencyMs          string // 请求延迟（毫秒）
	AuditLevel         string // 审计级别：full（完整记录）/ masked（脱敏记录）/ question_only（仅记录提问）/ none（不记录）
	CreatedAt          string // 创建时间
	UpdatedAt          string // 更新时间
	TenantRequestBody  string // 租户级请求体（按租户审计级别处理）
	TenantResponseBody string // 租户级响应体（按租户审计级别处理）
	TenantAuditLevel   string // 租户审计级别：full/full_text/masked/question_only/none
	ProjectId          string // 关联项目ID（通过API Key关联，NULL表示个人密钥无项目）
	FirstTokenMs       string // 首个 Token 出现的用时（毫秒），仅流式请求有值
	RequestHeaders     string // 请求头信息（仅审计级别为 all 时记录，管理后台调试用）
	ResponseHeaders    string // 响应头信息（仅审计级别为 all 时记录，管理后台调试用）
	ForwardingTrace    string // 请求转发路径追踪（仅管理员可见）
}

// audRequestLogsColumns holds the columns for the table aud_request_logs.
var audRequestLogsColumns = AudRequestLogsColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	UserId:             "user_id",
	ApiKeyId:           "api_key_id",
	RequestId:          "request_id",
	Method:             "method",
	Path:               "path",
	QueryParams:        "query_params",
	StatusCode:         "status_code",
	ClientIp:           "client_ip",
	UserAgent:          "user_agent",
	RequestBody:        "request_body",
	ResponseBody:       "response_body",
	LatencyMs:          "latency_ms",
	AuditLevel:         "audit_level",
	CreatedAt:          "created_at",
	UpdatedAt:          "updated_at",
	TenantRequestBody:  "tenant_request_body",
	TenantResponseBody: "tenant_response_body",
	TenantAuditLevel:   "tenant_audit_level",
	ProjectId:          "project_id",
	FirstTokenMs:       "first_token_ms",
	RequestHeaders:     "request_headers",
	ResponseHeaders:    "response_headers",
	ForwardingTrace:    "forwarding_trace",
}

// NewAudRequestLogsDao creates and returns a new DAO object for table data access.
func NewAudRequestLogsDao(handlers ...gdb.ModelHandler) *AudRequestLogsDao {
	return &AudRequestLogsDao{
		group:    "default",
		table:    "aud_request_logs",
		columns:  audRequestLogsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AudRequestLogsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AudRequestLogsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AudRequestLogsDao) Columns() AudRequestLogsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AudRequestLogsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AudRequestLogsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AudRequestLogsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
