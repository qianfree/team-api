// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ClgChangelogs is the golang structure of table clg_changelogs for DAO operations like Where/Data.
type ClgChangelogs struct {
	g.Meta      `orm:"table:clg_changelogs, do:true"`
	Id          any         // 主键ID
	Version     any         // 版本号
	Title       any         // 标题
	Content     any         // Markdown 内容
	Type        any         // 类型：feature / fix / improvement / breaking
	Status      any         // 状态：draft / published
	PublishedAt *gtime.Time // 发布时间
	CreatedBy   any         // 创建的管理员 ID
	CreatedAt   *gtime.Time //
	UpdatedAt   *gtime.Time //
}
