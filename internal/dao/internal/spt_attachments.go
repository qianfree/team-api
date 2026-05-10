// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SptAttachmentsDao is the data access object for the table spt_attachments.
type SptAttachmentsDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  SptAttachmentsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// SptAttachmentsColumns defines and stores column names for the table spt_attachments.
type SptAttachmentsColumns struct {
	Id          string // 主键ID
	TicketId    string // 工单ID
	ReplyId     string // 回复ID（NULL表示工单创建时的附件）
	FileName    string // 文件名
	FileUrl     string // 文件访问地址
	FileSize    string // 文件大小（字节）
	ContentType string // 文件MIME类型
	CreatedAt   string // 上传时间
}

// sptAttachmentsColumns holds the columns for the table spt_attachments.
var sptAttachmentsColumns = SptAttachmentsColumns{
	Id:          "id",
	TicketId:    "ticket_id",
	ReplyId:     "reply_id",
	FileName:    "file_name",
	FileUrl:     "file_url",
	FileSize:    "file_size",
	ContentType: "content_type",
	CreatedAt:   "created_at",
}

// NewSptAttachmentsDao creates and returns a new DAO object for table data access.
func NewSptAttachmentsDao(handlers ...gdb.ModelHandler) *SptAttachmentsDao {
	return &SptAttachmentsDao{
		group:    "default",
		table:    "spt_attachments",
		columns:  sptAttachmentsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SptAttachmentsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SptAttachmentsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SptAttachmentsDao) Columns() SptAttachmentsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SptAttachmentsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SptAttachmentsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SptAttachmentsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
