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
	Id              string //
	ModelId         string // 关联模型ID
	BillingMode     string // 计费模式：token（按量）/ per_request（按次）/ tiered（阶梯按量）
	CreatedAt       string //
	UpdatedAt       string //
	PriceNote       string // 价格说明（仅管理后台可见，调价背景等内部备注），仅 min_tokens=0 锚点行使用，NULL=无
	DiscountLabel   string // 折扣标签（对外展示，如"7折起"、"限时5折"），仅 min_tokens=0 锚点行使用，NULL/空=不展示
	PriceChangeNote string // 价格调整说明（对外展示，提示用户价格有变动，如"9月1日起输入价下调"），仅 min_tokens=0 锚点行使用，NULL/空=不展示
	Pricing         string // 计费详情（JSONB，按 billing_mode 单模式存储）：token={input_price,output_price,cache_read_price,cache_creation_price}；tiered={tiers:[{min_tokens,max_tokens,input_price,output_price}],cache_read_price,cache_creation_price}；per_request={price}；per_second={unit:"second",prices:{分辨率规格:每秒单价,"*":兜底价}}；顶层可含 time_segments（时段定价）
}

// mdlPricingColumns holds the columns for the table mdl_pricing.
var mdlPricingColumns = MdlPricingColumns{
	Id:              "id",
	ModelId:         "model_id",
	BillingMode:     "billing_mode",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
	PriceNote:       "price_note",
	DiscountLabel:   "discount_label",
	PriceChangeNote: "price_change_note",
	Pricing:         "pricing",
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
