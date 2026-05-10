package v1

import "github.com/gogf/gf/v2/frame/g"

// === 定时任务 ===

type CronJobListReq struct {
	g.Meta `path:"/cron-jobs" method:"get" mime:"json" tags:"管理后台-定时任务" summary:"定时任务列表"`
}

type CronJobItem struct {
	Name           string `json:"name"`
	Schedule       string `json:"schedule"`
	IsRunning      bool   `json:"is_running"`
	LastStatus     string `json:"last_status"`
	LastStartedAt  string `json:"last_started_at"`
	LastDurationMs int    `json:"last_duration_ms"`
	LastErrorMsg   string `json:"last_error_message"`
	TotalExecs     int    `json:"total_executions"`
	TotalFailures  int    `json:"total_failures"`
}

type CronJobListRes struct {
	List []CronJobItem `json:"list"`
}

type CronJobExecutionsReq struct {
	g.Meta   `path:"/cron-jobs/{name}/executions" method:"get" mime:"json" tags:"管理后台-定时任务" summary:"任务执行历史"`
	Name     string `json:"name" in:"path" v:"required"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"page_size" in:"query" d:"20"`
	Status   string `json:"status" in:"query" dc:"过滤状态：succeeded/failed"`
}

type CronJobExecutionsRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type CronJobTriggerReq struct {
	g.Meta `path:"/cron-jobs/{name}/trigger" method:"post" mime:"json" tags:"管理后台-定时任务" summary:"手动触发任务"`
	Name   string `json:"name" in:"path" v:"required"`
}

type CronJobTriggerRes struct {
	Message string `json:"message"`
}
