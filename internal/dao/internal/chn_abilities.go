// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChnAbilitiesDao is the data access object for the table chn_abilities.
type ChnAbilitiesDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  ChnAbilitiesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// ChnAbilitiesColumns defines and stores column names for the table chn_abilities.
type ChnAbilitiesColumns struct {
	Id            string // 主键ID
	ChannelId     string // 关联渠道ID
	ModelName     string // 平台标准模型名（用户请求使用的模型名）
	UpstreamModel string // 上游实际模型名（与平台标准名不同时需要映射，如平台名 gpt-4 → 上游名 gpt-4-0314）
	Enabled       string // 是否启用该模型能力
	CreatedAt     string // 创建时间
	UpdatedAt     string // 更新时间
}

// chnAbilitiesColumns holds the columns for the table chn_abilities.
var chnAbilitiesColumns = ChnAbilitiesColumns{
	Id:            "id",
	ChannelId:     "channel_id",
	ModelName:     "model_name",
	UpstreamModel: "upstream_model",
	Enabled:       "enabled",
	CreatedAt:     "created_at",
	UpdatedAt:     "updated_at",
}

// NewChnAbilitiesDao creates and returns a new DAO object for table data access.
func NewChnAbilitiesDao(handlers ...gdb.ModelHandler) *ChnAbilitiesDao {
	return &ChnAbilitiesDao{
		group:    "default",
		table:    "chn_abilities",
		columns:  chnAbilitiesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChnAbilitiesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChnAbilitiesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChnAbilitiesDao) Columns() ChnAbilitiesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChnAbilitiesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChnAbilitiesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChnAbilitiesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
