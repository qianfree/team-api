package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

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

// TenantUsageLogDetailReq 用量日志详情：列表只返回表格展示字段，
// billing_summary / error_message / user_agent 等大字段经此接口按需获取。
// 租户隔离：member 角色仅能查看自己的记录。
type TenantUsageLogDetailReq struct {
	g.Meta `path:"/usage-logs/{id}" method:"get" mime:"json" tags:"租户控制台-用量" summary:"用量日志详情"`
	Id     int64 `json:"id" in:"path" v:"min:1" dc:"用量记录ID"`
}

// TenantUsageLogDetailRes 详情对象平铺进统一响应的 data 字段（json:"-" + 自定义
// MarshalJSON），避免统一包装后再多包一层 data（与 admin 端详情接口同一模式）
type TenantUsageLogDetailRes struct {
	Data map[string]any `json:"-"`
}

func (r *TenantUsageLogDetailRes) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Data)
}

// TenantUsageLogsSummaryReq 用量日志统计请求（复用列表筛选条件，不含分页）
type TenantUsageLogsSummaryReq struct {
	g.Meta      `path:"/usage-logs/summary" method:"get" mime:"json" tags:"租户控制台-用量" summary:"用量日志统计汇总"`
	Username    string `json:"username" in:"query" dc:"用户名（模糊匹配）"`
	Model       string `json:"model" in:"query"`
	Status      string `json:"status" in:"query"`
	RequestType int    `json:"request_type" in:"query"`
	StartDate   string `json:"start_date" in:"query"`
	EndDate     string `json:"end_date" in:"query"`
}

// TenantUsageLogsSummaryRes 用量日志统计汇总
type TenantUsageLogsSummaryRes struct {
	TotalCost         float64 `json:"total_cost" dc:"总费用（本位币，与列表费用列同口径：actual_cost 优先，0/NULL 回退 total_cost）"`
	TotalOutputTokens int64   `json:"total_output_tokens" dc:"总输出 token 数"`
	CacheReadRatio    float64 `json:"cache_read_ratio" dc:"缓存读取占比(%)，= cache_read / input_tokens"`
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
