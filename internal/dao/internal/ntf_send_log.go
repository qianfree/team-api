// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NtfSendLogDao is the data access object for the table ntf_send_log.
type NtfSendLogDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  NtfSendLogColumns  // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// NtfSendLogColumns defines and stores column names for the table ntf_send_log.
type NtfSendLogColumns struct {
	Id           string // 主键ID
	TenantId     string // 租户ID（系统级通知为 NULL）
	UserId       string // 目标用户ID
	TemplateCode string // 使用的通知模板编码
	Channel      string // 发送渠道：email / sms / webhook
	Recipient    string // 接收方（邮箱地址/手机号/Webhook URL）
	Subject      string // 发送主题
	Body         string // 发送内容（渲染后的最终内容）
	Status       string // 状态：pending（待发送）/ sent（已发送）/ failed（发送失败）
	ErrorMessage string // 失败时的错误信息
	SentAt       string // 实际发送时间
	RetryCount   string // 重试次数（最多重试 3 次）
	CreatedAt    string // 创建时间
	UpdatedAt    string // 更新时间
}

// ntfSendLogColumns holds the columns for the table ntf_send_log.
var ntfSendLogColumns = NtfSendLogColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	UserId:       "user_id",
	TemplateCode: "template_code",
	Channel:      "channel",
	Recipient:    "recipient",
	Subject:      "subject",
	Body:         "body",
	Status:       "status",
	ErrorMessage: "error_message",
	SentAt:       "sent_at",
	RetryCount:   "retry_count",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewNtfSendLogDao creates and returns a new DAO object for table data access.
func NewNtfSendLogDao(handlers ...gdb.ModelHandler) *NtfSendLogDao {
	return &NtfSendLogDao{
		group:    "default",
		table:    "ntf_send_log",
		columns:  ntfSendLogColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *NtfSendLogDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *NtfSendLogDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *NtfSendLogDao) Columns() NtfSendLogColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *NtfSendLogDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *NtfSendLogDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *NtfSendLogDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
