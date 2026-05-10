// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OpsAlertRulesDao is the data access object for the table ops_alert_rules.
type OpsAlertRulesDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  OpsAlertRulesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// OpsAlertRulesColumns defines and stores column names for the table ops_alert_rules.
type OpsAlertRulesColumns struct {
	Id                  string // 主键ID
	Name                string // 规则名称
	MetricType          string // 指标类型：api.error_rate / api.p95_latency / api.p99_latency / api.qps / system.cpu_percent / system.memory_percent / system.disk_percent / db.active_connections / redis.used_memory_mb
	Condition           string // 比较条件：gt / gte / lt / lte / eq
	Threshold           string // 阈值
	DurationSeconds     string // 持续时间（秒），0表示立即触发
	NotificationMethods string // 通知方式数组：email / webhook / in_app
	WebhookUrl          string // Webhook回调地址
	Level               string // 告警级别：info / warning / critical
	IsEnabled           string // 是否启用
	CooldownSeconds     string // 冷却时间（秒），同一规则两次告警最小间隔
	LastTriggeredAt     string // 上次触发时间
	NotifyUserIds       string // 通知接收人管理员ID列表
	CreatedAt           string // 创建时间
	UpdatedAt           string // 更新时间
}

// opsAlertRulesColumns holds the columns for the table ops_alert_rules.
var opsAlertRulesColumns = OpsAlertRulesColumns{
	Id:                  "id",
	Name:                "name",
	MetricType:          "metric_type",
	Condition:           "condition",
	Threshold:           "threshold",
	DurationSeconds:     "duration_seconds",
	NotificationMethods: "notification_methods",
	WebhookUrl:          "webhook_url",
	Level:               "level",
	IsEnabled:           "is_enabled",
	CooldownSeconds:     "cooldown_seconds",
	LastTriggeredAt:     "last_triggered_at",
	NotifyUserIds:       "notify_user_ids",
	CreatedAt:           "created_at",
	UpdatedAt:           "updated_at",
}

// NewOpsAlertRulesDao creates and returns a new DAO object for table data access.
func NewOpsAlertRulesDao(handlers ...gdb.ModelHandler) *OpsAlertRulesDao {
	return &OpsAlertRulesDao{
		group:    "default",
		table:    "ops_alert_rules",
		columns:  opsAlertRulesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OpsAlertRulesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OpsAlertRulesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OpsAlertRulesDao) Columns() OpsAlertRulesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OpsAlertRulesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OpsAlertRulesDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *OpsAlertRulesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
