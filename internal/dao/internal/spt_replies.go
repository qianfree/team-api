// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SptRepliesDao is the data access object for the table spt_replies.
type SptRepliesDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  SptRepliesColumns  // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// SptRepliesColumns defines and stores column names for the table spt_replies.
type SptRepliesColumns struct {
	Id        string // 主键ID
	TicketId  string // 工单ID
	UserId    string // 回复者用户ID
	UserType  string // 回复者类型：admin（管理员）/ tenant（租户用户）
	Content   string // 回复内容
	CreatedAt string // 回复时间
}

// sptRepliesColumns holds the columns for the table spt_replies.
var sptRepliesColumns = SptRepliesColumns{
	Id:        "id",
	TicketId:  "ticket_id",
	UserId:    "user_id",
	UserType:  "user_type",
	Content:   "content",
	CreatedAt: "created_at",
}

// NewSptRepliesDao creates and returns a new DAO object for table data access.
func NewSptRepliesDao(handlers ...gdb.ModelHandler) *SptRepliesDao {
	return &SptRepliesDao{
		group:    "default",
		table:    "spt_replies",
		columns:  sptRepliesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SptRepliesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SptRepliesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SptRepliesDao) Columns() SptRepliesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SptRepliesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SptRepliesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SptRepliesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
