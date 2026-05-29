package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 租户工单 ===

type TenantTicketCreateReq struct {
	g.Meta      `path:"/tickets" method:"post" mime:"json" tags:"租户控制台-工单" summary:"创建工单"`
	Category    string `json:"category" v:"required|in:billing,technical,feature_request,other"`
	Title       string `json:"title" v:"required"`
	Description string `json:"description" v:"required"`
	Urgency     string `json:"urgency" v:"required|in:low,normal,high,urgent"`
}

type TenantTicketCreateRes struct {
	ID int64 `json:"id"`
}

type TenantTicketListReq struct {
	g.Meta   `path:"/tickets" method:"get" mime:"json" tags:"租户控制台-工单" summary:"工单列表"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"page_size" in:"query" d:"20"`
	Status   string `json:"status" in:"query"`
}

type TenantTicketItem struct {
	Id                int64       `json:"id"`
	TenantId          int64       `json:"tenant_id"`
	UserId            int64       `json:"user_id"`
	Category          string      `json:"category"`
	Title             string      `json:"title"`
	Description       string      `json:"description"`
	Urgency           string      `json:"urgency"`
	Status            string      `json:"status"`
	AssignedAdminId   int64       `json:"assigned_admin_id"`
	AssignedAdminName string      `json:"assigned_admin_name"`
	CreatedAt         *gtime.Time `json:"created_at"`
	UpdatedAt         *gtime.Time `json:"updated_at"`
}

type TenantTicketListRes struct {
	List     []*TenantTicketItem `json:"list"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

type TenantTicketGetReq struct {
	g.Meta `path:"/tickets/{id}" method:"get" mime:"json" tags:"租户控制台-工单" summary:"工单详情"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantTicketReplyItem struct {
	Id        int64       `json:"id"`
	TicketId  int64       `json:"ticket_id"`
	UserId    int64       `json:"user_id"`
	UserType  string      `json:"user_type"`
	Content   string      `json:"content"`
	CreatedAt *gtime.Time `json:"created_at"`
}

type TenantTicketAttachmentItem struct {
	Id          int64       `json:"id"`
	TicketId    int64       `json:"ticket_id"`
	ReplyId     int64       `json:"reply_id"`
	FileName    string      `json:"file_name"`
	FileUrl     string      `json:"file_url"`
	FileSize    int         `json:"file_size"`
	ContentType string      `json:"content_type"`
	CreatedAt   *gtime.Time `json:"created_at"`
}

type TenantTicketGetRes struct {
	Id                int64                         `json:"id"`
	TenantId          int64                         `json:"tenant_id"`
	UserId            int64                         `json:"user_id"`
	Category          string                        `json:"category"`
	Title             string                        `json:"title"`
	Description       string                        `json:"description"`
	Urgency           string                        `json:"urgency"`
	Status            string                        `json:"status"`
	AssignedAdminId   int64                         `json:"assigned_admin_id"`
	AssignedAdminName string                        `json:"assigned_admin_name"`
	CreatedAt         *gtime.Time                   `json:"created_at"`
	UpdatedAt         *gtime.Time                   `json:"updated_at"`
	Replies           []*TenantTicketReplyItem      `json:"replies"`
	Attachments       []*TenantTicketAttachmentItem `json:"attachments"`
}

type TenantTicketReplyReq struct {
	g.Meta  `path:"/tickets/{id}/reply" method:"post" mime:"json" tags:"租户控制台-工单" summary:"回复工单"`
	Id      int64  `json:"id" in:"path" v:"required|min:1"`
	Content string `json:"content" v:"required"`
}

type TenantTicketReplyRes struct{}

type TenantTicketCloseReq struct {
	g.Meta `path:"/tickets/{id}/close" method:"post" mime:"json" tags:"租户控制台-工单" summary:"关闭工单"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantTicketCloseRes struct{}

type TenantTicketReopenReq struct {
	g.Meta `path:"/tickets/{id}/reopen" method:"post" mime:"json" tags:"租户控制台-工单" summary:"重新打开工单"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantTicketReopenRes struct{}

// TenantTicketExportReq 导出工单列表请求
type TenantTicketExportReq struct {
	g.Meta `path:"/tickets/export" method:"get" mime:"json" tags:"租户控制台-工单" summary:"导出工单列表"`
	Format string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	Status string `json:"status" in:"query"`
}

type TenantTicketExportRes struct{}
