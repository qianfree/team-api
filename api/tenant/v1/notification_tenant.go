package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 租户通知 ===

type TenantNotificationsReq struct {
	g.Meta   `path:"/notifications" method:"get" mime:"json" tags:"租户控制台-通知" summary:"通知列表"`
	Page     int `json:"page" in:"query" d:"1"`
	PageSize int `json:"page_size" in:"query" d:"20"`
}

type TenantNotificationItem struct {
	Id          int64       `json:"id"`
	TenantId    int64       `json:"tenant_id"`
	UserId      int64       `json:"user_id"`
	Type        string      `json:"type"`
	Title       string      `json:"title"`
	Content     string      `json:"content"`
	Channel     string      `json:"channel"`
	IsRead      int         `json:"is_read"`
	IsBroadcast int         `json:"is_broadcast"`
	Metadata    string      `json:"metadata"`
	CreatedAt   *gtime.Time `json:"created_at"`
}

type TenantNotificationsRes struct {
	List     []*TenantNotificationItem `json:"list"`
	Total    int                       `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
}

type TenantUnreadCountReq struct {
	g.Meta `path:"/notifications/unread-count" method:"get" mime:"json" tags:"租户控制台-通知" summary:"未读数量"`
}

type TenantUnreadCountRes struct {
	UnreadCount int `json:"unread_count"`
}

type TenantMarkReadReq struct {
	g.Meta `path:"/notifications/{id}/read" method:"post" mime:"json" tags:"租户控制台-通知" summary:"标记已读"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantMarkReadRes struct{}

type TenantMarkAllReadReq struct {
	g.Meta `path:"/notifications/read-all" method:"post" mime:"json" tags:"租户控制台-通知" summary:"全部已读"`
}

type TenantMarkAllReadRes struct{}

type TenantNotificationDeleteReq struct {
	g.Meta `path:"/notifications/{id}" method:"delete" mime:"json" tags:"租户控制台-通知" summary:"删除消息"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantNotificationDeleteRes struct{}

type TenantNotificationPreferencesGetReq struct {
	g.Meta `path:"/notification-preferences" method:"get" mime:"json" tags:"租户控制台-通知" summary:"通知偏好"`
}

type TenantNotificationPreferencesGetRes struct {
	Data map[string]any `json:"data"`
}

type TenantNotificationPreferencesUpdateReq struct {
	g.Meta      `path:"/notification-preferences" method:"put" mime:"json" tags:"租户控制台-通知" summary:"更新通知偏好"`
	Scope       string         `json:"scope" v:"required|in:user,org"`
	Preferences map[string]any `json:"preferences" v:"required"`
}

type TenantNotificationPreferencesUpdateRes struct{}

type TenantAnnouncementsReq struct {
	g.Meta `path:"/announcements" method:"get" mime:"json" tags:"租户控制台-通知" summary:"公告列表"`
}

type TenantAnnouncementItem struct {
	Id              int64       `json:"id"`
	Title           string      `json:"title"`
	Type            string      `json:"type"`
	Content         string      `json:"content"`
	IsPinned        int         `json:"is_pinned"`
	DisplayPosition string      `json:"display_position"`
	EffectiveAt     *gtime.Time `json:"effective_at"`
	ExpiresAt       *gtime.Time `json:"expires_at"`
	CreatedAt       *gtime.Time `json:"created_at"`
}

type TenantAnnouncementsRes struct {
	List []*TenantAnnouncementItem `json:"list"`
}

// TenantNotificationsExportReq 导出通知列表请求
type TenantNotificationsExportReq struct {
	g.Meta `path:"/notifications/export" method:"get" mime:"json" tags:"租户控制台-通知" summary:"导出通知列表"`
	Format string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
}

type TenantNotificationsExportRes struct{}
