// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BilTransactionsDao is the data access object for the table bil_transactions.
type BilTransactionsDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  BilTransactionsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// BilTransactionsColumns defines and stores column names for the table bil_transactions.
type BilTransactionsColumns struct {
	Id           string // 主键ID
	TenantId     string // 租户ID
	WalletId     string // 关联钱包ID
	Type         string // 类型：recharge（充值）/ pre_deduct（预扣）/ settle（结算）/ refund（退款）/ adjust（调整）/ freeze（冻结）/ unfreeze（解冻）
	Amount       string // 变动金额（正数=收入，负数=支出）
	BalanceAfter string // 变动后总余额
	FrozenAfter  string // 变动后冻结余额
	RelatedId    string // 关联业务ID（如计费记录ID、订单ID等）
	RelatedType  string // 关联业务类型：billing_record / order / refund / adjustment / redemption
	Description  string // 交易描述
	CreatedAt    string // 创建时间
	UpdatedAt    string // 更新时间
}

// bilTransactionsColumns holds the columns for the table bil_transactions.
var bilTransactionsColumns = BilTransactionsColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	WalletId:     "wallet_id",
	Type:         "type",
	Amount:       "amount",
	BalanceAfter: "balance_after",
	FrozenAfter:  "frozen_after",
	RelatedId:    "related_id",
	RelatedType:  "related_type",
	Description:  "description",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewBilTransactionsDao creates and returns a new DAO object for table data access.
func NewBilTransactionsDao(handlers ...gdb.ModelHandler) *BilTransactionsDao {
	return &BilTransactionsDao{
		group:    "default",
		table:    "bil_transactions",
		columns:  bilTransactionsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BilTransactionsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BilTransactionsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BilTransactionsDao) Columns() BilTransactionsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BilTransactionsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BilTransactionsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BilTransactionsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
