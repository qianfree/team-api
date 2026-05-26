// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// NtfMessages is the golang structure of table ntf_messages for DAO operations like Where/Data.
type NtfMessages struct {
	g.Meta      `orm:"table:ntf_messages, do:true"`
	Id          any         // 主键ID
	TenantId    any         // 所属租户ID
	UserId      any         // 接收用户ID（广播消息时为 NULL）
	Type        any         // 消息类型：billing（计费）/ system（系统）/ security（安全）/ invitation（邀请）/ announcement（公告）
	Title       any         // 消息标题
	Content     any         // 消息内容
	Channel     any         // 发送渠道：in_app（站内）/ email（邮件）/ both（双渠道）
	IsRead      any         // 是否已读：0=未读, 1=已读
	IsBroadcast any         // 是否广播消息：0=个人消息, 1=广播消息
	Metadata    any         // 附加元数据（JSONB，如关联资源ID、跳转链接等）
	TargetRoles any         // 目标角色（NULL=全部角色，逗号分隔如 owner,admin 表示仅限这些角色）
	CreatedAt   *gtime.Time // 创建时间
}
