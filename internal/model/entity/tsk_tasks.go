// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// TskTasks is the golang structure for table tsk_tasks.
type TskTasks struct {
	Id           int64       `json:"id"            orm:"id"            description:"主键ID"`                                                                     // 主键ID
	Name         string      `json:"name"          orm:"name"          description:"任务名称（如 \"发送邮件\"、\"日对账\"、\"数据导出\"）"`                                        // 任务名称（如 "发送邮件"、"日对账"、"数据导出"）
	Handler      string      `json:"handler"       orm:"handler"       description:"Handler 函数路径（用于任务路由）"`                                                     // Handler 函数路径（用于任务路由）
	Status       string      `json:"status"        orm:"status"        description:"状态：pending（待执行）/ running（执行中）/ succeeded（成功）/ failed（失败）/ cancelled（已取消）"` // 状态：pending（待执行）/ running（执行中）/ succeeded（成功）/ failed（失败）/ cancelled（已取消）
	Payload      string      `json:"payload"       orm:"payload"       description:"任务输入参数（JSONB）"`                                                            // 任务输入参数（JSONB）
	Result       string      `json:"result"        orm:"result"        description:"任务执行结果（JSONB）"`                                                            // 任务执行结果（JSONB）
	MaxRetries   int         `json:"max_retries"   orm:"max_retries"   description:"最大重试次数"`                                                                   // 最大重试次数
	RetryCount   int         `json:"retry_count"   orm:"retry_count"   description:"已重试次数"`                                                                    // 已重试次数
	StartedAt    *gtime.Time `json:"started_at"    orm:"started_at"    description:"开始执行时间"`                                                                   // 开始执行时间
	FinishedAt   *gtime.Time `json:"finished_at"   orm:"finished_at"   description:"执行完成时间"`                                                                   // 执行完成时间
	ScheduledAt  *gtime.Time `json:"scheduled_at"  orm:"scheduled_at"  description:"计划执行时间（用于定时任务）"`                                                           // 计划执行时间（用于定时任务）
	ErrorMessage string      `json:"error_message" orm:"error_message" description:"失败时的错误信息"`                                                                 // 失败时的错误信息
	CreatedAt    *gtime.Time `json:"created_at"    orm:"created_at"    description:"创建时间"`                                                                     // 创建时间
	UpdatedAt    *gtime.Time `json:"updated_at"    orm:"updated_at"    description:"更新时间"`                                                                     // 更新时间
}
