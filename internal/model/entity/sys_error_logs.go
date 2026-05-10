// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysErrorLogs is the golang structure for table sys_error_logs.
type SysErrorLogs struct {
	Id           int64       `json:"id"            orm:"id"            description:"主键"`                             // 主键
	RequestId    string      `json:"request_id"    orm:"request_id"    description:"请求ID，用于链路追踪"`                    // 请求ID，用于链路追踪
	ErrorCode    int         `json:"error_code"    orm:"error_code"    description:"错误码（HTTP状态码或GoFrame错误码）"`        // 错误码（HTTP状态码或GoFrame错误码）
	ErrorMessage string      `json:"error_message" orm:"error_message" description:"错误消息"`                           // 错误消息
	StackTrace   string      `json:"stack_trace"   orm:"stack_trace"   description:"错误堆栈"`                           // 错误堆栈
	HttpMethod   string      `json:"http_method"   orm:"http_method"   description:"HTTP请求方法"`                       // HTTP请求方法
	RequestPath  string      `json:"request_path"  orm:"request_path"  description:"请求路径"`                           // 请求路径
	RequestBody  string      `json:"request_body"  orm:"request_body"  description:"请求体摘要（截断）"`                      // 请求体摘要（截断）
	Source       string      `json:"source"        orm:"source"        description:"错误来源：api/panic/cron/background"` // 错误来源：api/panic/cron/background
	Resolved     bool        `json:"resolved"      orm:"resolved"      description:"是否已处理"`                          // 是否已处理
	ResolvedBy   int64       `json:"resolved_by"   orm:"resolved_by"   description:"处理人ID"`                          // 处理人ID
	ResolvedAt   *gtime.Time `json:"resolved_at"   orm:"resolved_at"   description:"处理时间"`                           // 处理时间
	CreatedAt    *gtime.Time `json:"created_at"    orm:"created_at"    description:"创建时间"`                           // 创建时间
}
