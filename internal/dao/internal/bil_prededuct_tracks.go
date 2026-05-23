// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BilPredeductTracksDao is the data access object for the table bil_prededuct_tracks.
type BilPredeductTracksDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  BilPredeductTracksColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// BilPredeductTracksColumns defines and stores column names for the table bil_prededuct_tracks.
type BilPredeductTracksColumns struct {
	Id        string //
	TenantId  string // 租户 ID
	RequestId string // 请求唯一 ID
	Amount    string // 预扣金额（USD）
	ModelName string // 模型名称
	Status    string // frozen=冻结中, settled=已结算, expired=超时自动释放, released=手动释放
	CreatedAt string // 创建时间
	ExpiredAt string // 过期释放时间（仅 status=expired 时有值）
}

// bilPredeductTracksColumns holds the columns for the table bil_prededuct_tracks.
var bilPredeductTracksColumns = BilPredeductTracksColumns{
	Id:        "id",
	TenantId:  "tenant_id",
	RequestId: "request_id",
	Amount:    "amount",
	ModelName: "model_name",
	Status:    "status",
	CreatedAt: "created_at",
	ExpiredAt: "expired_at",
}

// NewBilPredeductTracksDao creates and returns a new DAO object for table data access.
func NewBilPredeductTracksDao(handlers ...gdb.ModelHandler) *BilPredeductTracksDao {
	return &BilPredeductTracksDao{
		group:    "default",
		table:    "bil_prededuct_tracks",
		columns:  bilPredeductTracksColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BilPredeductTracksDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BilPredeductTracksDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BilPredeductTracksDao) Columns() BilPredeductTracksColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BilPredeductTracksDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BilPredeductTracksDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BilPredeductTracksDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
