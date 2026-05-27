// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MdlGroupModelsDao is the data access object for the table mdl_group_models.
type MdlGroupModelsDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  MdlGroupModelsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// MdlGroupModelsColumns defines and stores column names for the table mdl_group_models.
type MdlGroupModelsColumns struct {
	Id        string // 主键ID
	GroupId   string // 分组ID（关联 mdl_model_groups.id）
	ModelId   string // 模型ID（关联 mdl_models.id）
	CreatedAt string // 创建时间
	UpdatedAt string // 更新时间
}

// mdlGroupModelsColumns holds the columns for the table mdl_group_models.
var mdlGroupModelsColumns = MdlGroupModelsColumns{
	Id:        "id",
	GroupId:   "group_id",
	ModelId:   "model_id",
	CreatedAt: "created_at",
	UpdatedAt: "updated_at",
}

// NewMdlGroupModelsDao creates and returns a new DAO object for table data access.
func NewMdlGroupModelsDao(handlers ...gdb.ModelHandler) *MdlGroupModelsDao {
	return &MdlGroupModelsDao{
		group:    "default",
		table:    "mdl_group_models",
		columns:  mdlGroupModelsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MdlGroupModelsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MdlGroupModelsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MdlGroupModelsDao) Columns() MdlGroupModelsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MdlGroupModelsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MdlGroupModelsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MdlGroupModelsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
