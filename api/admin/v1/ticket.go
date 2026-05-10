package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 工单管理（管理后台） ===

type TicketListReq struct {
	g.Meta          `path:"/tickets" method:"get" mime:"json" tags:"管理后台-工单" summary:"工单列表"`
	Page            int    `json:"page" in:"query" d:"1"`
	PageSize        int    `json:"page_size" in:"query" d:"20"`
	Status          string `json:"status" in:"query"`
	Category        string `json:"category" in:"query"`
	TenantID        int64  `json:"tenant_id" in:"query"`
	AssignedAdminID int64  `json:"assigned_admin_id" in:"query"`
}

type TicketItem struct {
	Id                int64       `json:"id"`
	TenantId          int64       `json:"tenant_id"`
	UserId            int64       `json:"user_id"`
	Category          string      `json:"category"`
	Title             string      `json:"title"`
	Description       string      `json:"description"`
	Urgency           string      `json:"urgency"`
	Status            string      `json:"status"`
	AssignedAdminId   int64       `json:"assigned_admin_id"`
	TenantName        string      `json:"tenant_name"`
	UserDisplayName   string      `json:"user_display_name"`
	AssignedAdminName string      `json:"assigned_admin_name"`
	CreatedAt         *gtime.Time `json:"created_at"`
	UpdatedAt         *gtime.Time `json:"updated_at"`
}

type TicketListRes struct {
	List     []*TicketItem `json:"list"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

type TicketGetReq struct {
	g.Meta `path:"/tickets/{id}" method:"get" mime:"json" tags:"管理后台-工单" summary:"工单详情"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TicketGetRes struct {
	Data map[string]any `json:"data"`
}

type TicketAssignReq struct {
	g.Meta  `path:"/tickets/{id}/assign" method:"put" mime:"json" tags:"管理后台-工单" summary:"分配工单"`
	Id      int64 `json:"id" in:"path" v:"required|min:1"`
	AdminID int64 `json:"admin_id" v:"required|min:1"`
}

type TicketAssignRes struct{}

type TicketReplyReq struct {
	g.Meta  `path:"/tickets/{id}/reply" method:"post" mime:"json" tags:"管理后台-工单" summary:"回复工单"`
	Id      int64  `json:"id" in:"path" v:"required|min:1"`
	Content string `json:"content" v:"required"`
}

type TicketReplyRes struct{}

type TicketStatusUpdateReq struct {
	g.Meta `path:"/tickets/{id}/status" method:"put" mime:"json" tags:"管理后台-工单" summary:"更新工单状态"`
	Id     int64  `json:"id" in:"path" v:"required|min:1"`
	Status string `json:"status" v:"required|in:pending,processing,replied,closed,reopened"`
}

type TicketStatusUpdateRes struct{}
