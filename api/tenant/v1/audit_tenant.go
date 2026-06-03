package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

// === 租户审计 ===

type TenantAuditConfigGetReq struct {
	g.Meta `path:"/audit/config" method:"get" mime:"json" tags:"租户控制台-审计" summary:"审计配置"`
}

type TenantAuditConfigGetRes struct {
	AuditLevel string `json:"audit_level"`
}

type TenantAuditConfigUpdateReq struct {
	g.Meta     `path:"/audit/config" method:"put" mime:"json" tags:"租户控制台-审计" summary:"更新审计配置"`
	AuditLevel string `json:"audit_level" v:"required"`
}

type TenantAuditConfigUpdateRes struct{}

type TenantAuditLogsReq struct {
	g.Meta   `path:"/audit/logs" method:"get" mime:"json" tags:"租户控制台-审计" summary:"审计日志"`
	Page     int `json:"page" d:"1"`
	PageSize int `json:"page_size" d:"20"`
}

type TenantAuditLogsRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// === 租户请求审计日志（API 调用的输入输出） ===

type TenantRequestAuditLogsReq struct {
	g.Meta     `path:"/audit/request-logs" method:"get" mime:"json" tags:"租户控制台-审计" summary:"请求审计日志"`
	Page       int    `json:"page" in:"query" d:"1" v:"min:1"`
	PageSize   int    `json:"page_size" in:"query" d:"20" v:"min:1|max:100"`
	Username   string `json:"username" in:"query" dc:"用户名（模糊匹配）"`
	RequestId  string `json:"request_id" in:"query" dc:"Request ID（精确匹配）"`
	TaskId     string `json:"task_id" in:"query" dc:"异步任务ID（精确匹配）"`
	Path       string `json:"path" in:"query" dc:"请求路径（模糊匹配）"`
	StatusCode int    `json:"status_code" in:"query" dc:"HTTP 状态码"`
	StartDate  string `json:"start_date" in:"query" dc:"开始日期"`
	EndDate    string `json:"end_date" in:"query" dc:"结束日期"`
}

type TenantRequestAuditLogsRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type TenantRequestAuditLogDetailReq struct {
	g.Meta `path:"/audit/request-logs/{id}" method:"get" mime:"json" tags:"租户控制台-审计" summary:"请求审计日志详情"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantRequestAuditLogDetailRes struct {
	Data map[string]any `json:"data"`
}
