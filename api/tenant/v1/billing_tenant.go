package v1

import "github.com/gogf/gf/v2/frame/g"

// === 租户用量与账单 ===

type TenantUsageLogsReq struct {
	g.Meta      `path:"/usage-logs" method:"get" mime:"json" tags:"租户控制台-用量" summary:"用量日志"`
	Page        int    `json:"page" in:"query" d:"1"`
	PageSize    int    `json:"page_size" in:"query" d:"20"`
	Username    string `json:"username" in:"query" dc:"用户名（模糊匹配）"`
	Model       string `json:"model" in:"query"`
	Status      string `json:"status" in:"query"`
	RequestType int    `json:"request_type" in:"query"`
	StartDate   string `json:"start_date" in:"query"`
	EndDate     string `json:"end_date" in:"query"`
}

type TenantUsageLogsRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// TenantUsageLogsExportReq 导出用量日志请求
type TenantUsageLogsExportReq struct {
	g.Meta      `path:"/usage-logs/export" method:"get" mime:"json" tags:"租户控制台-用量" summary:"导出用量日志"`
	Format      string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	Username    string `json:"username" in:"query" dc:"用户名（模糊匹配）"`
	Model       string `json:"model" in:"query"`
	Status      string `json:"status" in:"query"`
	RequestType int    `json:"request_type" in:"query"`
	StartDate   string `json:"start_date" in:"query"`
	EndDate     string `json:"end_date" in:"query"`
}

type TenantUsageLogsExportRes struct{}
