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

type MonitorDBPoolReq struct {
	g.Meta `path:"/monitor/db-pool" method:"get" mime:"json" tags:"管理后台-监控" summary:"数据库连接池"`
}

type MonitorDBPoolRes struct {
	Data map[string]any `json:"data"`
}

type MonitorRedisPoolReq struct {
	g.Meta `path:"/monitor/redis-pool" method:"get" mime:"json" tags:"管理后台-监控" summary:"Redis连接池"`
}

type MonitorRedisPoolRes struct {
	Data map[string]any `json:"data"`
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
