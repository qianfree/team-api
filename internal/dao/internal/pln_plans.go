// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PlnPlansDao is the data access object for the table pln_plans.
type PlnPlansDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  PlnPlansColumns    // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// PlnPlansColumns defines and stores column names for the table pln_plans.
type PlnPlansColumns struct {
	Id                 string // 主键ID
	Name               string // 套餐显示名称
	Identifier         string // 套餐唯一标识（free/basic/pro/enterprise）
	Description        string // 套餐描述（面向用户的营销文案）
	MonthlyPrice       string // 月度价格（CNY）
	YearlyPrice        string // 年度价格（CNY，通常为月价×10）
	Status             string // 状态：active（上架）/ archived（下架）
	MonthlyQuotaTokens string // 每月 Token 配额（0=不限）
	AllowedModels      string // 允许使用的模型列表（NULL=全部，空数组=无）
	IsRecommended      string // 是否推荐
	SortOrder          string // 排序权重（数字越小越靠前）
	CreatedAt          string // 创建时间
	UpdatedAt          string // 更新时间
}

// plnPlansColumns holds the columns for the table pln_plans.
var plnPlansColumns = PlnPlansColumns{
	Id:                 "id",
	Name:               "name",
	Identifier:         "identifier",
	Description:        "description",
	MonthlyPrice:       "monthly_price",
	YearlyPrice:        "yearly_price",
	Status:             "status",
	MonthlyQuotaTokens: "monthly_quota_tokens",
	AllowedModels:      "allowed_models",
	IsRecommended:      "is_recommended",
	SortOrder:          "sort_order",
	CreatedAt:          "created_at",
	UpdatedAt:          "updated_at",
}

// NewPlnPlansDao creates and returns a new DAO object for table data access.
func NewPlnPlansDao(handlers ...gdb.ModelHandler) *PlnPlansDao {
	return &PlnPlansDao{
		group:    "default",
		table:    "pln_plans",
		columns:  plnPlansColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PlnPlansDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PlnPlansDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PlnPlansDao) Columns() PlnPlansColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PlnPlansDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PlnPlansDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PlnPlansDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
