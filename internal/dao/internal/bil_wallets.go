// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BilWalletsDao is the data access object for the table bil_wallets.
type BilWalletsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  BilWalletsColumns  // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// BilWalletsColumns defines and stores column names for the table bil_wallets.
type BilWalletsColumns struct {
	Id                 string // 主键ID
	TenantId           string // 租户ID（每个租户一个钱包）
	Balance            string // 总余额
	FrozenBalance      string // 冻结余额（支付中/退款中，可用余额 = balance - frozen_balance）
	WarningThreshold   string // 余额预警线（低于此值触发通知）
	Currency           string // 货币（USD）
	CreatedAt          string // 创建时间
	UpdatedAt          string // 更新时间
	CumulativeRecharge string // 累计充值总额（USD）
}

// bilWalletsColumns holds the columns for the table bil_wallets.
var bilWalletsColumns = BilWalletsColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	Balance:            "balance",
	FrozenBalance:      "frozen_balance",
	WarningThreshold:   "warning_threshold",
	Currency:           "currency",
	CreatedAt:          "created_at",
	UpdatedAt:          "updated_at",
	CumulativeRecharge: "cumulative_recharge",
}

// NewBilWalletsDao creates and returns a new DAO object for table data access.
func NewBilWalletsDao(handlers ...gdb.ModelHandler) *BilWalletsDao {
	return &BilWalletsDao{
		group:    "default",
		table:    "bil_wallets",
		columns:  bilWalletsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BilWalletsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BilWalletsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BilWalletsDao) Columns() BilWalletsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BilWalletsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BilWalletsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BilWalletsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
