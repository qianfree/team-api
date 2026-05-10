// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OrdRedemptionUsagesDao is the data access object for the table ord_redemption_usages.
type OrdRedemptionUsagesDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  OrdRedemptionUsagesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// OrdRedemptionUsagesColumns defines and stores column names for the table ord_redemption_usages.
type OrdRedemptionUsagesColumns struct {
	Id            string // 主键ID
	RedemptionId  string // 关联兑换码ID
	TenantId      string // 使用兑换码的租户ID
	UserId        string // 执行兑换操作的用户ID
	Type          string // 兑换类型：quota / plan / duration
	Value         string // 兑换面值（quota类型为金额，plan/duration为0）
	TransactionId string // 关联的交易流水ID（仅quota类型有值）
	CreatedAt     string // 创建时间
	UpdatedAt     string // 更新时间
}

// ordRedemptionUsagesColumns holds the columns for the table ord_redemption_usages.
var ordRedemptionUsagesColumns = OrdRedemptionUsagesColumns{
	Id:            "id",
	RedemptionId:  "redemption_id",
	TenantId:      "tenant_id",
	UserId:        "user_id",
	Type:          "type",
	Value:         "value",
	TransactionId: "transaction_id",
	CreatedAt:     "created_at",
	UpdatedAt:     "updated_at",
}

// NewOrdRedemptionUsagesDao creates and returns a new DAO object for table data access.
func NewOrdRedemptionUsagesDao(handlers ...gdb.ModelHandler) *OrdRedemptionUsagesDao {
	return &OrdRedemptionUsagesDao{
		group:    "default",
		table:    "ord_redemption_usages",
		columns:  ordRedemptionUsagesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OrdRedemptionUsagesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OrdRedemptionUsagesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OrdRedemptionUsagesDao) Columns() OrdRedemptionUsagesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OrdRedemptionUsagesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OrdRedemptionUsagesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OrdRedemptionUsagesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
