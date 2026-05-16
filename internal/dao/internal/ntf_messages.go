// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NtfMessagesDao is the data access object for the table ntf_messages.
type NtfMessagesDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  NtfMessagesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// NtfMessagesColumns defines and stores column names for the table ntf_messages.
type NtfMessagesColumns struct {
	Id          string // 主键ID
	TenantId    string // 所属租户ID
	UserId      string // 接收用户ID（广播消息时为 NULL）
	Type        string // 消息类型：billing（计费）/ system（系统）/ security（安全）/ invitation（邀请）/ announcement（公告）
	Title       string // 消息标题
	Content     string // 消息内容
	Channel     string // 发送渠道：in_app（站内）/ email（邮件）/ both（双渠道）
	IsRead      string // 是否已读：0=未读, 1=已读
	IsBroadcast string // 是否广播消息：0=个人消息, 1=广播消息
	Metadata    string // 附加元数据（JSONB，如关联资源ID、跳转链接等）
	TargetRoles string // 目标角色（NULL=全部角色，逗号分隔如 owner,admin 表示仅限这些角色）
	CreatedAt   string // 创建时间
}

// ntfMessagesColumns holds the columns for the table ntf_messages.
var ntfMessagesColumns = NtfMessagesColumns{
	Id:          "id",
	TenantId:    "tenant_id",
	UserId:      "user_id",
	Type:        "type",
	Title:       "title",
	Content:     "content",
	Channel:     "channel",
	IsRead:      "is_read",
	IsBroadcast: "is_broadcast",
	Metadata:    "metadata",
	TargetRoles: "target_roles",
	CreatedAt:   "created_at",
}

// NewNtfMessagesDao creates and returns a new DAO object for table data access.
func NewNtfMessagesDao(handlers ...gdb.ModelHandler) *NtfMessagesDao {
	return &NtfMessagesDao{
		group:    "default",
		table:    "ntf_messages",
		columns:  ntfMessagesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *NtfMessagesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *NtfMessagesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *NtfMessagesDao) Columns() NtfMessagesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *NtfMessagesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *NtfMessagesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *NtfMessagesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
