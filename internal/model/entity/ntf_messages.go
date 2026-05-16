// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// NtfMessages is the golang structure for table ntf_messages.
type NtfMessages struct {
	Id          int64       `json:"id"           orm:"id"           description:"主键ID"`                                                                         // 主键ID
	TenantId    int64       `json:"tenant_id"    orm:"tenant_id"    description:"所属租户ID"`                                                                       // 所属租户ID
	UserId      int64       `json:"user_id"      orm:"user_id"      description:"接收用户ID（广播消息时为 NULL）"`                                                          // 接收用户ID（广播消息时为 NULL）
	Type        string      `json:"type"         orm:"type"         description:"消息类型：billing（计费）/ system（系统）/ security（安全）/ invitation（邀请）/ announcement（公告）"` // 消息类型：billing（计费）/ system（系统）/ security（安全）/ invitation（邀请）/ announcement（公告）
	Title       string      `json:"title"        orm:"title"        description:"消息标题"`                                                                         // 消息标题
	Content     string      `json:"content"      orm:"content"      description:"消息内容"`                                                                         // 消息内容
	Channel     string      `json:"channel"      orm:"channel"      description:"发送渠道：in_app（站内）/ email（邮件）/ both（双渠道）"`                                        // 发送渠道：in_app（站内）/ email（邮件）/ both（双渠道）
	IsRead      int         `json:"is_read"      orm:"is_read"      description:"是否已读：0=未读, 1=已读"`                                                              // 是否已读：0=未读, 1=已读
	IsBroadcast int         `json:"is_broadcast" orm:"is_broadcast" description:"是否广播消息：0=个人消息, 1=广播消息"`                                                        // 是否广播消息：0=个人消息, 1=广播消息
	Metadata    string      `json:"metadata"     orm:"metadata"     description:"附加元数据（JSONB，如关联资源ID、跳转链接等）"`                                                   // 附加元数据（JSONB，如关联资源ID、跳转链接等）
	TargetRoles string      `json:"target_roles" orm:"target_roles" description:"目标角色（NULL=全部角色，逗号分隔如 owner,admin 表示仅限这些角色）"`                                   // 目标角色（NULL=全部角色，逗号分隔如 owner,admin 表示仅限这些角色）
	CreatedAt   *gtime.Time `json:"created_at"   orm:"created_at"   description:"创建时间"`                                                                         // 创建时间
}
