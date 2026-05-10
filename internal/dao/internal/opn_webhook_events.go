// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OpnWebhookEventsDao is the data access object for the table opn_webhook_events.
type OpnWebhookEventsDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  OpnWebhookEventsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// OpnWebhookEventsColumns defines and stores column names for the table opn_webhook_events.
type OpnWebhookEventsColumns struct {
	Id              string // 主键ID
	TenantId        string // 所属租户ID
	WebhookConfigId string // 关联的 Webhook 配置ID
	EventId         string // 事件唯一标识
	EventType       string // 事件类型
	Payload         string // 事件载荷（JSON）
	Status          string // 状态：pending / delivered / failed
	Attempts        string // 已尝试次数
	NextRetryAt     string // 下次重试时间
	CreatedAt       string // 创建时间
	UpdatedAt       string // 更新时间
}

// opnWebhookEventsColumns holds the columns for the table opn_webhook_events.
var opnWebhookEventsColumns = OpnWebhookEventsColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	WebhookConfigId: "webhook_config_id",
	EventId:         "event_id",
	EventType:       "event_type",
	Payload:         "payload",
	Status:          "status",
	Attempts:        "attempts",
	NextRetryAt:     "next_retry_at",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewOpnWebhookEventsDao creates and returns a new DAO object for table data access.
func NewOpnWebhookEventsDao(handlers ...gdb.ModelHandler) *OpnWebhookEventsDao {
	return &OpnWebhookEventsDao{
		group:    "default",
		table:    "opn_webhook_events",
		columns:  opnWebhookEventsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OpnWebhookEventsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OpnWebhookEventsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OpnWebhookEventsDao) Columns() OpnWebhookEventsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OpnWebhookEventsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OpnWebhookEventsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OpnWebhookEventsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
