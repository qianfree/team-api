// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OrdOrdersDao is the data access object for the table ord_orders.
type OrdOrdersDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  OrdOrdersColumns   // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// OrdOrdersColumns defines and stores column names for the table ord_orders.
type OrdOrdersColumns struct {
	Id             string // 主键ID
	OrderNo        string // 订单号（唯一，格式 ORD + 时间戳 + 随机数）
	TenantId       string // 租户ID
	UserId         string // 下单用户ID
	OrderType      string // 订单类型：new_plan（新购）/ renew（续费）/ upgrade（升级）/ downgrade（降级）/ recharge（充值）
	PlanId         string // 套餐ID（充值订单时为 NULL）
	Amount         string // 原始金额
	DiscountAmount string // 优惠金额
	FinalAmount    string // 最终金额
	Currency       string // 货币
	PaymentChannel string // 支付渠道
	PaymentMethod  string // 支付方式描述
	PaymentNo      string // 第三方支付流水号
	Status         string // 订单状态
	PaidAt         string // 支付时间
	FulfilledAt    string // 履约完成时间
	ExpiredAt      string // 过期时间（未支付 30 分钟后自动过期）
	CancelledAt    string // 取消时间
	RelatedOrderId string // 关联订单ID（退款时指向原始订单）
	Description    string // 订单描述
	CreatedAt      string // 创建时间
	UpdatedAt      string // 更新时间
}

// ordOrdersColumns holds the columns for the table ord_orders.
var ordOrdersColumns = OrdOrdersColumns{
	Id:             "id",
	OrderNo:        "order_no",
	TenantId:       "tenant_id",
	UserId:         "user_id",
	OrderType:      "order_type",
	PlanId:         "plan_id",
	Amount:         "amount",
	DiscountAmount: "discount_amount",
	FinalAmount:    "final_amount",
	Currency:       "currency",
	PaymentChannel: "payment_channel",
	PaymentMethod:  "payment_method",
	PaymentNo:      "payment_no",
	Status:         "status",
	PaidAt:         "paid_at",
	FulfilledAt:    "fulfilled_at",
	ExpiredAt:      "expired_at",
	CancelledAt:    "cancelled_at",
	RelatedOrderId: "related_order_id",
	Description:    "description",
	CreatedAt:      "created_at",
	UpdatedAt:      "updated_at",
}

// NewOrdOrdersDao creates and returns a new DAO object for table data access.
func NewOrdOrdersDao(handlers ...gdb.ModelHandler) *OrdOrdersDao {
	return &OrdOrdersDao{
		group:    "default",
		table:    "ord_orders",
		columns:  ordOrdersColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OrdOrdersDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OrdOrdersDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OrdOrdersDao) Columns() OrdOrdersColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OrdOrdersDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OrdOrdersDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OrdOrdersDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
