// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OpnWebhookConfigsDao is the data access object for the table opn_webhook_configs.
type OpnWebhookConfigsDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  OpnWebhookConfigsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// OpnWebhookConfigsColumns defines and stores column names for the table opn_webhook_configs.
type OpnWebhookConfigsColumns struct {
	Id                     string // 主键ID
	TenantId               string // 所属租户ID
	Name                   string // 配置名称
	Url                    string // 回调地址（必须 HTTPS）
	SecretKey              string // HMAC-SHA256 签名密钥
	Events                 string // 订阅的事件类型列表
	IsActive               string // 是否启用
	RetryPolicy            string // 重试策略（JSON）
	ConsecutiveFailures    string // 连续失败次数
	MaxConsecutiveFailures string // 最大连续失败次数（超过后自动禁用）
	LastDeliveryAt         string // 最后投递时间
	CreatedAt              string // 创建时间
	UpdatedAt              string // 更新时间
}

// opnWebhookConfigsColumns holds the columns for the table opn_webhook_configs.
var opnWebhookConfigsColumns = OpnWebhookConfigsColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	Name:                   "name",
	Url:                    "url",
	SecretKey:              "secret_key",
	Events:                 "events",
	IsActive:               "is_active",
	RetryPolicy:            "retry_policy",
	ConsecutiveFailures:    "consecutive_failures",
	MaxConsecutiveFailures: "max_consecutive_failures",
	LastDeliveryAt:         "last_delivery_at",
	CreatedAt:              "created_at",
	UpdatedAt:              "updated_at",
}

// NewOpnWebhookConfigsDao creates and returns a new DAO object for table data access.
func NewOpnWebhookConfigsDao(handlers ...gdb.ModelHandler) *OpnWebhookConfigsDao {
	return &OpnWebhookConfigsDao{
		group:    "default",
		table:    "opn_webhook_configs",
		columns:  opnWebhookConfigsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OpnWebhookConfigsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OpnWebhookConfigsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OpnWebhookConfigsDao) Columns() OpnWebhookConfigsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OpnWebhookConfigsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OpnWebhookConfigsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OpnWebhookConfigsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
