// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// AudRequestLogs is the golang structure for table aud_request_logs.
type AudRequestLogs struct {
	Id                  int64       `json:"id"                    orm:"id"                    description:"主键ID"`                                                           // 主键ID
	TenantId            int64       `json:"tenant_id"             orm:"tenant_id"             description:"租户ID"`                                                           // 租户ID
	UserId              int64       `json:"user_id"               orm:"user_id"               description:"用户ID"`                                                           // 用户ID
	ApiKeyId            int64       `json:"api_key_id"            orm:"api_key_id"            description:"使用的 API Key ID"`                                                 // 使用的 API Key ID
	RequestId           string      `json:"request_id"            orm:"request_id"            description:"请求唯一ID（关联全链路追踪）"`                                                // 请求唯一ID（关联全链路追踪）
	Method              string      `json:"method"                orm:"method"                description:"HTTP 方法（GET/POST/PUT/DELETE）"`                                   // HTTP 方法（GET/POST/PUT/DELETE）
	Path                string      `json:"path"                  orm:"path"                  description:"请求路径"`                                                           // 请求路径
	QueryParams         string      `json:"query_params"          orm:"query_params"          description:"查询参数（URL Query String）"`                                         // 查询参数（URL Query String）
	StatusCode          int         `json:"status_code"           orm:"status_code"           description:"HTTP 响应状态码"`                                                     // HTTP 响应状态码
	ClientIp            string      `json:"client_ip"             orm:"client_ip"             description:"客户端 IP"`                                                         // 客户端 IP
	UserAgent           string      `json:"user_agent"            orm:"user_agent"            description:"客户端 User-Agent"`                                                 // 客户端 User-Agent
	RequestBody         string      `json:"request_body"          orm:"request_body"          description:"请求体（敏感字段脱敏后存储）"`                                                 // 请求体（敏感字段脱敏后存储）
	ResponseBody        string      `json:"response_body"         orm:"response_body"         description:"响应体（截断后存储）"`                                                     // 响应体（截断后存储）
	LatencyMs           int         `json:"latency_ms"            orm:"latency_ms"            description:"请求延迟（毫秒）"`                                                       // 请求延迟（毫秒）
	AuditLevel          string      `json:"audit_level"           orm:"audit_level"           description:"审计级别：full（完整记录）/ masked（脱敏记录）/ question_only（仅记录提问）/ none（不记录）"` // 审计级别：full（完整记录）/ masked（脱敏记录）/ question_only（仅记录提问）/ none（不记录）
	CreatedAt           *gtime.Time `json:"created_at"            orm:"created_at"            description:"创建时间"`                                                           // 创建时间
	UpdatedAt           *gtime.Time `json:"updated_at"            orm:"updated_at"            description:"更新时间"`                                                           // 更新时间
	TenantRequestBody   string      `json:"tenant_request_body"   orm:"tenant_request_body"   description:"租户级请求体（按租户审计级别处理）"`                                              // 租户级请求体（按租户审计级别处理）
	TenantResponseBody  string      `json:"tenant_response_body"  orm:"tenant_response_body"  description:"租户级响应体（按租户审计级别处理）"`                                              // 租户级响应体（按租户审计级别处理）
	TenantAuditLevel    string      `json:"tenant_audit_level"    orm:"tenant_audit_level"    description:"租户审计级别：full/full_text/masked/question_only/none"`                // 租户审计级别：full/full_text/masked/question_only/none
	ProjectId           int64       `json:"project_id"            orm:"project_id"            description:"关联项目ID（通过API Key关联，NULL表示个人密钥无项目）"`                              // 关联项目ID（通过API Key关联，NULL表示个人密钥无项目）
	FirstTokenMs        int         `json:"first_token_ms"        orm:"first_token_ms"        description:"首个 Token 出现的用时（毫秒），仅流式请求有值"`                                     // 首个 Token 出现的用时（毫秒），仅流式请求有值
	RequestHeaders      string      `json:"request_headers"       orm:"request_headers"       description:"请求头信息（仅审计级别为 all 时记录，管理后台调试用）"`                                  // 请求头信息（仅审计级别为 all 时记录，管理后台调试用）
	ResponseHeaders     string      `json:"response_headers"      orm:"response_headers"      description:"响应头信息（仅审计级别为 all 时记录，管理后台调试用）"`                                  // 响应头信息（仅审计级别为 all 时记录，管理后台调试用）
	ForwardingTrace     string      `json:"forwarding_trace"      orm:"forwarding_trace"      description:"请求转发路径追踪（仅管理员可见）"`                                               // 请求转发路径追踪（仅管理员可见）
	TaskId              string      `json:"task_id"               orm:"task_id"               description:"异步任务公开ID（task_xxxxx），关联 tsk_model_tasks.public_task_id"`         // 异步任务公开ID（task_xxxxx），关联 tsk_model_tasks.public_task_id
	TaskStatus          string      `json:"task_status"           orm:"task_status"           description:"异步任务终态：SUCCESS / FAILURE"`                                       // 异步任务终态：SUCCESS / FAILURE
	TaskResult          string      `json:"task_result"           orm:"task_result"           description:"异步任务完成时上游返回的原始响应体"`                                              // 异步任务完成时上游返回的原始响应体
	TaskUpstreamHeaders string      `json:"task_upstream_headers" orm:"task_upstream_headers" description:"异步任务完成时上游返回的响应头（仅审计级别为 full 时记录）"`                               // 异步任务完成时上游返回的响应头（仅审计级别为 full 时记录）
	TaskCompletedAt     *gtime.Time `json:"task_completed_at"     orm:"task_completed_at"     description:"异步任务达到终态的时间"`                                                    // 异步任务达到终态的时间
	AttachmentIds       string      `json:"attachment_ids"        orm:"attachment_ids"        description:"提取的媒体附件 fil_files ID 数组（JSONB: [1,2,3]）"`                        // 提取的媒体附件 fil_files ID 数组（JSONB: [1,2,3]）
}
