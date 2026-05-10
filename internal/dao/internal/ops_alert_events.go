// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OpsAlertEventsDao is the data access object for the table ops_alert_events.
type OpsAlertEventsDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  OpsAlertEventsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// OpsAlertEventsColumns defines and stores column names for the table ops_alert_events.
type OpsAlertEventsColumns struct {
	Id              string // 主键ID
	RuleId          string // 关联规则ID
	RuleName        string // 规则名称（冗余存储）
	MetricType      string // 指标类型
	Level           string // 告警级别：info / warning / critical
	Status          string // 状态：firing（触发中）/ acknowledged（已确认）/ resolved（已恢复）
	TriggerValue    string // 触发时的实际指标值
	ThresholdValue  string // 规则阈值
	TriggerMessage  string // 触发消息描述
	AcknowledgedBy  string // 确认人管理员ID
	AcknowledgedAt  string // 确认时间
	ResolveNotes    string // 处理备注
	ResolvedBy      string // 解决人管理员ID
	ResolvedAt      string // 解决时间
	NotifiedMethods string // 已发送的通知方式
	CreatedAt       string // 创建时间
	UpdatedAt       string // 更新时间
}

// opsAlertEventsColumns holds the columns for the table ops_alert_events.
var opsAlertEventsColumns = OpsAlertEventsColumns{
	Id:              "id",
	RuleId:          "rule_id",
	RuleName:        "rule_name",
	MetricType:      "metric_type",
	Level:           "level",
	Status:          "status",
	TriggerValue:    "trigger_value",
	ThresholdValue:  "threshold_value",
	TriggerMessage:  "trigger_message",
	AcknowledgedBy:  "acknowledged_by",
	AcknowledgedAt:  "acknowledged_at",
	ResolveNotes:    "resolve_notes",
	ResolvedBy:      "resolved_by",
	ResolvedAt:      "resolved_at",
	NotifiedMethods: "notified_methods",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewOpsAlertEventsDao creates and returns a new DAO object for table data access.
func NewOpsAlertEventsDao(handlers ...gdb.ModelHandler) *OpsAlertEventsDao {
	return &OpsAlertEventsDao{
		group:    "default",
		table:    "ops_alert_events",
		columns:  opsAlertEventsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OpsAlertEventsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OpsAlertEventsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OpsAlertEventsDao) Columns() OpsAlertEventsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OpsAlertEventsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OpsAlertEventsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OpsAlertEventsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
