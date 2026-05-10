// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TntProjectsDao is the data access object for the table tnt_projects.
type TntProjectsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  TntProjectsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// TntProjectsColumns defines and stores column names for the table tnt_projects.
type TntProjectsColumns struct {
	Id          string // 主键ID
	TenantId    string // 所属租户ID
	Name        string // 项目名称
	Description string // 项目描述
	Status      string // 状态：active（活跃）/ archived（归档）/ budget_exhausted（预算耗尽）
	Budget      string // 项目预算上限（NUMERIC(20,10) 金额，NULL 表示不限制）
	CreatedBy   string // 创建者用户ID
	CreatedAt   string // 创建时间
	UpdatedAt   string // 更新时间
}

// tntProjectsColumns holds the columns for the table tnt_projects.
var tntProjectsColumns = TntProjectsColumns{
	Id:          "id",
	TenantId:    "tenant_id",
	Name:        "name",
	Description: "description",
	Status:      "status",
	Budget:      "budget",
	CreatedBy:   "created_by",
	CreatedAt:   "created_at",
	UpdatedAt:   "updated_at",
}

// NewTntProjectsDao creates and returns a new DAO object for table data access.
func NewTntProjectsDao(handlers ...gdb.ModelHandler) *TntProjectsDao {
	return &TntProjectsDao{
		group:    "default",
		table:    "tnt_projects",
		columns:  tntProjectsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TntProjectsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TntProjectsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TntProjectsDao) Columns() TntProjectsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TntProjectsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TntProjectsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TntProjectsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
