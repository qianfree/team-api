// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// NtfAnnouncements is the golang structure of table ntf_announcements for DAO operations like Where/Data.
type NtfAnnouncements struct {
	g.Meta          `orm:"table:ntf_announcements, do:true"`
	Id              any         // 主键ID
	Title           any         // 公告标题
	Type            any         // 公告类型：info（通知）/ warning（警告）/ important（重要）
	Content         any         // 公告内容
	Status          any         // 状态：draft（草稿）/ published（已发布）/ archived（已归档）
	IsPinned        any         // 是否置顶：0=否, 1=是
	DisplayPosition any         // 展示位置：login（登录页）/ console（控制台）/ both（双位置）
	EffectiveAt     *gtime.Time // 生效时间（NULL=立即生效）
	ExpiresAt       *gtime.Time // 过期时间（NULL=永不过期）
	CreatedBy       any         // 创建者（管理员ID）
	CreatedAt       *gtime.Time // 创建时间
	UpdatedAt       *gtime.Time // 更新时间
}
