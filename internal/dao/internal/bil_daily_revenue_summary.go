// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BilDailyRevenueSummaryDao is the data access object for the table bil_daily_revenue_summary.
type BilDailyRevenueSummaryDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  BilDailyRevenueSummaryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// BilDailyRevenueSummaryColumns defines and stores column names for the table bil_daily_revenue_summary.
type BilDailyRevenueSummaryColumns struct {
	Id               string //
	Date             string //
	TotalRecharge    string //
	TotalConsumption string //
	NetRevenue       string //
	NewOrders        string //
	PaidOrders       string //
	CreatedAt        string //
	UpdatedAt        string //
}

// bilDailyRevenueSummaryColumns holds the columns for the table bil_daily_revenue_summary.
var bilDailyRevenueSummaryColumns = BilDailyRevenueSummaryColumns{
	Id:               "id",
	Date:             "date",
	TotalRecharge:    "total_recharge",
	TotalConsumption: "total_consumption",
	NetRevenue:       "net_revenue",
	NewOrders:        "new_orders",
	PaidOrders:       "paid_orders",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
}

// NewBilDailyRevenueSummaryDao creates and returns a new DAO object for table data access.
func NewBilDailyRevenueSummaryDao(handlers ...gdb.ModelHandler) *BilDailyRevenueSummaryDao {
	return &BilDailyRevenueSummaryDao{
		group:    "default",
		table:    "bil_daily_revenue_summary",
		columns:  bilDailyRevenueSummaryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BilDailyRevenueSummaryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BilDailyRevenueSummaryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BilDailyRevenueSummaryDao) Columns() BilDailyRevenueSummaryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BilDailyRevenueSummaryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BilDailyRevenueSummaryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BilDailyRevenueSummaryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
