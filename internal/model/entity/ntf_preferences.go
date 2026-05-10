// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// NtfPreferences is the golang structure for table ntf_preferences.
type NtfPreferences struct {
	Id          int64       `json:"id"          orm:"id"          description:"主键ID"`                                                                                                       // 主键ID
	TenantId    int64       `json:"tenant_id"   orm:"tenant_id"   description:"所属租户ID"`                                                                                                     // 所属租户ID
	UserId      int64       `json:"user_id"     orm:"user_id"     description:"用户ID（组织级偏好时为 NULL）"`                                                                                         // 用户ID（组织级偏好时为 NULL）
	Scope       string      `json:"scope"       orm:"scope"       description:"偏好范围：user（用户级）/ org（组织级）"`                                                                                   // 偏好范围：user（用户级）/ org（组织级）
	Preferences string      `json:"preferences" orm:"preferences" description:"偏好配置（JSONB，如 {\"billing\":{\"email\":true,\"in_app\":true},\"security\":{\"email\":true,\"in_app\":true}}）"` // 偏好配置（JSONB，如 {"billing":{"email":true,"in_app":true},"security":{"email":true,"in_app":true}}）
	CreatedAt   *gtime.Time `json:"created_at"  orm:"created_at"  description:"创建时间"`                                                                                                       // 创建时间
	UpdatedAt   *gtime.Time `json:"updated_at"  orm:"updated_at"  description:"更新时间"`                                                                                                       // 更新时间
}
