// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysCronJobExecutions is the golang structure for table sys_cron_job_executions.
type SysCronJobExecutions struct {
	Id           int64       `json:"id"            orm:"id"            description:"主键ID"`                          // 主键ID
	JobName      string      `json:"job_name"      orm:"job_name"      description:"任务名称（代码中定义的唯一标识）"`              // 任务名称（代码中定义的唯一标识）
	Status       string      `json:"status"        orm:"status"        description:"执行状态：succeeded/failed"`         // 执行状态：succeeded/failed
	StartedAt    *gtime.Time `json:"started_at"    orm:"started_at"    description:"开始执行时间"`                        // 开始执行时间
	FinishedAt   *gtime.Time `json:"finished_at"   orm:"finished_at"   description:"执行完成时间"`                        // 执行完成时间
	DurationMs   int         `json:"duration_ms"   orm:"duration_ms"   description:"执行耗时（毫秒）"`                      // 执行耗时（毫秒）
	ErrorMessage string      `json:"error_message" orm:"error_message" description:"错误消息（仅失败时有值）"`                  // 错误消息（仅失败时有值）
	TriggeredBy  string      `json:"triggered_by"  orm:"triggered_by"  description:"触发方式：auto（自动调度）/ manual（手动触发）"` // 触发方式：auto（自动调度）/ manual（手动触发）
	CreatedAt    *gtime.Time `json:"created_at"    orm:"created_at"    description:"创建时间"`                          // 创建时间
	UpdatedAt    *gtime.Time `json:"updated_at"    orm:"updated_at"    description:"更新时间"`                          // 更新时间
}
