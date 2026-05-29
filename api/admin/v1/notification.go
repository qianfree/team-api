package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 通知模板管理 ===

type TemplateListReq struct {
	g.Meta   `path:"/notification/templates" method:"get" mime:"json" tags:"管理后台-通知" summary:"模板列表"`
	Page     int `json:"page" in:"query" d:"1"`
	PageSize int `json:"page_size" in:"query" d:"20"`
}

type TemplateItem struct {
	Id           int64       `json:"id"`
	Code         string      `json:"code"`
	Channel      string      `json:"channel"`
	Subject      string      `json:"subject"`
	BodyTemplate string      `json:"body_template"`
	Variables    string      `json:"variables"`
	Status       string      `json:"status"`
	CreatedAt    *gtime.Time `json:"created_at"`
	UpdatedAt    *gtime.Time `json:"updated_at"`
}

type TemplateListRes struct {
	List     []*TemplateItem `json:"list"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

type TemplateGetReq struct {
	g.Meta `path:"/notification/templates/{code}" method:"get" mime:"json" tags:"管理后台-通知" summary:"模板详情"`
	Code   string `json:"code" in:"path" v:"required"`
}

type TemplateGetRes struct {
	Id           int64       `json:"id"`
	Code         string      `json:"code"`
	Channel      string      `json:"channel"`
	Subject      string      `json:"subject"`
	BodyTemplate string      `json:"body_template"`
	Variables    string      `json:"variables"`
	Status       string      `json:"status"`
	CreatedAt    *gtime.Time `json:"created_at"`
	UpdatedAt    *gtime.Time `json:"updated_at"`
}

type TemplateUpdateReq struct {
	g.Meta       `path:"/notification/templates/{code}" method:"put" mime:"json" tags:"管理后台-通知" summary:"更新模板"`
	Code         string `json:"code" in:"path" v:"required"`
	Subject      string `json:"subject"`
	BodyTemplate string `json:"body_template"`
	Channel      string `json:"channel"`
}

type TemplateUpdateRes struct{}

type TemplateTestReq struct {
	g.Meta    `path:"/notification/templates/{code}/test" method:"post" mime:"json" tags:"管理后台-通知" summary:"测试模板"`
	Code      string         `json:"code" in:"path" v:"required"`
	Variables map[string]any `json:"variables"`
}

type TemplateRenderResult struct {
	Original string `json:"original"`
	Rendered string `json:"rendered,omitempty"`
	Error    string `json:"error,omitempty"`
}

type TemplateTestRes struct {
	Subject TemplateRenderResult `json:"subject"`
	Body    TemplateRenderResult `json:"body"`
	Channel string               `json:"channel"`
}

// === 站内消息管理 ===

type MessageListReq struct {
	g.Meta   `path:"/notification/messages" method:"get" mime:"json" tags:"管理后台-通知" summary:"消息列表"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"page_size" in:"query" d:"20"`
	TenantID int64  `json:"tenant_id" in:"query"`
	Type     string `json:"type" in:"query"`
}

type MessageItem struct {
	Id          int64       `json:"id"`
	TenantId    int64       `json:"tenant_id"`
	TenantName  string      `json:"tenant_name"`
	UserId      int64       `json:"user_id"`
	UserName    string      `json:"user_name"`
	Type        string      `json:"type"`
	Title       string      `json:"title"`
	Content     string      `json:"content"`
	Channel     string      `json:"channel"`
	IsRead      int         `json:"is_read"`
	IsBroadcast int         `json:"is_broadcast"`
	Metadata    string      `json:"metadata"`
	CreatedAt   *gtime.Time `json:"created_at"`
}

type MessageListRes struct {
	List     []*MessageItem `json:"list"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

type MessageSendReq struct {
	g.Meta   `path:"/notification/messages/send" method:"post" mime:"json" tags:"管理后台-通知" summary:"发送消息"`
	TenantID int64  `json:"tenant_id" v:"required|min:1"`
	UserID   int64  `json:"user_id" dc:"目标用户ID，留空则发送给租户所有人"`
	Title    string `json:"title" v:"required"`
	Content  string `json:"content" v:"required"`
	Channel  string `json:"channel"`
}

type MessageSendRes struct{}

type MessageBroadcastReq struct {
	g.Meta      `path:"/notification/messages/broadcast" method:"post" mime:"json" tags:"管理后台-通知" summary:"广播消息"`
	TenantID    *int64 `json:"tenant_id" dc:"目标租户ID，不传或为空表示广播到所有租户"`
	Title       string `json:"title" v:"required"`
	Content     string `json:"content" v:"required"`
	TargetRoles string `json:"target_roles" d:"" dc:"目标角色，逗号分隔如 owner,admin；留空表示全部角色"`
}

type MessageBroadcastRes struct{}

// === 公告管理 ===

type AnnouncementListReq struct {
	g.Meta   `path:"/announcements" method:"get" mime:"json" tags:"管理后台-公告" summary:"公告列表"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"page_size" in:"query" d:"20"`
	Status   string `json:"status" in:"query"`
}

type AnnouncementItem struct {
	Id              int64       `json:"id"`
	Title           string      `json:"title"`
	Type            string      `json:"type"`
	Content         string      `json:"content"`
	Status          string      `json:"status"`
	IsPinned        int         `json:"is_pinned"`
	DisplayPosition string      `json:"display_position"`
	EffectiveAt     *gtime.Time `json:"effective_at"`
	ExpiresAt       *gtime.Time `json:"expires_at"`
	CreatedBy       int64       `json:"created_by"`
	CreatedAt       *gtime.Time `json:"created_at"`
	UpdatedAt       *gtime.Time `json:"updated_at"`
}

type AnnouncementListRes struct {
	List     []*AnnouncementItem `json:"list"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

type AnnouncementCreateReq struct {
	g.Meta          `path:"/announcements" method:"post" mime:"json" tags:"管理后台-公告" summary:"创建公告"`
	Title           string `json:"title" v:"required"`
	Type            string `json:"type"`
	Content         string `json:"content" v:"required"`
	Status          string `json:"status"`
	IsPinned        int    `json:"is_pinned"`
	DisplayPosition string `json:"display_position"`
	EffectiveAt     string `json:"effective_at"`
	ExpiresAt       string `json:"expires_at"`
}

type AnnouncementCreateRes struct {
	ID int64 `json:"id"`
}

type AnnouncementUpdateReq struct {
	g.Meta          `path:"/announcements/{id}" method:"put" mime:"json" tags:"管理后台-公告" summary:"更新公告"`
	Id              int64  `json:"id" in:"path" v:"required|min:1"`
	Title           string `json:"title" dc:"标题"`
	Type            string `json:"type" dc:"类型：info/warning/important"`
	Content         string `json:"content" dc:"内容"`
	Status          string `json:"status" dc:"状态：draft/published/archived"`
	IsPinned        *int   `json:"is_pinned" dc:"是否置顶"`
	DisplayPosition string `json:"display_position" dc:"展示位置：login/console/both"`
	EffectiveAt     string `json:"effective_at" dc:"生效时间"`
	ExpiresAt       string `json:"expires_at" dc:"过期时间"`
}

type AnnouncementUpdateRes struct{}

type AnnouncementPublishReq struct {
	g.Meta `path:"/announcements/{id}/publish" method:"put" mime:"json" tags:"管理后台-公告" summary:"发布公告"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type AnnouncementPublishRes struct{}

type AnnouncementArchiveReq struct {
	g.Meta `path:"/announcements/{id}/archive" method:"put" mime:"json" tags:"管理后台-公告" summary:"归档公告"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type AnnouncementArchiveRes struct{}
