package v1

import "github.com/gogf/gf/v2/frame/g"

// === 租户端异步任务查询 ===

type TenantTaskListReq struct {
	g.Meta       `path:"/tasks" method:"get" mime:"json" tags:"租户控制台-任务管理" summary:"租户异步任务列表"`
	Page         int    `json:"page" in:"query" d:"1" v:"min:1" dc:"页码"`
	PageSize     int    `json:"page_size" in:"query" d:"20" v:"min:1|max:100" dc:"每页数量"`
	Status       string `json:"status" in:"query" dc:"筛选状态"`
	Platform     string `json:"platform" in:"query" dc:"筛选平台(sora/kling/midjourney/suno)"`
	PublicTaskID string `json:"public_task_id" in:"query" dc:"任务ID（精确匹配）"`
}

type TenantTaskItem struct {
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
	Username        string  `json:"username,omitempty"`
	SubmitTime      string  `json:"submit_time,omitempty"`
	FinishTime      string  `json:"finish_time,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

type TenantTaskListRes struct {
	List     []TenantTaskItem `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type TenantTaskDetailReq struct {
	g.Meta `path:"/tasks/{id}" method:"get" mime:"json" tags:"租户控制台-任务管理" summary:"租户异步任务详情"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"任务ID"`
}

type TenantTaskDetailRes struct {
	Task TenantTaskItem `json:"task"`
}

// TenantTaskExportReq 导出异步任务日志请求
type TenantTaskExportReq struct {
	g.Meta       `path:"/tasks/export" method:"get" mime:"json" tags:"租户控制台-任务管理" summary:"导出异步任务日志"`
	Format       string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	Status       string `json:"status" in:"query" dc:"筛选状态"`
	Platform     string `json:"platform" in:"query" dc:"筛选平台(sora/kling/midjourney/suno)"`
	PublicTaskID string `json:"public_task_id" in:"query" dc:"任务ID（精确匹配）"`
}

type TenantTaskExportRes struct{}
