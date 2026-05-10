// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChnChannelAffinitiesDao is the data access object for the table chn_channel_affinities.
type ChnChannelAffinitiesDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  ChnChannelAffinitiesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// ChnChannelAffinitiesColumns defines and stores column names for the table chn_channel_affinities.
type ChnChannelAffinitiesColumns struct {
	Id        string // 主键ID
	TenantId  string // 租户ID
	UserId    string // 用户ID
	ModelName string // 模型名
	ChannelId string // 绑定的渠道ID
	HitCount  string // 命中次数（同一渠道连续成功次数）
	ExpiresAt string // 过期时间（默认 1800 秒后过期）
	CreatedAt string // 创建时间
	UpdatedAt string // 更新时间
}

// chnChannelAffinitiesColumns holds the columns for the table chn_channel_affinities.
var chnChannelAffinitiesColumns = ChnChannelAffinitiesColumns{
	Id:        "id",
	TenantId:  "tenant_id",
	UserId:    "user_id",
	ModelName: "model_name",
	ChannelId: "channel_id",
	HitCount:  "hit_count",
	ExpiresAt: "expires_at",
	CreatedAt: "created_at",
	UpdatedAt: "updated_at",
}

// NewChnChannelAffinitiesDao creates and returns a new DAO object for table data access.
func NewChnChannelAffinitiesDao(handlers ...gdb.ModelHandler) *ChnChannelAffinitiesDao {
	return &ChnChannelAffinitiesDao{
		group:    "default",
		table:    "chn_channel_affinities",
		columns:  chnChannelAffinitiesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChnChannelAffinitiesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChnChannelAffinitiesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChnChannelAffinitiesDao) Columns() ChnChannelAffinitiesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChnChannelAffinitiesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChnChannelAffinitiesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChnChannelAffinitiesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
