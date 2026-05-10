// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OrdPromoCodesDao is the data access object for the table ord_promo_codes.
type OrdPromoCodesDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  OrdPromoCodesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// OrdPromoCodesColumns defines and stores column names for the table ord_promo_codes.
type OrdPromoCodesColumns struct {
	Id            string //
	Code          string // 优惠码文本（唯一）
	Name          string //
	Type          string // 类型：percentage（折扣百分比）/ fixed（立减固定金额）
	DiscountValue string // 折扣值（百分比 0-100，立减为金额）
	MinAmount     string // 最低订单金额
	MaxDiscount   string // 最大折扣金额（0=不限）
	TotalCount    string //
	UsedCount     string //
	PerUserLimit  string //
	ValidFrom     string //
	ValidTo       string //
	PlanIds       string // 适用套餐ID数组（NULL=全部）
	Status        string //
	CreatedAt     string //
	UpdatedAt     string //
}

// ordPromoCodesColumns holds the columns for the table ord_promo_codes.
var ordPromoCodesColumns = OrdPromoCodesColumns{
	Id:            "id",
	Code:          "code",
	Name:          "name",
	Type:          "type",
	DiscountValue: "discount_value",
	MinAmount:     "min_amount",
	MaxDiscount:   "max_discount",
	TotalCount:    "total_count",
	UsedCount:     "used_count",
	PerUserLimit:  "per_user_limit",
	ValidFrom:     "valid_from",
	ValidTo:       "valid_to",
	PlanIds:       "plan_ids",
	Status:        "status",
	CreatedAt:     "created_at",
	UpdatedAt:     "updated_at",
}

// NewOrdPromoCodesDao creates and returns a new DAO object for table data access.
func NewOrdPromoCodesDao(handlers ...gdb.ModelHandler) *OrdPromoCodesDao {
	return &OrdPromoCodesDao{
		group:    "default",
		table:    "ord_promo_codes",
		columns:  ordPromoCodesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OrdPromoCodesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OrdPromoCodesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OrdPromoCodesDao) Columns() OrdPromoCodesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OrdPromoCodesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OrdPromoCodesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OrdPromoCodesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
