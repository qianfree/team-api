// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysCronJobExecutions is the golang structure of table sys_cron_job_executions for DAO operations like Where/Data.
type SysCronJobExecutions struct {
	g.Meta       `orm:"table:sys_cron_job_executions, do:true"`
	Id           any         // 主键ID
	JobName      any         // 任务名称（代码中定义的唯一标识）
	Status       any         // 执行状态：succeeded/failed
	StartedAt    *gtime.Time // 开始执行时间
	FinishedAt   *gtime.Time // 执行完成时间
	DurationMs   any         // 执行耗时（毫秒）
	ErrorMessage any         // 错误消息（仅失败时有值）
	TriggeredBy  any         // 触发方式：auto（自动调度）/ manual（手动触发）
	CreatedAt    *gtime.Time // 创建时间
	UpdatedAt    *gtime.Time // 更新时间
}
