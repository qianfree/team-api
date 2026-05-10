// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// OpsAlertRules is the golang structure for table ops_alert_rules.
type OpsAlertRules struct {
	Id                  int64       `json:"id"                   orm:"id"                   description:"主键ID"`                                                                                                                                                                                // 主键ID
	Name                string      `json:"name"                 orm:"name"                 description:"规则名称"`                                                                                                                                                                                // 规则名称
	MetricType          string      `json:"metric_type"          orm:"metric_type"          description:"指标类型：api.error_rate / api.p95_latency / api.p99_latency / api.qps / system.cpu_percent / system.memory_percent / system.disk_percent / db.active_connections / redis.used_memory_mb"` // 指标类型：api.error_rate / api.p95_latency / api.p99_latency / api.qps / system.cpu_percent / system.memory_percent / system.disk_percent / db.active_connections / redis.used_memory_mb
	Condition           string      `json:"condition"            orm:"condition"            description:"比较条件：gt / gte / lt / lte / eq"`                                                                                                                                                       // 比较条件：gt / gte / lt / lte / eq
	Threshold           float64     `json:"threshold"            orm:"threshold"            description:"阈值"`                                                                                                                                                                                  // 阈值
	DurationSeconds     int         `json:"duration_seconds"     orm:"duration_seconds"     description:"持续时间（秒），0表示立即触发"`                                                                                                                                                                     // 持续时间（秒），0表示立即触发
	NotificationMethods []string    `json:"notification_methods" orm:"notification_methods" description:"通知方式数组：email / webhook / in_app"`                                                                                                                                                     // 通知方式数组：email / webhook / in_app
	WebhookUrl          string      `json:"webhook_url"          orm:"webhook_url"          description:"Webhook回调地址"`                                                                                                                                                                         // Webhook回调地址
	Level               string      `json:"level"                orm:"level"                description:"告警级别：info / warning / critical"`                                                                                                                                                      // 告警级别：info / warning / critical
	IsEnabled           bool        `json:"is_enabled"           orm:"is_enabled"           description:"是否启用"`                                                                                                                                                                                // 是否启用
	CooldownSeconds     int         `json:"cooldown_seconds"     orm:"cooldown_seconds"     description:"冷却时间（秒），同一规则两次告警最小间隔"`                                                                                                                                                                // 冷却时间（秒），同一规则两次告警最小间隔
	LastTriggeredAt     *gtime.Time `json:"last_triggered_at"    orm:"last_triggered_at"    description:"上次触发时间"`                                                                                                                                                                              // 上次触发时间
	NotifyUserIds       []int64     `json:"notify_user_ids"      orm:"notify_user_ids"      description:"通知接收人管理员ID列表"`                                                                                                                                                                        // 通知接收人管理员ID列表
	CreatedAt           *gtime.Time `json:"created_at"           orm:"created_at"           description:"创建时间"`                                                                                                                                                                                // 创建时间
	UpdatedAt           *gtime.Time `json:"updated_at"           orm:"updated_at"           description:"更新时间"`                                                                                                                                                                                // 更新时间
}
