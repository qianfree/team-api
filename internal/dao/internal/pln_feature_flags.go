// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PlnFeatureFlagsDao is the data access object for the table pln_feature_flags.
type PlnFeatureFlagsDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  PlnFeatureFlagsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// PlnFeatureFlagsColumns defines and stores column names for the table pln_feature_flags.
type PlnFeatureFlagsColumns struct {
	Id             string // 主键ID
	FeatureKey     string // 功能标识（如 api_docs, export_csv）
	Description    string // 功能描述
	DefaultEnabled string // 默认是否启用
	Enabled        string // 当前是否启用（计算后的最终值）
	Source         string // 来源：plan（套餐）/ tenant（租户覆盖）/ manual（手动）
	SourceId       string // 来源ID（plan_id 或 tenant_id）
	TenantId       string // 关联租户ID（租户级覆盖时使用）
	PlanId         string // 关联套餐ID（套餐级配置时使用）
	CreatedAt      string // 创建时间
	UpdatedAt      string // 更新时间
}

// plnFeatureFlagsColumns holds the columns for the table pln_feature_flags.
var plnFeatureFlagsColumns = PlnFeatureFlagsColumns{
	Id:             "id",
	FeatureKey:     "feature_key",
	Description:    "description",
	DefaultEnabled: "default_enabled",
	Enabled:        "enabled",
	Source:         "source",
	SourceId:       "source_id",
	TenantId:       "tenant_id",
	PlanId:         "plan_id",
	CreatedAt:      "created_at",
	UpdatedAt:      "updated_at",
}

// NewPlnFeatureFlagsDao creates and returns a new DAO object for table data access.
func NewPlnFeatureFlagsDao(handlers ...gdb.ModelHandler) *PlnFeatureFlagsDao {
	return &PlnFeatureFlagsDao{
		group:    "default",
		table:    "pln_feature_flags",
		columns:  plnFeatureFlagsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PlnFeatureFlagsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PlnFeatureFlagsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PlnFeatureFlagsDao) Columns() PlnFeatureFlagsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PlnFeatureFlagsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PlnFeatureFlagsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PlnFeatureFlagsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
