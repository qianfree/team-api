package v1

import "github.com/gogf/gf/v2/frame/g"

// === 任务管理 ===

type TaskListReq struct {
	g.Meta   `path:"/tasks" method:"get" mime:"json" tags:"管理后台-任务管理" summary:"任务列表"`
	Page     int    `json:"page" in:"query" d:"1" v:"min:1" dc:"页码"`
	PageSize int    `json:"page_size" in:"query" d:"20" v:"min:1|max:100" dc:"每页数量"`
	Status   string `json:"status" in:"query" dc:"筛选状态"`
	Handler  string `json:"handler" in:"query" dc:"筛选handler"`
}

type TaskItem struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Handler      string `json:"handler"`
	Status       string `json:"status"`
	MaxRetries   int    `json:"max_retries"`
	RetryCount   int    `json:"retry_count"`
	ErrorMessage string `json:"error_message,omitempty"`
	StartedAt    string `json:"started_at,omitempty"`
	FinishedAt   string `json:"finished_at,omitempty"`
	ScheduledAt  string `json:"scheduled_at,omitempty"`
	CreatedAt    string `json:"created_at"`
}

type TaskListRes struct {
	List     []TaskItem `json:"list"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

type TaskDetailReq struct {
	g.Meta `path:"/tasks/{id}" method:"get" mime:"json" tags:"管理后台-任务管理" summary:"任务详情"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"任务ID"`
}

type TaskLogItem struct {
	Level     string `json:"level"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

type TaskDetailRes struct {
	Task       TaskItem      `json:"task"`
	RecentLogs []TaskLogItem `json:"recent_logs"`
}

type TaskCancelReq struct {
	g.Meta `path:"/tasks/{id}/cancel" method:"post" mime:"json" tags:"管理后台-任务管理" summary:"取消任务"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"任务ID"`
}

type TaskCancelRes struct{}

type TaskRetryReq struct {
	g.Meta `path:"/tasks/{id}/retry" method:"post" mime:"json" tags:"管理后台-任务管理" summary:"重试任务"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"任务ID"`
}

type TaskRetryRes struct {
	TaskID int64 `json:"task_id"`
}
