// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OpnWebhookDeliveryLogsDao is the data access object for the table opn_webhook_delivery_logs.
type OpnWebhookDeliveryLogsDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  OpnWebhookDeliveryLogsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// OpnWebhookDeliveryLogsColumns defines and stores column names for the table opn_webhook_delivery_logs.
type OpnWebhookDeliveryLogsColumns struct {
	Id              string // 主键ID
	TenantId        string // 所属租户ID
	WebhookConfigId string // Webhook 配置ID
	EventId         string // 关联的事件ID
	Attempt         string // 第几次尝试
	RequestUrl      string // 请求 URL
	RequestHeaders  string // 请求头（JSON）
	ResponseStatus  string // HTTP 响应状态码
	ResponseBody    string // 响应体（截断到 2000 字符）
	ResponseTimeMs  string // 响应时间（毫秒）
	ErrorMessage    string // 错误信息
	CreatedAt       string // 投递时间
}

// opnWebhookDeliveryLogsColumns holds the columns for the table opn_webhook_delivery_logs.
var opnWebhookDeliveryLogsColumns = OpnWebhookDeliveryLogsColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	WebhookConfigId: "webhook_config_id",
	EventId:         "event_id",
	Attempt:         "attempt",
	RequestUrl:      "request_url",
	RequestHeaders:  "request_headers",
	ResponseStatus:  "response_status",
	ResponseBody:    "response_body",
	ResponseTimeMs:  "response_time_ms",
	ErrorMessage:    "error_message",
	CreatedAt:       "created_at",
}

// NewOpnWebhookDeliveryLogsDao creates and returns a new DAO object for table data access.
func NewOpnWebhookDeliveryLogsDao(handlers ...gdb.ModelHandler) *OpnWebhookDeliveryLogsDao {
	return &OpnWebhookDeliveryLogsDao{
		group:    "default",
		table:    "opn_webhook_delivery_logs",
		columns:  opnWebhookDeliveryLogsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OpnWebhookDeliveryLogsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OpnWebhookDeliveryLogsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OpnWebhookDeliveryLogsDao) Columns() OpnWebhookDeliveryLogsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OpnWebhookDeliveryLogsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OpnWebhookDeliveryLogsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OpnWebhookDeliveryLogsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
