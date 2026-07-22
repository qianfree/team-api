package v1

import "github.com/gogf/gf/v2/frame/g"

// === 大模型异步任务管理 ===

type TaskListReq struct {
	g.Meta       `path:"/tasks" method:"get" mime:"json" tags:"管理后台-任务管理" summary:"大模型异步任务列表"`
	Page         int    `json:"page" in:"query" d:"1" v:"min:1" dc:"页码"`
	PageSize     int    `json:"page_size" in:"query" d:"20" v:"min:1|max:100" dc:"每页数量"`
	Status       string `json:"status" in:"query" dc:"筛选状态"`
	Platform     string `json:"platform" in:"query" dc:"筛选平台(sora/kling/midjourney/suno)"`
	PublicTaskID string `json:"public_task_id" in:"query" dc:"任务ID（精确匹配）"`
}

type ModelTaskItem struct {
	ID              int64   `json:"id"`
	PublicTaskID    string  `json:"public_task_id"`
	Platform        string  `json:"platform"`
	Action          string  `json:"action"`
	Status          string  `json:"status"`
	Progress        string  `json:"progress"`
	ModelName       string  `json:"model_name"`
	FailReason      string  `json:"fail_reason,omitempty"`
	PreDeductAmount float64 `json:"pre_deduct_amount"`
	ActualCost      float64 `json:"actual_cost"`
	BillingSettled  bool    `json:"billing_settled"`
	ResultURL       string  `json:"result_url,omitempty"`
	ResultThumbURL  string  `json:"result_thumb_url,omitempty"` // 结果图预览缩略图 URL（仅任务详情、re-host 图片任务返回）
	TenantID        int64   `json:"tenant_id"`
	UserID          int64   `json:"user_id"`
	SubmitTime      string  `json:"submit_time,omitempty"`
	FinishTime      string  `json:"finish_time,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

type TaskListRes struct {
	List     []ModelTaskItem `json:"list"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

type TaskDetailReq struct {
	g.Meta `path:"/tasks/{id}" method:"get" mime:"json" tags:"管理后台-任务管理" summary:"大模型异步任务详情"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"任务ID"`
}

type TaskDetailRes struct {
	Task ModelTaskItem `json:"task"`
}

type TaskCancelReq struct {
	g.Meta `path:"/tasks/{id}/cancel" method:"post" mime:"json" tags:"管理后台-任务管理" summary:"取消大模型异步任务"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"任务ID"`
}

type TaskCancelRes struct{}
