// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MdlPricingDao is the data access object for the table mdl_pricing.
type MdlPricingDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  MdlPricingColumns  // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// MdlPricingColumns defines and stores column names for the table mdl_pricing.
type MdlPricingColumns struct {
	Id                 string //
	ModelId            string // 关联模型ID
	BillingMode        string // 计费模式：token（按量）/ per_request（按次）/ tiered（阶梯按量）
	MinTokens          string // 阶梯起始 token 数（仅 tiered 模式，其他模式为 0）
	MaxTokens          string // 阶梯结束 token 数（NULL=无上限，仅 tiered 模式）
	InputPrice         string // 每 1M input token 价格（token/tiered 模式）
	OutputPrice        string // 每 1M output token 价格（token/tiered 模式）
	PerRequestPrice    string // 按次计费单价（仅 per_request 模式）
	CreatedAt          string //
	UpdatedAt          string //
	CacheReadPrice     string // 缓存读取每 1M token 价格（直接定价）
	CacheCreationPrice string // 缓存创建每 1M token 价格（直接定价）
}

// mdlPricingColumns holds the columns for the table mdl_pricing.
var mdlPricingColumns = MdlPricingColumns{
	Id:                 "id",
	ModelId:            "model_id",
	BillingMode:        "billing_mode",
	MinTokens:          "min_tokens",
	MaxTokens:          "max_tokens",
	InputPrice:         "input_price",
	OutputPrice:        "output_price",
	PerRequestPrice:    "per_request_price",
	CreatedAt:          "created_at",
	UpdatedAt:          "updated_at",
	CacheReadPrice:     "cache_read_price",
	CacheCreationPrice: "cache_creation_price",
}

// NewMdlPricingDao creates and returns a new DAO object for table data access.
func NewMdlPricingDao(handlers ...gdb.ModelHandler) *MdlPricingDao {
	return &MdlPricingDao{
		group:    "default",
		table:    "mdl_pricing",
		columns:  mdlPricingColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MdlPricingDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MdlPricingDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MdlPricingDao) Columns() MdlPricingColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MdlPricingDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MdlPricingDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MdlPricingDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
