// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OrdRefundsDao is the data access object for the table ord_refunds.
type OrdRefundsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  OrdRefundsColumns  // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// OrdRefundsColumns defines and stores column names for the table ord_refunds.
type OrdRefundsColumns struct {
	Id              string // 主键ID
	OrderId         string // 关联订单ID
	TenantId        string // 租户ID
	Amount          string // 退款金额
	Reason          string // 退款原因
	Status          string // 退款状态
	PaymentChannel  string // 原支付渠道
	PaymentRefundId string // 第三方退款流水号
	ApprovedBy      string // 审批人（管理员ID）
	ApprovedAt      string // 审批时间
	CreatedAt       string // 创建时间
	UpdatedAt       string // 更新时间
}

// ordRefundsColumns holds the columns for the table ord_refunds.
var ordRefundsColumns = OrdRefundsColumns{
	Id:              "id",
	OrderId:         "order_id",
	TenantId:        "tenant_id",
	Amount:          "amount",
	Reason:          "reason",
	Status:          "status",
	PaymentChannel:  "payment_channel",
	PaymentRefundId: "payment_refund_id",
	ApprovedBy:      "approved_by",
	ApprovedAt:      "approved_at",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewOrdRefundsDao creates and returns a new DAO object for table data access.
func NewOrdRefundsDao(handlers ...gdb.ModelHandler) *OrdRefundsDao {
	return &OrdRefundsDao{
		group:    "default",
		table:    "ord_refunds",
		columns:  ordRefundsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OrdRefundsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OrdRefundsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OrdRefundsDao) Columns() OrdRefundsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OrdRefundsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OrdRefundsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OrdRefundsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
