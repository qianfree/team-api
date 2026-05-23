package v1

import "github.com/gogf/gf/v2/frame/g"

// === 渠道错误监控 ===

type ChannelErrorEventListReq struct {
	g.Meta        `path:"/monitor/channel-errors" method:"get" mime:"json" tags:"管理后台-监控" summary:"渠道错误事件列表"`
	Page          int    `json:"page" in:"query" d:"1"`
	PageSize      int    `json:"page_size" in:"query" d:"20"`
	ChannelID     int64  `json:"channel_id" in:"query"`
	ErrorCategory string `json:"error_category" in:"query"`
	StatusCode    int    `json:"status_code" in:"query"`
	StartDate     string `json:"start_date" in:"query"`
	EndDate       string `json:"end_date" in:"query"`
	Keyword       string `json:"keyword" in:"query"`
}

type ChannelErrorEventListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type ChannelErrorStatsReq struct {
	g.Meta    `path:"/monitor/channel-errors/stats" method:"get" mime:"json" tags:"管理后台-监控" summary:"渠道错误统计"`
	Hours     int   `json:"hours" in:"query" d:"24"`
	ChannelID int64 `json:"channel_id" in:"query"`
}

type ChannelErrorStatsRes struct {
	Total        int              `json:"total"`
	ByCategory   []map[string]any `json:"by_category"`
	ByStatusCode []map[string]any `json:"by_status_code"`
	TopChannels  []map[string]any `json:"top_channels"`
}

type ChannelErrorTrendReq struct {
	g.Meta    `path:"/monitor/channel-errors/trend" method:"get" mime:"json" tags:"管理后台-监控" summary:"渠道错误趋势"`
	Hours     int    `json:"hours" in:"query" d:"24"`
	ChannelID int64  `json:"channel_id" in:"query"`
	Category  string `json:"category" in:"query"`
}

type ChannelErrorTrendRes struct {
	Points []map[string]any `json:"points"`
}

type ChannelErrorTopChannelsReq struct {
	g.Meta `path:"/monitor/channel-errors/top-channels" method:"get" mime:"json" tags:"管理后台-监控" summary:"错误最多的渠道"`
	Hours  int `json:"hours" in:"query" d:"24"`
	Limit  int `json:"limit" in:"query" d:"10"`
}

type ChannelErrorTopChannelsRes struct {
	List []map[string]any `json:"list"`
}

type ChannelErrorCategoriesReq struct {
	g.Meta `path:"/monitor/channel-errors/categories" method:"get" mime:"json" tags:"管理后台-监控" summary:"错误分类选项"`
}

type ChannelErrorCategoriesRes struct {
	Data []map[string]string `json:"data"`
}
