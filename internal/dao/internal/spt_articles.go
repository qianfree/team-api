// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SptArticlesDao is the data access object for the table spt_articles.
type SptArticlesDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  SptArticlesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// SptArticlesColumns defines and stores column names for the table spt_articles.
type SptArticlesColumns struct {
	Id          string // 主键ID
	CategoryId  string // 所属分类ID
	Title       string // 文章标题
	Slug        string // URL 友好标识，唯一
	Content     string // 文章内容（Markdown）
	Summary     string // 文章摘要
	Status      string // 状态：draft / published
	AuthorId    string // 作者（管理员）ID
	ViewCount   string // 浏览次数
	SortOrder   string // 排序序号，越小越靠前
	Keywords    string // 关键词（JSON 数组）
	PublishedAt string // 发布时间
	CreatedAt   string //
	UpdatedAt   string //
}

// sptArticlesColumns holds the columns for the table spt_articles.
var sptArticlesColumns = SptArticlesColumns{
	Id:          "id",
	CategoryId:  "category_id",
	Title:       "title",
	Slug:        "slug",
	Content:     "content",
	Summary:     "summary",
	Status:      "status",
	AuthorId:    "author_id",
	ViewCount:   "view_count",
	SortOrder:   "sort_order",
	Keywords:    "keywords",
	PublishedAt: "published_at",
	CreatedAt:   "created_at",
	UpdatedAt:   "updated_at",
}

// NewSptArticlesDao creates and returns a new DAO object for table data access.
func NewSptArticlesDao(handlers ...gdb.ModelHandler) *SptArticlesDao {
	return &SptArticlesDao{
		group:    "default",
		table:    "spt_articles",
		columns:  sptArticlesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SptArticlesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SptArticlesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SptArticlesDao) Columns() SptArticlesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SptArticlesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SptArticlesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SptArticlesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
