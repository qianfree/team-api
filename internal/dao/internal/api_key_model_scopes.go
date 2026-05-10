// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ApiKeyModelScopesDao is the data access object for the table api_key_model_scopes.
type ApiKeyModelScopesDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  ApiKeyModelScopesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// ApiKeyModelScopesColumns defines and stores column names for the table api_key_model_scopes.
type ApiKeyModelScopesColumns struct {
	Id        string // 主键ID
	ApiKeyId  string // 关联 API Key ID
	ModelName string // 允许调用的模型名
	CreatedAt string // 创建时间
	UpdatedAt string // 更新时间
}

// apiKeyModelScopesColumns holds the columns for the table api_key_model_scopes.
var apiKeyModelScopesColumns = ApiKeyModelScopesColumns{
	Id:        "id",
	ApiKeyId:  "api_key_id",
	ModelName: "model_name",
	CreatedAt: "created_at",
	UpdatedAt: "updated_at",
}

// NewApiKeyModelScopesDao creates and returns a new DAO object for table data access.
func NewApiKeyModelScopesDao(handlers ...gdb.ModelHandler) *ApiKeyModelScopesDao {
	return &ApiKeyModelScopesDao{
		group:    "default",
		table:    "api_key_model_scopes",
		columns:  apiKeyModelScopesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ApiKeyModelScopesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ApiKeyModelScopesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ApiKeyModelScopesDao) Columns() ApiKeyModelScopesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ApiKeyModelScopesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ApiKeyModelScopesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ApiKeyModelScopesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
