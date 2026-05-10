// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// TskTaskLogs is the golang structure of table tsk_task_logs for DAO operations like Where/Data.
type TskTaskLogs struct {
	g.Meta    `orm:"table:tsk_task_logs, do:true"`
	Id        any         // 主键ID
	TaskId    any         // 关联任务ID
	Level     any         // 日志级别：info（信息）/ warn（警告）/ error（错误）
	Message   any         // 日志内容
	CreatedAt *gtime.Time // 创建时间
	UpdatedAt *gtime.Time // 更新时间
}
