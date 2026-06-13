// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAgreements is the golang structure of table sys_agreements for DAO operations like Where/Data.
type SysAgreements struct {
	g.Meta      `orm:"table:sys_agreements, do:true"`
	Id          any         //
	Code        any         // 协议标识码
	Version     any         // 版本号
	Title       any         // 协议标题
	Content     any         // 协议正文（Markdown）
	Summary     any         // 版本变更摘要
	Status      any         // 状态：draft / published / archived
	IsCurrent   any         // 是否为当前生效版本
	ForceAccept any         // 是否强制用户接受
	PublishedAt *gtime.Time // 发布时间
	CreatedBy   any         // 创建的管理员ID
	CreatedAt   *gtime.Time //
	UpdatedAt   *gtime.Time //
}
