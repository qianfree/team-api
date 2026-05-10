// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OpsAlertRules is the golang structure of table ops_alert_rules for DAO operations like Where/Data.
type OpsAlertRules struct {
	g.Meta              `orm:"table:ops_alert_rules, do:true"`
	Id                  any         // 主键ID
	Name                any         // 规则名称
	MetricType          any         // 指标类型：api.error_rate / api.p95_latency / api.p99_latency / api.qps / system.cpu_percent / system.memory_percent / system.disk_percent / db.active_connections / redis.used_memory_mb
	Condition           any         // 比较条件：gt / gte / lt / lte / eq
	Threshold           any         // 阈值
	DurationSeconds     any         // 持续时间（秒），0表示立即触发
	NotificationMethods []string    // 通知方式数组：email / webhook / in_app
	WebhookUrl          any         // Webhook回调地址
	Level               any         // 告警级别：info / warning / critical
	IsEnabled           any         // 是否启用
	CooldownSeconds     any         // 冷却时间（秒），同一规则两次告警最小间隔
	LastTriggeredAt     *gtime.Time // 上次触发时间
	NotifyUserIds       []int64     // 通知接收人管理员ID列表
	CreatedAt           *gtime.Time // 创建时间
	UpdatedAt           *gtime.Time // 更新时间
}
