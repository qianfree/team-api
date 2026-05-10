// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SptCategoriesDao is the data access object for the table spt_categories.
type SptCategoriesDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  SptCategoriesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// SptCategoriesColumns defines and stores column names for the table spt_categories.
type SptCategoriesColumns struct {
	Id           string // 主键ID
	ParentId     string // 父分类ID，0表示顶级分类
	Name         string // 分类名称
	Slug         string // URL 友好标识，唯一
	Description  string // 分类描述
	SortOrder    string // 排序序号，越小越靠前
	Icon         string // 图标名称
	IsVisible    string // 是否对外可见
	ArticleCount string // 分类下文章数量（冗余计数）
	CreatedAt    string //
	UpdatedAt    string //
}

// sptCategoriesColumns holds the columns for the table spt_categories.
var sptCategoriesColumns = SptCategoriesColumns{
	Id:           "id",
	ParentId:     "parent_id",
	Name:         "name",
	Slug:         "slug",
	Description:  "description",
	SortOrder:    "sort_order",
	Icon:         "icon",
	IsVisible:    "is_visible",
	ArticleCount: "article_count",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewSptCategoriesDao creates and returns a new DAO object for table data access.
func NewSptCategoriesDao(handlers ...gdb.ModelHandler) *SptCategoriesDao {
	return &SptCategoriesDao{
		group:    "default",
		table:    "spt_categories",
		columns:  sptCategoriesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SptCategoriesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SptCategoriesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SptCategoriesDao) Columns() SptCategoriesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SptCategoriesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SptCategoriesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SptCategoriesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
