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
	Id          any         // 主键ID
	Code        any         // 协议标识码：terms(用户协议) / privacy(隐私政策)
	Version     any         // 版本号，如 1.0、2.0
	Title       any         // 协议标题
	Content     any         // 协议正文（Markdown）
	Summary     any         // 版本变更摘要
	Status      any         // 状态：draft(草稿) / published(已发布) / archived(已归档)
	IsCurrent   any         // 是否为该标识码的当前生效版本（每个code仅一条）
	ForceAccept any         // 是否强制用户接受（true=登录后必须接受才能继续）
	PublishedAt *gtime.Time // 发布时间
	CreatedBy   any         // 创建的管理员ID
	CreatedAt   *gtime.Time // 创建时间
	UpdatedAt   *gtime.Time // 更新时间
}
