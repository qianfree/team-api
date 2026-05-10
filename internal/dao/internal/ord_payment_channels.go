// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OrdPaymentChannelsDao is the data access object for the table ord_payment_channels.
type OrdPaymentChannelsDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  OrdPaymentChannelsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// OrdPaymentChannelsColumns defines and stores column names for the table ord_payment_channels.
type OrdPaymentChannelsColumns struct {
	Id          string // 主键ID
	Channel     string // 渠道标识（alipay/wechat/stripe/mock）
	Name        string // 显示名称
	Config      string // 渠道配置（JSONB，含 API 密钥等敏感信息）
	IsEnabled   string // 是否启用
	SortOrder   string // 排序权重
	CreatedAt   string // 创建时间
	UpdatedAt   string // 更新时间
	PaymentType string // 子支付方式（alipay/wxpay 等，空表示该渠道支持所有方式）
	CallbackUrl string // 支付回调地址覆盖（为空则使用系统默认）
	ReturnUrl   string // 支付完成后前端跳转地址覆盖
}

// ordPaymentChannelsColumns holds the columns for the table ord_payment_channels.
var ordPaymentChannelsColumns = OrdPaymentChannelsColumns{
	Id:          "id",
	Channel:     "channel",
	Name:        "name",
	Config:      "config",
	IsEnabled:   "is_enabled",
	SortOrder:   "sort_order",
	CreatedAt:   "created_at",
	UpdatedAt:   "updated_at",
	PaymentType: "payment_type",
	CallbackUrl: "callback_url",
	ReturnUrl:   "return_url",
}

// NewOrdPaymentChannelsDao creates and returns a new DAO object for table data access.
func NewOrdPaymentChannelsDao(handlers ...gdb.ModelHandler) *OrdPaymentChannelsDao {
	return &OrdPaymentChannelsDao{
		group:    "default",
		table:    "ord_payment_channels",
		columns:  ordPaymentChannelsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OrdPaymentChannelsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OrdPaymentChannelsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OrdPaymentChannelsDao) Columns() OrdPaymentChannelsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OrdPaymentChannelsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OrdPaymentChannelsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OrdPaymentChannelsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
