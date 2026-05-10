package v1

import "github.com/gogf/gf/v2/frame/g"

// === 用量日志清理 ===

type UsageLogCleanupCreateReq struct {
	g.Meta    `path:"/usage-logs/cleanup" method:"post" mime:"json" tags:"管理后台-日志清理" summary:"创建清理任务"`
	StartTime string `json:"start_time" v:"required|datetime" dc:"起始时间（RFC3339）"`
	EndTime   string `json:"end_time" v:"required|datetime" dc:"截止时间（RFC3339）"`
	TenantID  *int64 `json:"tenant_id,omitempty" dc:"指定租户（可选）"`
	ModelName string `json:"model_name,omitempty" dc:"指定模型（可选）"`
	Status    string `json:"status,omitempty" dc:"指定状态（可选）"`
	BatchSize int    `json:"batch_size,omitempty" d:"5000" v:"min:100|max:50000" dc:"每批删除行数"`
	DryRun    bool   `json:"dry_run,omitempty" dc:"预演模式（不实际删除）"`
}

type UsageLogCleanupCreateRes struct {
	TaskID int64 `json:"task_id"`
}

type UsageLogCleanupListReq struct {
	g.Meta   `path:"/usage-logs/cleanup/tasks" method:"get" mime:"json" tags:"管理后台-日志清理" summary:"清理任务列表"`
	Page     int `json:"page" in:"query" d:"1" v:"min:1" dc:"页码"`
	PageSize int `json:"page_size" in:"query" d:"20" v:"min:1|max:100" dc:"每页数量"`
}

type UsageLogCleanupTaskItem struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message,omitempty"`
	Result       string `json:"result,omitempty"`
	StartedAt    string `json:"started_at,omitempty"`
	FinishedAt   string `json:"finished_at,omitempty"`
	CreatedAt    string `json:"created_at"`
}

type UsageLogCleanupListRes struct {
	List     []UsageLogCleanupTaskItem `json:"list"`
	Total    int                       `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
}

type UsageLogCleanupCancelReq struct {
	g.Meta `path:"/usage-logs/cleanup/tasks/{id}/cancel" method:"post" mime:"json" tags:"管理后台-日志清理" summary:"取消清理任务"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"任务ID"`
}

type UsageLogCleanupCancelRes struct{}
