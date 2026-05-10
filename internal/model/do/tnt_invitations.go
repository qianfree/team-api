// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// TntInvitations is the golang structure of table tnt_invitations for DAO operations like Where/Data.
type TntInvitations struct {
	g.Meta       `orm:"table:tnt_invitations, do:true"`
	Id           any         // 主键ID
	TenantId     any         // 所属租户ID
	Code         any         // 邀请码（唯一标识）
	InvitedEmail any         // 被邀请人邮箱（可选，指定后仅该邮箱可使用）
	Role         any         // 邀请后分配的角色：owner / admin / member
	ExpiresAt    *gtime.Time // 过期时间：7天 / 30天 / 永久（NULL）
	UsedByUserId any         // 使用该邀请注册的用户ID（NULL表示未使用）
	UsedAt       *gtime.Time // 使用时间
	CreatedBy    any         // 创建者用户ID
	CreatedAt    *gtime.Time // 创建时间
	UpdatedAt    *gtime.Time // 更新时间
	MaxUses      any         // 最大使用次数，0表示不限
	UseCount     any         // 已使用次数
}
