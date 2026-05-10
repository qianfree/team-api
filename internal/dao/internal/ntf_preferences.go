// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NtfPreferencesDao is the data access object for the table ntf_preferences.
type NtfPreferencesDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  NtfPreferencesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// NtfPreferencesColumns defines and stores column names for the table ntf_preferences.
type NtfPreferencesColumns struct {
	Id          string // 主键ID
	TenantId    string // 所属租户ID
	UserId      string // 用户ID（组织级偏好时为 NULL）
	Scope       string // 偏好范围：user（用户级）/ org（组织级）
	Preferences string // 偏好配置（JSONB，如 {"billing":{"email":true,"in_app":true},"security":{"email":true,"in_app":true}}）
	CreatedAt   string // 创建时间
	UpdatedAt   string // 更新时间
}

// ntfPreferencesColumns holds the columns for the table ntf_preferences.
var ntfPreferencesColumns = NtfPreferencesColumns{
	Id:          "id",
	TenantId:    "tenant_id",
	UserId:      "user_id",
	Scope:       "scope",
	Preferences: "preferences",
	CreatedAt:   "created_at",
	UpdatedAt:   "updated_at",
}

// NewNtfPreferencesDao creates and returns a new DAO object for table data access.
func NewNtfPreferencesDao(handlers ...gdb.ModelHandler) *NtfPreferencesDao {
	return &NtfPreferencesDao{
		group:    "default",
		table:    "ntf_preferences",
		columns:  ntfPreferencesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *NtfPreferencesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *NtfPreferencesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *NtfPreferencesDao) Columns() NtfPreferencesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *NtfPreferencesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *NtfPreferencesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *NtfPreferencesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
