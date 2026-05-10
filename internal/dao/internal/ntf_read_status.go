// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NtfReadStatusDao is the data access object for the table ntf_read_status.
type NtfReadStatusDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  NtfReadStatusColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// NtfReadStatusColumns defines and stores column names for the table ntf_read_status.
type NtfReadStatusColumns struct {
	Id        string // 主键ID
	MessageId string // 广播消息ID
	UserId    string // 已读用户ID
	ReadAt    string // 已读时间
}

// ntfReadStatusColumns holds the columns for the table ntf_read_status.
var ntfReadStatusColumns = NtfReadStatusColumns{
	Id:        "id",
	MessageId: "message_id",
	UserId:    "user_id",
	ReadAt:    "read_at",
}

// NewNtfReadStatusDao creates and returns a new DAO object for table data access.
func NewNtfReadStatusDao(handlers ...gdb.ModelHandler) *NtfReadStatusDao {
	return &NtfReadStatusDao{
		group:    "default",
		table:    "ntf_read_status",
		columns:  ntfReadStatusColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *NtfReadStatusDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *NtfReadStatusDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *NtfReadStatusDao) Columns() NtfReadStatusColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *NtfReadStatusDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *NtfReadStatusDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *NtfReadStatusDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
