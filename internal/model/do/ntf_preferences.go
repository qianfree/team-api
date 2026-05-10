// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// NtfPreferences is the golang structure of table ntf_preferences for DAO operations like Where/Data.
type NtfPreferences struct {
	g.Meta      `orm:"table:ntf_preferences, do:true"`
	Id          any         // 主键ID
	TenantId    any         // 所属租户ID
	UserId      any         // 用户ID（组织级偏好时为 NULL）
	Scope       any         // 偏好范围：user（用户级）/ org（组织级）
	Preferences any         // 偏好配置（JSONB，如 {"billing":{"email":true,"in_app":true},"security":{"email":true,"in_app":true}}）
	CreatedAt   *gtime.Time // 创建时间
	UpdatedAt   *gtime.Time // 更新时间
}
