// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ClgChangelogsDao is the data access object for the table clg_changelogs.
type ClgChangelogsDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  ClgChangelogsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// ClgChangelogsColumns defines and stores column names for the table clg_changelogs.
type ClgChangelogsColumns struct {
	Id          string // 主键ID
	Version     string // 版本号
	Title       string // 标题
	Content     string // Markdown 内容
	Type        string // 类型：feature / fix / improvement / breaking
	Status      string // 状态：draft / published
	PublishedAt string // 发布时间
	CreatedBy   string // 创建的管理员 ID
	CreatedAt   string //
	UpdatedAt   string //
}

// clgChangelogsColumns holds the columns for the table clg_changelogs.
var clgChangelogsColumns = ClgChangelogsColumns{
	Id:          "id",
	Version:     "version",
	Title:       "title",
	Content:     "content",
	Type:        "type",
	Status:      "status",
	PublishedAt: "published_at",
	CreatedBy:   "created_by",
	CreatedAt:   "created_at",
	UpdatedAt:   "updated_at",
}

// NewClgChangelogsDao creates and returns a new DAO object for table data access.
func NewClgChangelogsDao(handlers ...gdb.ModelHandler) *ClgChangelogsDao {
	return &ClgChangelogsDao{
		group:    "default",
		table:    "clg_changelogs",
		columns:  clgChangelogsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ClgChangelogsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ClgChangelogsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ClgChangelogsDao) Columns() ClgChangelogsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ClgChangelogsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ClgChangelogsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ClgChangelogsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
