package v1

import "github.com/gogf/gf/v2/frame/g"

// PublicSettingsGetReq 获取公开设置项
type PublicSettingsGetReq struct {
	g.Meta `path:"/public" method:"get" mime:"json" tags:"公开-系统设置" summary:"获取公开设置项" middleware:"-"`
}

// PublicSettingsGetRes 返回所有 IsPublic=true 的设置项（key → typed value）
type PublicSettingsGetRes struct {
	Settings map[string]any `json:"settings"`
}

// PublicAnnouncementsReq 获取公开公告（无需认证，用于登录页等）
type PublicAnnouncementsReq struct {
	g.Meta   `path:"/announcements" method:"get" mime:"json" tags:"公开-公告" summary:"获取公开公告" middleware:"-"`
	Position string `json:"position" in:"query" dc:"展示位置过滤：login/console/both"`
}

type PublicAnnouncementsRes struct {
	List []PublicAnnouncementItem `json:"list"`
}

type PublicAnnouncementItem struct {
	Id              int64  `json:"id"`
	Title           string `json:"title"`
	Type            string `json:"type"`
	Content         string `json:"content"`
	IsPinned        int    `json:"is_pinned"`
	DisplayPosition string `json:"display_position"`
	CreatedAt       string `json:"created_at"`
}
