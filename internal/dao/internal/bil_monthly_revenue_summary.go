// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BilMonthlyRevenueSummaryDao is the data access object for the table bil_monthly_revenue_summary.
type BilMonthlyRevenueSummaryDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  BilMonthlyRevenueSummaryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// BilMonthlyRevenueSummaryColumns defines and stores column names for the table bil_monthly_revenue_summary.
type BilMonthlyRevenueSummaryColumns struct {
	Id               string //
	Month            string //
	TotalRecharge    string //
	TotalConsumption string //
	NetRevenue       string //
	CreatedAt        string //
	UpdatedAt        string //
}

// bilMonthlyRevenueSummaryColumns holds the columns for the table bil_monthly_revenue_summary.
var bilMonthlyRevenueSummaryColumns = BilMonthlyRevenueSummaryColumns{
	Id:               "id",
	Month:            "month",
	TotalRecharge:    "total_recharge",
	TotalConsumption: "total_consumption",
	NetRevenue:       "net_revenue",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
}

// NewBilMonthlyRevenueSummaryDao creates and returns a new DAO object for table data access.
func NewBilMonthlyRevenueSummaryDao(handlers ...gdb.ModelHandler) *BilMonthlyRevenueSummaryDao {
	return &BilMonthlyRevenueSummaryDao{
		group:    "default",
		table:    "bil_monthly_revenue_summary",
		columns:  bilMonthlyRevenueSummaryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BilMonthlyRevenueSummaryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BilMonthlyRevenueSummaryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BilMonthlyRevenueSummaryDao) Columns() BilMonthlyRevenueSummaryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BilMonthlyRevenueSummaryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BilMonthlyRevenueSummaryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BilMonthlyRevenueSummaryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
