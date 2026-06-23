// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MdlModelGroupsDao is the data access object for the table mdl_model_groups.
type MdlModelGroupsDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  MdlModelGroupsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// MdlModelGroupsColumns defines and stores column names for the table mdl_model_groups.
type MdlModelGroupsColumns struct {
	Id          string // 主键ID
	Name        string // 分组名称（如"全量模型"、"基础对话"）
	Code        string // 分组唯一标识（如 full_access、basic_chat）
	Description string // 分组描述
	Status      string // 状态：active（启用）/ disabled（禁用）
	CreatedAt   string // 创建时间
	UpdatedAt   string // 更新时间
	IsDefault   string // 是否为新租户默认模型组，注册时自动关联
}

// mdlModelGroupsColumns holds the columns for the table mdl_model_groups.
var mdlModelGroupsColumns = MdlModelGroupsColumns{
	Id:          "id",
	Name:        "name",
	Code:        "code",
	Description: "description",
	Status:      "status",
	CreatedAt:   "created_at",
	UpdatedAt:   "updated_at",
	IsDefault:   "is_default",
}

// NewMdlModelGroupsDao creates and returns a new DAO object for table data access.
func NewMdlModelGroupsDao(handlers ...gdb.ModelHandler) *MdlModelGroupsDao {
	return &MdlModelGroupsDao{
		group:    "default",
		table:    "mdl_model_groups",
		columns:  mdlModelGroupsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MdlModelGroupsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MdlModelGroupsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MdlModelGroupsDao) Columns() MdlModelGroupsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MdlModelGroupsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MdlModelGroupsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MdlModelGroupsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
