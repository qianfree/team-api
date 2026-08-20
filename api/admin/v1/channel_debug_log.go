package v1

import "github.com/gogf/gf/v2/frame/g"

// 渠道调试日志：调试开关开启的渠道，per-attempt 记录客户端↔系统↔上游四段完整报文。
// 端点全部为 GET/DELETE，路径在 /channels/ 下，自动命中 rbac 前缀规则
//（GET → channel:view，DELETE → channel:delete），无需新增权限点。

// ChannelDebugLogListReq 渠道调试日志列表请求（只返回元数据与四段体积，不返回 body）
type ChannelDebugLogListReq struct {
	g.Meta         `path:"/channels/{channel_id}/debug-logs" method:"get" mime:"json" tags:"管理后台-渠道" summary:"渠道调试日志列表"`
	ChannelID      int64  `json:"channel_id" in:"path" v:"required" dc:"渠道ID"`
	Page           int    `json:"page" in:"query" d:"1" v:"min:1" dc:"页码"`
	PageSize       int    `json:"page_size" in:"query" d:"20" v:"min:1|max:100" dc:"每页数量"`
	RequestID      string `json:"request_id" in:"query" dc:"请求ID精确筛选"`
	ModelName      string `json:"model_name" in:"query" dc:"模型名模糊筛选"`
	UpstreamStatus *int   `json:"upstream_status" in:"query" dc:"上游状态码筛选"`
	ClientStatus   *int   `json:"client_status" in:"query" dc:"客户端状态码筛选"`
	IsStream       *bool  `json:"is_stream" in:"query" dc:"是否流式"`
	OnlyError      *bool  `json:"only_error" in:"query" dc:"仅看有错误的记录"`
	StartDate      string `json:"start_date" in:"query" dc:"开始日期 YYYY-MM-DD"`
	EndDate        string `json:"end_date" in:"query" dc:"结束日期 YYYY-MM-DD"`
}

// ChannelDebugLogListRes 渠道调试日志列表响应
type ChannelDebugLogListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// ChannelDebugLogStatsReq 渠道调试日志统计请求
type ChannelDebugLogStatsReq struct {
	g.Meta    `path:"/channels/{channel_id}/debug-logs/stats" method:"get" mime:"json" tags:"管理后台-渠道" summary:"渠道调试日志统计"`
	ChannelID int64 `json:"channel_id" in:"path" v:"required" dc:"渠道ID"`
}

// ChannelDebugLogStatsRes 渠道调试日志统计响应
type ChannelDebugLogStatsRes struct {
	Total      int64  `json:"total" dc:"记录条数"`
	TotalBytes int64  `json:"total_bytes" dc:"四段 body 落库总字节数"`
	OldestAt   string `json:"oldest_at" dc:"最早记录时间"`
}

// ChannelDebugLogDetailReq 渠道调试日志详情请求（返回四段完整报文）
type ChannelDebugLogDetailReq struct {
	g.Meta    `path:"/channels/{channel_id}/debug-logs/{id}" method:"get" mime:"json" tags:"管理后台-渠道" summary:"渠道调试日志详情"`
	ChannelID int64 `json:"channel_id" in:"path" v:"required" dc:"渠道ID"`
	ID        int64 `json:"id" in:"path" v:"required" dc:"记录ID"`
}

// ChannelDebugLogDetailRes 渠道调试日志详情响应
type ChannelDebugLogDetailRes struct {
	Data map[string]any `json:"data"`
}

// ChannelDebugLogDeleteReq 删除单条调试日志请求（硬删除）
type ChannelDebugLogDeleteReq struct {
	g.Meta    `path:"/channels/{channel_id}/debug-logs/{id}" method:"delete" mime:"json" tags:"管理后台-渠道" summary:"删除调试日志"`
	ChannelID int64 `json:"channel_id" in:"path" v:"required" dc:"渠道ID"`
	ID        int64 `json:"id" in:"path" v:"required" dc:"记录ID"`
}

// ChannelDebugLogDeleteRes 删除调试日志响应
type ChannelDebugLogDeleteRes struct{}

// ChannelDebugLogClearReq 清空渠道调试日志请求（按渠道硬删除全部）
type ChannelDebugLogClearReq struct {
	g.Meta    `path:"/channels/{channel_id}/debug-logs" method:"delete" mime:"json" tags:"管理后台-渠道" summary:"清空渠道调试日志"`
	ChannelID int64 `json:"channel_id" in:"path" v:"required" dc:"渠道ID"`
}

// ChannelDebugLogClearRes 清空渠道调试日志响应
type ChannelDebugLogClearRes struct{}
