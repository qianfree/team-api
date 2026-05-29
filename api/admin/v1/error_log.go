package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

// === 系统错误日志 ===

type ErrorLogListReq struct {
	g.Meta    `path:"/error-logs" method:"get" mime:"json" tags:"管理后台-系统错误" summary:"错误日志列表"`
	Page      int    `json:"page" d:"1"`
	PageSize  int    `json:"page_size" d:"20"`
	Source    string `json:"source" in:"query" dc:"错误来源：api/panic/cron/background"`
	ErrorCode int    `json:"error_code" in:"query" dc:"错误码"`
	Resolved  string `json:"resolved" in:"query" dc:"处理状态：true/false"`
	StartDate string `json:"start_date" in:"query" dc:"开始日期"`
	EndDate   string `json:"end_date" in:"query" dc:"结束日期"`
	Keyword   string `json:"keyword" in:"query" dc:"关键词搜索（错误消息/请求路径）"`
}

type ErrorLogListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type ErrorLogDetailReq struct {
	g.Meta `path:"/error-logs/{id}" method:"get" mime:"json" tags:"管理后台-系统错误" summary:"错误日志详情"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type ErrorLogDetailRes struct {
	Data map[string]any `json:"-"`
}

func (r *ErrorLogDetailRes) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Data)
}

type ErrorLogResolveReq struct {
	g.Meta `path:"/error-logs/{id}/resolve" method:"put" mime:"json" tags:"管理后台-系统错误" summary:"标记错误已处理"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type ErrorLogResolveRes struct{}

type ErrorLogBatchResolveReq struct {
	g.Meta `path:"/error-logs/batch-resolve" method:"put" mime:"json" tags:"管理后台-系统错误" summary:"批量标记已处理"`
	Ids    []int64 `json:"ids" v:"required"`
}

type ErrorLogBatchResolveRes struct{}

type ErrorLogStatsReq struct {
	g.Meta `path:"/error-logs/stats" method:"get" mime:"json" tags:"管理后台-系统错误" summary:"错误统计"`
}

type ErrorLogStatsRes struct {
	Data map[string]any `json:"data"`
}
