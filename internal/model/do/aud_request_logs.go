// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// AudRequestLogs is the golang structure of table aud_request_logs for DAO operations like Where/Data.
type AudRequestLogs struct {
	g.Meta              `orm:"table:aud_request_logs, do:true"`
	Id                  any         // 主键ID
	TenantId            any         // 租户ID
	UserId              any         // 用户ID
	ApiKeyId            any         // 使用的 API Key ID
	RequestId           any         // 请求唯一ID（关联全链路追踪）
	Method              any         // HTTP 方法（GET/POST/PUT/DELETE）
	Path                any         // 请求路径
	QueryParams         any         // 查询参数（URL Query String）
	StatusCode          any         // HTTP 响应状态码
	ClientIp            any         // 客户端 IP
	UserAgent           any         // 客户端 User-Agent
	RequestBody         any         // 请求体（敏感字段脱敏后存储）
	ResponseBody        any         // 响应体（截断后存储）
	LatencyMs           any         // 请求延迟（毫秒）
	AuditLevel          any         // 审计级别：full（完整记录）/ masked（脱敏记录）/ question_only（仅记录提问）/ none（不记录）
	CreatedAt           *gtime.Time // 创建时间
	UpdatedAt           *gtime.Time // 更新时间
	TenantRequestBody   any         // 租户级请求体（按租户审计级别处理）
	TenantResponseBody  any         // 租户级响应体（按租户审计级别处理）
	TenantAuditLevel    any         // 租户审计级别：full/full_text/masked/question_only/none
	ProjectId           any         // 关联项目ID（通过API Key关联，NULL表示个人密钥无项目）
	FirstTokenMs        any         // 首个 Token 出现的用时（毫秒），仅流式请求有值
	RequestHeaders      any         // 请求头信息（仅审计级别为 all 时记录，管理后台调试用）
	ResponseHeaders     any         // 响应头信息（仅审计级别为 all 时记录，管理后台调试用）
	ForwardingTrace     any         // 请求转发路径追踪（仅管理员可见）
	TaskId              any         // 异步任务公开ID（task_xxxxx），关联 tsk_model_tasks.public_task_id
	TaskStatus          any         // 异步任务终态：SUCCESS / FAILURE
	TaskResult          any         // 异步任务完成时上游返回的原始响应体
	TaskUpstreamHeaders any         // 异步任务完成时上游返回的响应头（仅审计级别为 full 时记录）
	TaskCompletedAt     *gtime.Time // 异步任务达到终态的时间
	AttachmentIds       any         // 提取的媒体附件 fil_files ID 数组（JSONB: [1,2,3]）
	Model               any         // 请求使用的模型名称（从请求体或 URL 中提取）
}
