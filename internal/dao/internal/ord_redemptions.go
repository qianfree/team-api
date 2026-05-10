// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OrdRedemptionsDao is the data access object for the table ord_redemptions.
type OrdRedemptionsDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  OrdRedemptionsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// OrdRedemptionsColumns defines and stores column names for the table ord_redemptions.
type OrdRedemptionsColumns struct {
	Id           string //
	Code         string //
	Type         string // 类型：quota（额度）/ plan（套餐时长）/ duration（时长天数）
	Value        string //
	PlanId       string //
	DurationDays string //
	MaxUses      string //
	UsedCount    string //
	RedeemedBy   string //
	RedeemedAt   string //
	ExpiresAt    string //
	Status       string //
	BatchNo      string // 批次号（批量生成时，便于管理）
	CreatedAt    string //
	UpdatedAt    string //
}

// ordRedemptionsColumns holds the columns for the table ord_redemptions.
var ordRedemptionsColumns = OrdRedemptionsColumns{
	Id:           "id",
	Code:         "code",
	Type:         "type",
	Value:        "value",
	PlanId:       "plan_id",
	DurationDays: "duration_days",
	MaxUses:      "max_uses",
	UsedCount:    "used_count",
	RedeemedBy:   "redeemed_by",
	RedeemedAt:   "redeemed_at",
	ExpiresAt:    "expires_at",
	Status:       "status",
	BatchNo:      "batch_no",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewOrdRedemptionsDao creates and returns a new DAO object for table data access.
func NewOrdRedemptionsDao(handlers ...gdb.ModelHandler) *OrdRedemptionsDao {
	return &OrdRedemptionsDao{
		group:    "default",
		table:    "ord_redemptions",
		columns:  ordRedemptionsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OrdRedemptionsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OrdRedemptionsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OrdRedemptionsDao) Columns() OrdRedemptionsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OrdRedemptionsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OrdRedemptionsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OrdRedemptionsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
