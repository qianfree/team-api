// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// TskTasks is the golang structure of table tsk_tasks for DAO operations like Where/Data.
type TskTasks struct {
	g.Meta       `orm:"table:tsk_tasks, do:true"`
	Id           any         // 主键ID
	Name         any         // 任务名称（如 "发送邮件"、"日对账"、"数据导出"）
	Handler      any         // Handler 函数路径（用于任务路由）
	Status       any         // 状态：pending（待执行）/ running（执行中）/ succeeded（成功）/ failed（失败）/ cancelled（已取消）
	Payload      any         // 任务输入参数（JSONB）
	Result       any         // 任务执行结果（JSONB）
	MaxRetries   any         // 最大重试次数
	RetryCount   any         // 已重试次数
	StartedAt    *gtime.Time // 开始执行时间
	FinishedAt   *gtime.Time // 执行完成时间
	ScheduledAt  *gtime.Time // 计划执行时间（用于定时任务）
	ErrorMessage any         // 失败时的错误信息
	CreatedAt    *gtime.Time // 创建时间
	UpdatedAt    *gtime.Time // 更新时间
}
