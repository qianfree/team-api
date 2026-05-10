// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// TntInvitations is the golang structure for table tnt_invitations.
type TntInvitations struct {
	Id           int64       `json:"id"              orm:"id"              description:"主键ID"`                            // 主键ID
	TenantId     int64       `json:"tenant_id"       orm:"tenant_id"       description:"所属租户ID"`                          // 所属租户ID
	Code         string      `json:"code"            orm:"code"            description:"邀请码（唯一标识）"`                       // 邀请码（唯一标识）
	InvitedEmail string      `json:"invited_email"   orm:"invited_email"   description:"被邀请人邮箱（可选，指定后仅该邮箱可使用）"`           // 被邀请人邮箱（可选，指定后仅该邮箱可使用）
	Role         string      `json:"role"            orm:"role"            description:"邀请后分配的角色：owner / admin / member"` // 邀请后分配的角色：owner / admin / member
	ExpiresAt    *gtime.Time `json:"expires_at"      orm:"expires_at"      description:"过期时间：7天 / 30天 / 永久（NULL）"`        // 过期时间：7天 / 30天 / 永久（NULL）
	UsedByUserId int64       `json:"used_by_user_id" orm:"used_by_user_id" description:"使用该邀请注册的用户ID（NULL表示未使用）"`         // 使用该邀请注册的用户ID（NULL表示未使用）
	UsedAt       *gtime.Time `json:"used_at"         orm:"used_at"         description:"使用时间"`                            // 使用时间
	CreatedBy    int64       `json:"created_by"      orm:"created_by"      description:"创建者用户ID"`                         // 创建者用户ID
	CreatedAt    *gtime.Time `json:"created_at"      orm:"created_at"      description:"创建时间"`                            // 创建时间
	UpdatedAt    *gtime.Time `json:"updated_at"      orm:"updated_at"      description:"更新时间"`                            // 更新时间
	MaxUses      int         `json:"max_uses"        orm:"max_uses"        description:"最大使用次数，0表示不限"`                    // 最大使用次数，0表示不限
	UseCount     int         `json:"use_count"       orm:"use_count"       description:"已使用次数"`                           // 已使用次数
}
