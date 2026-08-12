// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BilMonthlyUsageSummaryDao is the data access object for the table bil_monthly_usage_summary.
type BilMonthlyUsageSummaryDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  BilMonthlyUsageSummaryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// BilMonthlyUsageSummaryColumns defines and stores column names for the table bil_monthly_usage_summary.
type BilMonthlyUsageSummaryColumns struct {
	Id            string //
	TenantId      string //
	Month         string //
	TotalRequests string //
	TotalTokens   string //
	TotalCost     string //
	CreatedAt     string //
	UpdatedAt     string //
}

// bilMonthlyUsageSummaryColumns holds the columns for the table bil_monthly_usage_summary.
var bilMonthlyUsageSummaryColumns = BilMonthlyUsageSummaryColumns{
	Id:            "id",
	TenantId:      "tenant_id",
	Month:         "month",
	TotalRequests: "total_requests",
	TotalTokens:   "total_tokens",
	TotalCost:     "total_cost",
	CreatedAt:     "created_at",
	UpdatedAt:     "updated_at",
}

// NewBilMonthlyUsageSummaryDao creates and returns a new DAO object for table data access.
func NewBilMonthlyUsageSummaryDao(handlers ...gdb.ModelHandler) *BilMonthlyUsageSummaryDao {
	return &BilMonthlyUsageSummaryDao{
		group:    "default",
		table:    "bil_monthly_usage_summary",
		columns:  bilMonthlyUsageSummaryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BilMonthlyUsageSummaryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BilMonthlyUsageSummaryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BilMonthlyUsageSummaryDao) Columns() BilMonthlyUsageSummaryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BilMonthlyUsageSummaryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BilMonthlyUsageSummaryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BilMonthlyUsageSummaryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
