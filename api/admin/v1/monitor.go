package v1

import "github.com/gogf/gf/v2/frame/g"

// === 监控仪表盘 ===

type MonitorDashboardReq struct {
	g.Meta  `path:"/monitor/dashboard" method:"get" mime:"json" tags:"管理后台-监控" summary:"监控仪表盘"`
	Minutes int `json:"minutes" in:"query" d:"5"`
}

type MonitorDashboardRes struct {
	Data map[string]any `json:"data"`
}

type MonitorTrafficReq struct {
	g.Meta  `path:"/monitor/traffic" method:"get" mime:"json" tags:"管理后台-监控" summary:"流量曲线"`
	Minutes int `json:"minutes" in:"query" d:"30"`
}

type MonitorTrafficRes struct {
	Data any `json:"data"`
}

type MonitorTrafficFlowReq struct {
	g.Meta    `path:"/monitor/traffic-flow" method:"get" mime:"json" tags:"管理后台-监控" summary:"流量流向桑基图"`
	StartDate string `json:"start_date" in:"query"`      // YYYY-MM-DD，缺省=近30天起点
	EndDate   string `json:"end_date" in:"query"`        // YYYY-MM-DD，缺省=今天
	Metric    string `json:"metric" in:"query" d:"cost"` // cost|tokens|requests，桑基边权重指标
}

type MonitorTrafficFlowRes struct {
	Data any `json:"data"`
}

type MonitorModelPerformanceReq struct {
	g.Meta    `path:"/monitor/model-performance" method:"get" mime:"json" tags:"管理后台-监控" summary:"模型性能指标"`
	StartDate string `json:"start_date" in:"query"` // YYYY-MM-DD，缺省=近30天起点
	EndDate   string `json:"end_date" in:"query"`   // YYYY-MM-DD，缺省=今天
	// 可选渠道筛选：0=全部渠道（选定渠道后当天数据改从 bil_usage_logs 明细实时聚合；0 哨兵被占用，无法筛「无渠道」）
	ChannelId int64 `json:"channel_id" in:"query" d:"0"`
	// 可选模型筛选：空=全部模型（与 bil_usage_daily.model_name 同口径）
	ModelName string `json:"model_name" in:"query"`
}

type MonitorModelPerformanceRes struct {
	List []ModelPerformanceSummary `json:"list"`
}

// ModelPerformanceSummary 模型性能摘要
type ModelPerformanceSummary struct {
	ModelName            string  `json:"model_name"`
	RequestCount         int     `json:"request_count"`
	SuccessCount         int     `json:"success_count"`
	SuccessRate          float64 `json:"success_rate"`
	AvgLatencyMs         float64 `json:"avg_latency_ms"`
	AvgFirstTokenMs      float64 `json:"avg_first_token_ms"`
	TPS                  float64 `json:"tps"`
	InputTokens          int64   `json:"input_tokens"`
	OutputTokens         int64   `json:"output_tokens"`
	TotalTokens          int64   `json:"total_tokens"`
	TotalCost            float64 `json:"total_cost"`
	CacheCreationTokens  int64   `json:"cache_creation_tokens"`
	CacheReadTokens      int64   `json:"cache_read_tokens"`
	CacheHitRequestCount int     `json:"cache_hit_request_count"`
	CacheHitRequestRate  float64 `json:"cache_hit_request_rate"`
	CacheHitRate         float64 `json:"cache_hit_rate"`
	Grade                string  `json:"grade"`
}

type MonitorModelChannelsReq struct {
	g.Meta    `path:"/monitor/model-channels" method:"get" mime:"json" tags:"管理后台-监控" summary:"模型各渠道性能"`
	StartDate string `json:"start_date" in:"query"` // YYYY-MM-DD，缺省=近30天起点
	EndDate   string `json:"end_date" in:"query"`   // YYYY-MM-DD，缺省=今天
	ModelName string `json:"model_name" in:"query" v:"required" dc:"必填，待查询的模型名"`
}

type MonitorModelChannelsRes struct {
	List []ModelChannelPerformance `json:"list"`
}

// ModelChannelPerformance 模型在各渠道的性能数据
type ModelChannelPerformance struct {
	ChannelID            int64   `json:"channel_id"`
	ChannelName          string  `json:"channel_name"`
	RequestCount         int     `json:"request_count"`
	SuccessCount         int     `json:"success_count"`
	SuccessRate          float64 `json:"success_rate"`
	AvgLatencyMs         float64 `json:"avg_latency_ms"`
	AvgFirstTokenMs      float64 `json:"avg_first_token_ms"`
	TPS                  float64 `json:"tps"`
	CacheCreationTokens  int64   `json:"cache_creation_tokens"`
	CacheReadTokens      int64   `json:"cache_read_tokens"`
	CacheHitRequestCount int     `json:"cache_hit_request_count"`
	CacheHitRequestRate  float64 `json:"cache_hit_request_rate"`
	CacheHitRate         float64 `json:"cache_hit_rate"`
}

type MonitorLatencyReq struct {
	g.Meta  `path:"/monitor/latency" method:"get" mime:"json" tags:"管理后台-监控" summary:"延迟直方图"`
	Minutes int `json:"minutes" in:"query" d:"5"`
}

type MonitorLatencyRes struct {
	Data map[string]any `json:"data"`
}

type MonitorSystemReq struct {
	g.Meta  `path:"/monitor/system" method:"get" mime:"json" tags:"管理后台-监控" summary:"系统指标"`
	Minutes int `json:"minutes" in:"query" d:"60"`
}

type MonitorSystemRes struct {
	Data map[string]any `json:"data"`
}

// MonitorDispatchReq 渠道调度引擎指标请求
type MonitorDispatchReq struct {
	g.Meta  `path:"/monitor/dispatch" method:"get" mime:"json" tags:"管理后台-监控" summary:"调度引擎指标"`
	Minutes int `json:"minutes" in:"query" d:"60" v:"between:5,10080" dc:"统计窗口（分钟）"`
}

// MonitorDispatchRes 渠道调度引擎指标响应。
// Latest 为进程启动以来的累计快照；Window 为窗口内增量（窗口首尾快照做差，
// 计数器因重启回绕时按 0 处理）。窗口内不足两条快照时 Window 为空。
type MonitorDispatchRes struct {
	Available     bool                 `json:"available" dc:"是否有指标数据"`
	CollectedAt   string               `json:"collected_at,omitempty" dc:"最新快照时间"`
	WindowMinutes int                  `json:"window_minutes" dc:"实际统计窗口（分钟）"`
	Latest        *DispatchMetricsData `json:"latest,omitempty" dc:"累计快照"`
	Window        *DispatchMetricsData `json:"window,omitempty" dc:"窗口增量"`
}

// DispatchMetricsData 调度指标数据（累计或增量）
type DispatchMetricsData struct {
	Selections     map[string]int64 `json:"selections" dc:"选择次数按原因（bind/hrw/overflow/probe/cred_rotate）"`
	Retries        map[string]int64 `json:"retries" dc:"重试决策按 错误类别→决策"`
	OverflowByTier map[string]int64 `json:"overflow_by_tier" dc:"溢出次数按目标层级"`
	SessionSources map[string]int64 `json:"session_sources" dc:"会话键来源分布（hdr/anthropic/openai/ident）"`
	BreakerOpens   int64            `json:"breaker_opens" dc:"熔断打开次数"`
	NoCandidate    int64            `json:"no_candidate" dc:"无可用渠道次数"`
}

type MonitorDBPoolReq struct {
	g.Meta `path:"/monitor/db-pool" method:"get" mime:"json" tags:"管理后台-监控" summary:"数据库连接池"`
}

type MonitorDBPoolRes struct {
	ActiveConnections int `json:"active_connections"`
	IdleConnections   int `json:"idle_connections"`
	TotalConnections  int `json:"total_connections"`
	MaxConnections    int `json:"max_connections"`
	WaitingQueries    int `json:"waiting_queries"`
}

type MonitorRedisPoolReq struct {
	g.Meta `path:"/monitor/redis-pool" method:"get" mime:"json" tags:"管理后台-监控" summary:"Redis连接池"`
}

type MonitorRedisPoolRes struct {
	ConnectedClients  int     `json:"connected_clients"`
	UsedMemoryMB      float64 `json:"used_memory_mb"`
	MaxMemoryMB       float64 `json:"max_memory_mb"`
	UsedMemoryPercent float64 `json:"used_memory_percent"`
	TotalCommands     int64   `json:"total_commands"`
	InstantaneousOps  int64   `json:"instantaneous_ops"`
	KeyspaceHits      int64   `json:"keyspace_hits"`
	KeyspaceMisses    int64   `json:"keyspace_misses"`
	HitRate           float64 `json:"hit_rate"`
}

// === 实时监控 ===

type MonitorRealtimeReq struct {
	g.Meta `path:"/monitor/realtime" method:"get" mime:"json" tags:"管理后台-监控" summary:"实时监控数据"`
}

type MonitorRealtimeRes struct {
	Data any `json:"data"`
}

// === 告警规则 ===

type AlertRuleListReq struct {
	g.Meta     `path:"/alert/rules" method:"get" mime:"json" tags:"管理后台-告警" summary:"告警规则列表"`
	Page       int    `json:"page" in:"query" d:"1"`
	PageSize   int    `json:"page_size" in:"query" d:"20"`
	MetricType string `json:"metric_type" in:"query"`
	Level      string `json:"level" in:"query"`
	Enabled    string `json:"enabled" in:"query"`
}

type AlertRuleListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type AlertOptionsReq struct {
	g.Meta `path:"/alert/options" method:"get" mime:"json" tags:"管理后台-告警" summary:"告警选项"`
}

type AlertOptionsRes struct {
	Data map[string]any `json:"data"`
}

type AlertRuleCreateReq struct {
	g.Meta              `path:"/alert/rules" method:"post" mime:"json" tags:"管理后台-告警" summary:"创建告警规则"`
	Name                string   `json:"name" v:"required"`
	MetricType          string   `json:"metric_type" v:"required"`
	Condition           string   `json:"condition" v:"required"`
	Threshold           float64  `json:"threshold" v:"required"`
	DurationSeconds     int      `json:"duration_seconds"`
	NotificationMethods []string `json:"notification_methods"`
	WebhookURL          string   `json:"webhook_url"`
	Level               string   `json:"level"`
	CooldownSeconds     int      `json:"cooldown_seconds"`
	NotifyUserIDs       []int64  `json:"notify_user_ids"`
}

type AlertRuleCreateRes struct {
	ID int64 `json:"id"`
}

type AlertRuleUpdateReq struct {
	g.Meta              `path:"/alert/rules/{id}" method:"put" mime:"json" tags:"管理后台-告警" summary:"更新告警规则"`
	Id                  int64    `json:"id" in:"path" v:"required|min:1"`
	Name                string   `json:"name"`
	MetricType          string   `json:"metric_type"`
	Condition           string   `json:"condition"`
	Threshold           *float64 `json:"threshold"`
	DurationSeconds     *int     `json:"duration_seconds"`
	NotificationMethods []string `json:"notification_methods"`
	WebhookURL          string   `json:"webhook_url"`
	Level               string   `json:"level"`
	CooldownSeconds     *int     `json:"cooldown_seconds"`
	NotifyUserIDs       []int64  `json:"notify_user_ids"`
}

type AlertRuleUpdateRes struct{}

type AlertRuleDeleteReq struct {
	g.Meta `path:"/alert/rules/{id}" method:"delete" mime:"json" tags:"管理后台-告警" summary:"删除告警规则"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type AlertRuleDeleteRes struct{}

type AlertRuleToggleReq struct {
	g.Meta `path:"/alert/rules/{id}/toggle" method:"put" mime:"json" tags:"管理后台-告警" summary:"切换告警规则"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type AlertRuleToggleRes struct{}

type AlertTestReq struct {
	g.Meta `path:"/alert/rules/{id}/test" method:"post" mime:"json" tags:"管理后台-告警" summary:"测试告警"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type AlertTestRes struct {
	Message string `json:"message"`
}

// === 告警事件 ===

type AlertEventListReq struct {
	g.Meta   `path:"/alert/events" method:"get" mime:"json" tags:"管理后台-告警" summary:"告警事件列表"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"page_size" in:"query" d:"20"`
	Status   string `json:"status" in:"query"`
	Level    string `json:"level" in:"query"`
	RuleID   int64  `json:"rule_id" in:"query"`
}

type AlertEventListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type AlertEventAcknowledgeReq struct {
	g.Meta `path:"/alert/events/{id}/acknowledge" method:"put" mime:"json" tags:"管理后台-告警" summary:"确认告警"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type AlertEventAcknowledgeRes struct{}

type AlertEventResolveReq struct {
	g.Meta `path:"/alert/events/{id}/resolve" method:"put" mime:"json" tags:"管理后台-告警" summary:"解决告警"`
	Id     int64  `json:"id" in:"path" v:"required|min:1"`
	Notes  string `json:"notes"`
}

type AlertEventResolveRes struct{}

// AlertEventClearReq 清空全部告警记录（硬删除，同时清理 Redis 中的触发状态）
type AlertEventClearReq struct {
	g.Meta `path:"/alert/events/clear" method:"delete" mime:"json" tags:"管理后台-告警" summary:"清空告警记录（硬删除全部）"`
}

// AlertEventClearRes 清空告警记录响应
type AlertEventClearRes struct {
	Deleted int64 `json:"deleted" dc:"删除的记录数"`
}
