// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BilDailyUsageSummaryDao is the data access object for the table bil_daily_usage_summary.
type BilDailyUsageSummaryDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  BilDailyUsageSummaryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// BilDailyUsageSummaryColumns defines and stores column names for the table bil_daily_usage_summary.
type BilDailyUsageSummaryColumns struct {
	Id            string //
	TenantId      string //
	Date          string //
	TotalRequests string //
	TotalTokens   string //
	TotalCost     string //
	CreatedAt     string //
	UpdatedAt     string //
}

// bilDailyUsageSummaryColumns holds the columns for the table bil_daily_usage_summary.
var bilDailyUsageSummaryColumns = BilDailyUsageSummaryColumns{
	Id:            "id",
	TenantId:      "tenant_id",
	Date:          "date",
	TotalRequests: "total_requests",
	TotalTokens:   "total_tokens",
	TotalCost:     "total_cost",
	CreatedAt:     "created_at",
	UpdatedAt:     "updated_at",
}

// NewBilDailyUsageSummaryDao creates and returns a new DAO object for table data access.
func NewBilDailyUsageSummaryDao(handlers ...gdb.ModelHandler) *BilDailyUsageSummaryDao {
	return &BilDailyUsageSummaryDao{
		group:    "default",
		table:    "bil_daily_usage_summary",
		columns:  bilDailyUsageSummaryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BilDailyUsageSummaryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BilDailyUsageSummaryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BilDailyUsageSummaryDao) Columns() BilDailyUsageSummaryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BilDailyUsageSummaryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BilDailyUsageSummaryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BilDailyUsageSummaryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
