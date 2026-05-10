package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 用户反馈（管理后台） ===

type FeedbackListAllReq struct {
	g.Meta   `path:"/feedbacks" method:"get" mime:"json" tags:"管理后台-反馈" summary:"反馈列表"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"page_size" in:"query" d:"20"`
	Status   string `json:"status" in:"query"`
	Category string `json:"category" in:"query"`
	TenantID int64  `json:"tenant_id" in:"query"`
	Priority string `json:"priority" in:"query"`
}

type FeedbackAdminItem struct {
	Id              int64       `json:"id"`
	TenantId        int64       `json:"tenant_id"`
	TenantName      string      `json:"tenant_name"`
	UserId          int64       `json:"user_id"`
	UserDisplayName string      `json:"user_display_name"`
	Category        string      `json:"category"`
	Title           string      `json:"title"`
	Description     string      `json:"description"`
	Status          string      `json:"status"`
	Priority        string      `json:"priority"`
	AdminReply      string      `json:"admin_reply,omitempty"`
	Resolution      string      `json:"resolution,omitempty"`
	CreatedAt       *gtime.Time `json:"created_at"`
	UpdatedAt       *gtime.Time `json:"updated_at"`
}

type FeedbackListAllRes struct {
	List     []*FeedbackAdminItem `json:"list"`
	Total    int                  `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

type FeedbackReplyReq struct {
	g.Meta     `path:"/feedbacks/{id}/reply" method:"post" mime:"json" tags:"管理后台-反馈" summary:"回复反馈"`
	Id         int64  `json:"id" in:"path" v:"required|min:1"`
	Reply      string `json:"reply" v:"required|length:1,5000" dc:"回复内容"`
	Status     string `json:"status" v:"in:acknowledged,in_progress,resolved,closed" dc:"更新状态"`
	Resolution string `json:"resolution" dc:"解决方案摘要"`
}

type FeedbackReplyRes struct{}

type FeedbackUpdateStatusReq struct {
	g.Meta   `path:"/feedbacks/{id}/status" method:"put" mime:"json" tags:"管理后台-反馈" summary:"更新反馈状态"`
	Id       int64  `json:"id" in:"path" v:"required|min:1"`
	Status   string `json:"status" v:"required|in:pending,acknowledged,in_progress,resolved,closed" dc:"状态"`
	Priority string `json:"priority" v:"in:low,normal,high,critical" dc:"优先级"`
}

type FeedbackUpdateStatusRes struct{}

type FeedbackStatsReq struct {
	g.Meta `path:"/feedbacks/stats" method:"get" mime:"json" tags:"管理后台-反馈" summary:"反馈统计"`
}

type FeedbackStatsRes struct {
	Total        int            `json:"total"`
	Pending      int            `json:"pending"`
	Acknowledged int            `json:"acknowledged"`
	InProgress   int            `json:"in_progress"`
	Resolved     int            `json:"resolved"`
	Closed       int            `json:"closed"`
	ByCategory   map[string]int `json:"by_category"`
	RecentTrend  []TrendItem    `json:"recent_trend"`
}

type TrendItem struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}
