// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysCronJobs is the golang structure of table sys_cron_jobs for DAO operations like Where/Data.
type SysCronJobs struct {
	g.Meta           `orm:"table:sys_cron_jobs, do:true"`
	Id               any         //
	JobName          any         // 任务名称（代码中定义的唯一标识）
	Schedule         any         // cron 表达式
	LastStatus       any         // 最近一次执行状态：succeeded/failed
	LastStartedAt    *gtime.Time // 最近一次开始执行时间
	LastFinishedAt   *gtime.Time // 最近一次执行完成时间
	LastDurationMs   any         // 最近一次执行耗时（毫秒）
	LastErrorMessage any         // 最近一次错误信息（仅失败时有值）
	LastTriggeredBy  any         // 最近一次触发方式：auto/manual
	TotalRuns        any         // 累计执行次数
	TotalFailures    any         // 累计失败次数
	CreatedAt        *gtime.Time //
	UpdatedAt        *gtime.Time //
}
