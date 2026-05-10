// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// TskTaskLogs is the golang structure for table tsk_task_logs.
type TskTaskLogs struct {
	Id        int64       `json:"id"         orm:"id"         description:"主键ID"`                               // 主键ID
	TaskId    int64       `json:"task_id"    orm:"task_id"    description:"关联任务ID"`                             // 关联任务ID
	Level     string      `json:"level"      orm:"level"      description:"日志级别：info（信息）/ warn（警告）/ error（错误）"` // 日志级别：info（信息）/ warn（警告）/ error（错误）
	Message   string      `json:"message"    orm:"message"    description:"日志内容"`                               // 日志内容
	CreatedAt *gtime.Time `json:"created_at" orm:"created_at" description:"创建时间"`                               // 创建时间
	UpdatedAt *gtime.Time `json:"updated_at" orm:"updated_at" description:"更新时间"`                               // 更新时间
}
