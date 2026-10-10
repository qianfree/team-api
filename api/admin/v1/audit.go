package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

// === 审计管理 ===

type AuditConfigGetReq struct {
	g.Meta `path:"/audit/config" method:"get" mime:"json" tags:"管理后台-审计" summary:"获取审计配置"`
}

type AuditConfigGetRes struct {
	AuditLevel string `json:"audit_level"`
}

type AuditConfigUpdateReq struct {
	g.Meta     `path:"/audit/config" method:"put" mime:"json" tags:"管理后台-审计" summary:"更新审计配置"`
	AuditLevel string `json:"audit_level" v:"required|in:full,full_text,masked,question_only,none"`
}

type AuditConfigUpdateRes struct{}

type OperationLogListReq struct {
	g.Meta    `path:"/audit/operation-logs" method:"get" mime:"json" tags:"管理后台-审计" summary:"操作日志列表"`
	Page      int    `json:"page" d:"1"`
	PageSize  int    `json:"page_size" d:"20"`
	UserID    int    `json:"user_id" dc:"用户ID"`
	UserType  string `json:"user_type" dc:"用户类型"`
	Action    string `json:"action" dc:"操作类型"`
	StartDate string `json:"start_date" dc:"开始日期"`
	EndDate   string `json:"end_date" dc:"结束日期"`
}

type OperationLogListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type SensitiveLogListReq struct {
	g.Meta       `path:"/audit/sensitive-logs" method:"get" mime:"json" tags:"管理后台-审计" summary:"敏感访问日志"`
	Page         int    `json:"page" d:"1"`
	PageSize     int    `json:"page_size" d:"20"`
	UserID       int    `json:"user_id" dc:"用户ID"`
	ResourceType string `json:"resource_type" dc:"资源类型"`
	StartDate    string `json:"start_date" dc:"开始日期 YYYY-MM-DD"`
	EndDate      string `json:"end_date" dc:"结束日期 YYYY-MM-DD"`
}

type SensitiveLogListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type RequestAuditLogListReq struct {
	g.Meta        `path:"/audit/request-logs" method:"get" mime:"json" tags:"管理后台-审计" summary:"请求审计日志"`
	Page          int    `json:"page" d:"1"`
	PageSize      int    `json:"page_size" d:"20"`
	ID            int64  `json:"id" dc:"日志ID"`
	TenantID      int    `json:"tenant_id" dc:"租户ID"`
	UserID        int64  `json:"user_id" dc:"用户ID"`
	ApiKeyID      int    `json:"api_key_id" dc:"API Key ID"`
	Username      string `json:"username" dc:"用户名（模糊匹配）"`
	RequestId     string `json:"request_id" dc:"Request ID（精确匹配）"`
	TaskId        string `json:"task_id" dc:"异步任务ID（精确匹配）"`
	Method        string `json:"method" dc:"HTTP 方法"`
	Path          string `json:"path" dc:"请求路径"`
	Model         string `json:"model" dc:"模型名称（精确匹配）"`
	StatusCode    int    `json:"status_code" dc:"状态码"`
	SessionKey    string `json:"session_key" dc:"会话关联键（精确匹配）"`
	SessionSource string `json:"session_source" dc:"会话键来源：header/anthropic/openai"`
	StartDate     string `json:"start_date" dc:"开始时间（YYYY-MM-DD 或 YYYY-MM-DD HH:mm:ss）"`
	EndDate       string `json:"end_date" dc:"结束时间（YYYY-MM-DD 或 YYYY-MM-DD HH:mm:ss）"`
}

type RequestAuditLogListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type RequestAuditLogDetailReq struct {
	g.Meta `path:"/audit/request-logs/{id}" method:"get" mime:"json" tags:"管理后台-审计" summary:"请求审计日志详情"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type RequestAuditLogDetailRes struct {
	Data map[string]any `json:"-"`
}

func (r *RequestAuditLogDetailRes) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Data)
}

// RequestAuditLogConversationReq 会话视图：从锚点审计记录还原完整对话
// （按三层降级关联：会话键 → 内容链 → 身份推断，段/分支/缺失标注齐全）
type RequestAuditLogConversationReq struct {
	g.Meta `path:"/audit/request-logs/{id}/conversation" method:"get" mime:"json" tags:"管理后台-审计" summary:"审计日志会话视图（还原完整对话）"`
	Id     int64 `json:"id" in:"path" v:"required|min:1" dc:"锚点审计记录ID"`
}

// RequestAuditLogConversationRes 会话重组结果平铺（json:"-" + MarshalJSON 避免双层 data）
type RequestAuditLogConversationRes struct {
	Data map[string]any `json:"-"`
}

func (r *RequestAuditLogConversationRes) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Data)
}

// ForwardingTraceGetReq 按 request_id 查询请求转发路径追踪（用量日志详情懒加载用，仅取 trace 列）。
type ForwardingTraceGetReq struct {
	g.Meta    `path:"/audit/forwarding-trace/{request_id}" method:"get" mime:"json" tags:"管理后台-审计" summary:"按 request_id 查转发路径追踪"`
	RequestId string `json:"request_id" in:"path" v:"required" dc:"请求ID"`
}

// ForwardingTraceGetRes 返回已解析的 trace 对象（api 包不导入 relay/common，logic 层 json.Unmarshal）。
type ForwardingTraceGetRes struct {
	Found           bool           `json:"found"`            // aud_request_logs 是否有记录（区分「未审计」与「无 trace」）
	AuditLevel      string         `json:"audit_level"`      // 审计级别，便于前端展示原因
	ForwardingTrace map[string]any `json:"forwarding_trace"` // 已解析对象；无 trace 时为 nil
}

// OperationLogExportReq 导出操作日志请求
type OperationLogExportReq struct {
	g.Meta    `path:"/audit/operation-logs/export" method:"get" mime:"json" tags:"管理后台-审计" summary:"导出操作日志"`
	Format    string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	UserID    int    `json:"user_id" in:"query" dc:"用户ID"`
	UserType  string `json:"user_type" in:"query" dc:"用户类型"`
	Action    string `json:"action" in:"query" dc:"操作类型"`
	StartDate string `json:"start_date" in:"query" dc:"开始日期"`
	EndDate   string `json:"end_date" in:"query" dc:"结束日期"`
}

type OperationLogExportRes struct{}

// === 内容过滤拦截日志 ===

type ContentFilterLogListReq struct {
	g.Meta    `path:"/audit/content-filter-logs" method:"get" mime:"json" tags:"管理后台-审计" summary:"内容过滤拦截日志"`
	Page      int    `json:"page" d:"1"`
	PageSize  int    `json:"page_size" d:"20"`
	TenantID  int    `json:"tenant_id" dc:"租户ID"`
	Mode      string `json:"mode" dc:"过滤模式：log/replace/block"`
	Blocked   string `json:"blocked" dc:"是否被拦截：true/false"`
	StartDate string `json:"start_date" dc:"开始日期"`
	EndDate   string `json:"end_date" dc:"结束日期"`
	Keyword   string `json:"keyword" dc:"关键词搜索（敏感词/路径）"`
}

type ContentFilterLogListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// ContentFilterLogClearReq 清空全部内容过滤拦截日志（硬删除，用于日志大量堆积时快速释放数据库空间）
type ContentFilterLogClearReq struct {
	g.Meta `path:"/audit/content-filter-logs/clear" method:"delete" mime:"json" tags:"管理后台-审计" summary:"清空内容过滤拦截日志（硬删除全部）"`
}

// ContentFilterLogClearRes 清空内容过滤拦截日志响应
type ContentFilterLogClearRes struct {
	Deleted int64 `json:"deleted" dc:"删除的记录数"`
}
