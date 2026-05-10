package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 用户反馈（租户端） ===

type FeedbackCreateReq struct {
	g.Meta      `path:"/feedbacks" method:"post" mime:"json" tags:"租户-反馈" summary:"提交反馈"`
	Category    string         `json:"category" v:"required|in:bug_report,feature_request,suggestion,complaint" dc:"反馈类型"`
	Title       string         `json:"title" v:"required|length:1,200" dc:"反馈标题"`
	Description string         `json:"description" v:"required|length:1,5000" dc:"反馈详细描述"`
	Metadata    map[string]any `json:"metadata,omitempty" dc:"元数据（环境信息、截图链接等）"`
}

type FeedbackCreateRes struct {
	Id int64 `json:"id"`
}

type FeedbackListReq struct {
	g.Meta   `path:"/feedbacks" method:"get" mime:"json" tags:"租户-反馈" summary:"我的反馈列表"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"page_size" in:"query" d:"20"`
	Status   string `json:"status" in:"query"`
	Category string `json:"category" in:"query"`
}

type FeedbackItem struct {
	Id          int64       `json:"id"`
	Category    string      `json:"category"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Status      string      `json:"status"`
	Priority    string      `json:"priority"`
	AdminReply  string      `json:"admin_reply,omitempty"`
	Resolution  string      `json:"resolution,omitempty"`
	CreatedAt   *gtime.Time `json:"created_at"`
	UpdatedAt   *gtime.Time `json:"updated_at"`
}

type FeedbackListRes struct {
	List     []*FeedbackItem `json:"list"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

type FeedbackGetReq struct {
	g.Meta `path:"/feedbacks/{id}" method:"get" mime:"json" tags:"租户-反馈" summary:"反馈详情"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type FeedbackGetRes struct {
	Id          int64          `json:"id"`
	Category    string         `json:"category"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Status      string         `json:"status"`
	Priority    string         `json:"priority"`
	AdminReply  string         `json:"admin_reply,omitempty"`
	Resolution  string         `json:"resolution,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   *gtime.Time    `json:"created_at"`
	UpdatedAt   *gtime.Time    `json:"updated_at"`
}
