// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OrdPromoCodeUsagesDao is the data access object for the table ord_promo_code_usages.
type OrdPromoCodeUsagesDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  OrdPromoCodeUsagesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// OrdPromoCodeUsagesColumns defines and stores column names for the table ord_promo_code_usages.
type OrdPromoCodeUsagesColumns struct {
	Id             string //
	PromoCodeId    string //
	TenantId       string //
	OrderId        string //
	UserId         string //
	DiscountAmount string // 实际折扣金额
	CreatedAt      string //
}

// ordPromoCodeUsagesColumns holds the columns for the table ord_promo_code_usages.
var ordPromoCodeUsagesColumns = OrdPromoCodeUsagesColumns{
	Id:             "id",
	PromoCodeId:    "promo_code_id",
	TenantId:       "tenant_id",
	OrderId:        "order_id",
	UserId:         "user_id",
	DiscountAmount: "discount_amount",
	CreatedAt:      "created_at",
}

// NewOrdPromoCodeUsagesDao creates and returns a new DAO object for table data access.
func NewOrdPromoCodeUsagesDao(handlers ...gdb.ModelHandler) *OrdPromoCodeUsagesDao {
	return &OrdPromoCodeUsagesDao{
		group:    "default",
		table:    "ord_promo_code_usages",
		columns:  ordPromoCodeUsagesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OrdPromoCodeUsagesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OrdPromoCodeUsagesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OrdPromoCodeUsagesDao) Columns() OrdPromoCodeUsagesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OrdPromoCodeUsagesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OrdPromoCodeUsagesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OrdPromoCodeUsagesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
