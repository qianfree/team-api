// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysCronJobs is the golang structure for table sys_cron_jobs.
type SysCronJobs struct {
	Id               int64       `json:"id"                 orm:"id"                 description:""`                          //
	JobName          string      `json:"job_name"           orm:"job_name"           description:"任务名称（代码中定义的唯一标识）"`          // 任务名称（代码中定义的唯一标识）
	Schedule         string      `json:"schedule"           orm:"schedule"           description:"cron 表达式"`                  // cron 表达式
	LastStatus       string      `json:"last_status"        orm:"last_status"        description:"最近一次执行状态：succeeded/failed"` // 最近一次执行状态：succeeded/failed
	LastStartedAt    *gtime.Time `json:"last_started_at"    orm:"last_started_at"    description:"最近一次开始执行时间"`                // 最近一次开始执行时间
	LastFinishedAt   *gtime.Time `json:"last_finished_at"   orm:"last_finished_at"   description:"最近一次执行完成时间"`                // 最近一次执行完成时间
	LastDurationMs   int         `json:"last_duration_ms"   orm:"last_duration_ms"   description:"最近一次执行耗时（毫秒）"`              // 最近一次执行耗时（毫秒）
	LastErrorMessage string      `json:"last_error_message" orm:"last_error_message" description:"最近一次错误信息（仅失败时有值）"`          // 最近一次错误信息（仅失败时有值）
	LastTriggeredBy  string      `json:"last_triggered_by"  orm:"last_triggered_by"  description:"最近一次触发方式：auto/manual"`      // 最近一次触发方式：auto/manual
	TotalRuns        int         `json:"total_runs"         orm:"total_runs"         description:"累计执行次数"`                    // 累计执行次数
	TotalFailures    int         `json:"total_failures"     orm:"total_failures"     description:"累计失败次数"`                    // 累计失败次数
	CreatedAt        *gtime.Time `json:"created_at"         orm:"created_at"         description:""`                          //
	UpdatedAt        *gtime.Time `json:"updated_at"         orm:"updated_at"         description:""`                          //
}
