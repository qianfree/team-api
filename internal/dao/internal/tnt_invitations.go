// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TntInvitationsDao is the data access object for the table tnt_invitations.
type TntInvitationsDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  TntInvitationsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// TntInvitationsColumns defines and stores column names for the table tnt_invitations.
type TntInvitationsColumns struct {
	Id           string // 主键ID
	TenantId     string // 所属租户ID
	Code         string // 邀请码（唯一标识）
	InvitedEmail string // 被邀请人邮箱（可选，指定后仅该邮箱可使用）
	Role         string // 邀请后分配的角色：owner / admin / member
	ExpiresAt    string // 过期时间：7天 / 30天 / 永久（NULL）
	UsedByUserId string // 使用该邀请注册的用户ID（NULL表示未使用）
	UsedAt       string // 使用时间
	CreatedBy    string // 创建者用户ID
	CreatedAt    string // 创建时间
	UpdatedAt    string // 更新时间
	MaxUses      string // 最大使用次数，0表示不限
	UseCount     string // 已使用次数
}

// tntInvitationsColumns holds the columns for the table tnt_invitations.
var tntInvitationsColumns = TntInvitationsColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	Code:         "code",
	InvitedEmail: "invited_email",
	Role:         "role",
	ExpiresAt:    "expires_at",
	UsedByUserId: "used_by_user_id",
	UsedAt:       "used_at",
	CreatedBy:    "created_by",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
	MaxUses:      "max_uses",
	UseCount:     "use_count",
}

// NewTntInvitationsDao creates and returns a new DAO object for table data access.
func NewTntInvitationsDao(handlers ...gdb.ModelHandler) *TntInvitationsDao {
	return &TntInvitationsDao{
		group:    "default",
		table:    "tnt_invitations",
		columns:  tntInvitationsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TntInvitationsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TntInvitationsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TntInvitationsDao) Columns() TntInvitationsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TntInvitationsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TntInvitationsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TntInvitationsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
