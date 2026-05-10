// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysErrorLogs is the golang structure of table sys_error_logs for DAO operations like Where/Data.
type SysErrorLogs struct {
	g.Meta       `orm:"table:sys_error_logs, do:true"`
	Id           any         // 主键
	RequestId    any         // 请求ID，用于链路追踪
	ErrorCode    any         // 错误码（HTTP状态码或GoFrame错误码）
	ErrorMessage any         // 错误消息
	StackTrace   any         // 错误堆栈
	HttpMethod   any         // HTTP请求方法
	RequestPath  any         // 请求路径
	RequestBody  any         // 请求体摘要（截断）
	Source       any         // 错误来源：api/panic/cron/background
	Resolved     any         // 是否已处理
	ResolvedBy   any         // 处理人ID
	ResolvedAt   *gtime.Time // 处理时间
	CreatedAt    *gtime.Time // 创建时间
}
