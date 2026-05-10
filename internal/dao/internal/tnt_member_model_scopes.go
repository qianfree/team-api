// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TntMemberModelScopesDao is the data access object for the table tnt_member_model_scopes.
type TntMemberModelScopesDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  TntMemberModelScopesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// TntMemberModelScopesColumns defines and stores column names for the table tnt_member_model_scopes.
type TntMemberModelScopesColumns struct {
	Id        string // 主键ID
	TenantId  string // 所属租户ID
	UserId    string // 成员用户ID
	ModelId   string // 模型ID
	CreatedAt string // 创建时间
}

// tntMemberModelScopesColumns holds the columns for the table tnt_member_model_scopes.
var tntMemberModelScopesColumns = TntMemberModelScopesColumns{
	Id:        "id",
	TenantId:  "tenant_id",
	UserId:    "user_id",
	ModelId:   "model_id",
	CreatedAt: "created_at",
}

// NewTntMemberModelScopesDao creates and returns a new DAO object for table data access.
func NewTntMemberModelScopesDao(handlers ...gdb.ModelHandler) *TntMemberModelScopesDao {
	return &TntMemberModelScopesDao{
		group:    "default",
		table:    "tnt_member_model_scopes",
		columns:  tntMemberModelScopesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TntMemberModelScopesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TntMemberModelScopesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TntMemberModelScopesDao) Columns() TntMemberModelScopesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TntMemberModelScopesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TntMemberModelScopesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TntMemberModelScopesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
