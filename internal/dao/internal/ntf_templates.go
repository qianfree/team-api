// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NtfTemplatesDao is the data access object for the table ntf_templates.
type NtfTemplatesDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  NtfTemplatesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// NtfTemplatesColumns defines and stores column names for the table ntf_templates.
type NtfTemplatesColumns struct {
	Id           string // 主键ID
	Code         string // 模板编码（唯一标识，如 email_verify_code、balance_warning）
	Channel      string // 发送渠道：email（邮件）/ sms（短信）/ webhook（Webhook）
	Subject      string // 邮件/消息主题
	BodyTemplate string // 消息体模板（支持变量占位符，如 {{.code}}）
	Variables    string // 模板变量列表（JSONB 数组，如 ["username", "tenant_name", "code"]）
	Status       string // 状态：active（启用）/ disabled（禁用）
	CreatedAt    string // 创建时间
	UpdatedAt    string // 更新时间
}

// ntfTemplatesColumns holds the columns for the table ntf_templates.
var ntfTemplatesColumns = NtfTemplatesColumns{
	Id:           "id",
	Code:         "code",
	Channel:      "channel",
	Subject:      "subject",
	BodyTemplate: "body_template",
	Variables:    "variables",
	Status:       "status",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewNtfTemplatesDao creates and returns a new DAO object for table data access.
func NewNtfTemplatesDao(handlers ...gdb.ModelHandler) *NtfTemplatesDao {
	return &NtfTemplatesDao{
		group:    "default",
		table:    "ntf_templates",
		columns:  ntfTemplatesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *NtfTemplatesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *NtfTemplatesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *NtfTemplatesDao) Columns() NtfTemplatesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *NtfTemplatesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *NtfTemplatesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *NtfTemplatesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
