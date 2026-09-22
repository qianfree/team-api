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
	ModelName    string `json:"model_name" in:"query" dc:"筛选模型名（精确匹配）"`
	TenantID     int64  `json:"tenant_id" in:"query" dc:"筛选租户ID"`
	UserID       int64  `json:"user_id" in:"query" dc:"筛选用户ID"`
	StartDate    string `json:"start_date" in:"query" dc:"开始时间（YYYY-MM-DD 或 YYYY-MM-DD HH:mm:ss）"`
	EndDate      string `json:"end_date" in:"query" dc:"结束时间（YYYY-MM-DD 或 YYYY-MM-DD HH:mm:ss）"`
}

type ModelTaskItem struct {
	ID           int64  `json:"id"`
	PublicTaskID string `json:"public_task_id"`
	Platform     string `json:"platform"`
	Action       string `json:"action"`
	Status       string `json:"status"`
	Progress     string `json:"progress"`
	ModelName    string `json:"model_name"`
	// UpstreamModel 上游供应商实际使用的模型名（仅任务详情返回，排障时用于和请求模型对照）
	UpstreamModel   string  `json:"upstream_model,omitempty"`
	FailReason      string  `json:"fail_reason,omitempty"`
	PreDeductAmount float64 `json:"pre_deduct_amount"`
	ActualCost      float64 `json:"actual_cost"`
	BillingSettled  bool    `json:"billing_settled"`
	ResultURL       string  `json:"result_url,omitempty"`
	ResultThumbURL  string  `json:"result_thumb_url,omitempty"` // 结果图预览缩略图 URL（仅任务详情、re-host 图片任务返回）
	TenantID        int64   `json:"tenant_id"`
	UserID          int64   `json:"user_id"`
	SubmitTime      string  `json:"submit_time,omitempty"`
	// StartTime 上游开始执行任务的时间（仅任务详情返回；详情时间线用它区分「排队耗时」与「执行耗时」）
	StartTime  string `json:"start_time,omitempty"`
	FinishTime string `json:"finish_time,omitempty"`
	CreatedAt  string `json:"created_at"`
	// RequestID 任务提交时的原始请求 ID（req_xxxxx，仅任务详情返回），关联 aud_request_logs.request_id
	RequestID string `json:"request_id,omitempty"`
	// 上游排障标识（仅任务详情返回；从 private_data 提取，其余内部字段不外露）：
	// UpstreamTaskID 任务句柄（上游轮询查询用）；UpstreamRequestID 提交调用追踪 ID（部分上游返回）
	UpstreamTaskID    string `json:"upstream_task_id,omitempty"`
	UpstreamRequestID string `json:"upstream_request_id,omitempty"`
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
