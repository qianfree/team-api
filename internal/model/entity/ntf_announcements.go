// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// NtfAnnouncements is the golang structure for table ntf_announcements.
type NtfAnnouncements struct {
	Id              int64       `json:"id"               orm:"id"               description:"主键ID"`                                        // 主键ID
	Title           string      `json:"title"            orm:"title"            description:"公告标题"`                                        // 公告标题
	Type            string      `json:"type"             orm:"type"             description:"公告类型：info（通知）/ warning（警告）/ important（重要）"`   // 公告类型：info（通知）/ warning（警告）/ important（重要）
	Content         string      `json:"content"          orm:"content"          description:"公告内容"`                                        // 公告内容
	Status          string      `json:"status"           orm:"status"           description:"状态：draft（草稿）/ published（已发布）/ archived（已归档）"` // 状态：draft（草稿）/ published（已发布）/ archived（已归档）
	IsPinned        int         `json:"is_pinned"        orm:"is_pinned"        description:"是否置顶：0=否, 1=是"`                               // 是否置顶：0=否, 1=是
	DisplayPosition string      `json:"display_position" orm:"display_position" description:"展示位置：login（登录页）/ console（控制台）/ both（双位置）"`    // 展示位置：login（登录页）/ console（控制台）/ both（双位置）
	EffectiveAt     *gtime.Time `json:"effective_at"     orm:"effective_at"     description:"生效时间（NULL=立即生效）"`                             // 生效时间（NULL=立即生效）
	ExpiresAt       *gtime.Time `json:"expires_at"       orm:"expires_at"       description:"过期时间（NULL=永不过期）"`                             // 过期时间（NULL=永不过期）
	CreatedBy       int64       `json:"created_by"       orm:"created_by"       description:"创建者（管理员ID）"`                                  // 创建者（管理员ID）
	CreatedAt       *gtime.Time `json:"created_at"       orm:"created_at"       description:"创建时间"`                                        // 创建时间
	UpdatedAt       *gtime.Time `json:"updated_at"       orm:"updated_at"       description:"更新时间"`                                        // 更新时间
}
