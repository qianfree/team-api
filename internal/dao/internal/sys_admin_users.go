// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysAdminUsersDao is the data access object for the table sys_admin_users.
type SysAdminUsersDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  SysAdminUsersColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// SysAdminUsersColumns defines and stores column names for the table sys_admin_users.
type SysAdminUsersColumns struct {
	Id             string // 主键ID
	Username       string // 登录用户名
	PasswordHash   string // 密码哈希（bcrypt）
	Email          string // 邮箱地址（可选，可为NULL）
	DisplayName    string // 显示名称
	Role           string // 角色：super_admin（全权限）/ admin（可配置权限）
	Status         string // 状态：active（启用）/ disabled（禁用）
	LastLoginAt    string // 最后登录时间
	LastLoginIp    string // 最后登录IP
	CreatedAt      string // 创建时间
	UpdatedAt      string // 更新时间
	TotpSecret     string // TOTP 密钥（AES-256 加密存储）
	TotpEnabled    string // 是否启用双因素认证
	BackupCodes    string // 备用恢复码（bcrypt 哈希存储）
	FailedAttempts string // 连续登录失败次数（成功登录后归零）
	LockedUntil    string // 锁定截止时间（连续5次失败后锁定30分钟）
}

// sysAdminUsersColumns holds the columns for the table sys_admin_users.
var sysAdminUsersColumns = SysAdminUsersColumns{
	Id:             "id",
	Username:       "username",
	PasswordHash:   "password_hash",
	Email:          "email",
	DisplayName:    "display_name",
	Role:           "role",
	Status:         "status",
	LastLoginAt:    "last_login_at",
	LastLoginIp:    "last_login_ip",
	CreatedAt:      "created_at",
	UpdatedAt:      "updated_at",
	TotpSecret:     "totp_secret",
	TotpEnabled:    "totp_enabled",
	BackupCodes:    "backup_codes",
	FailedAttempts: "failed_attempts",
	LockedUntil:    "locked_until",
}

// NewSysAdminUsersDao creates and returns a new DAO object for table data access.
func NewSysAdminUsersDao(handlers ...gdb.ModelHandler) *SysAdminUsersDao {
	return &SysAdminUsersDao{
		group:    "default",
		table:    "sys_admin_users",
		columns:  sysAdminUsersColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysAdminUsersDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysAdminUsersDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysAdminUsersDao) Columns() SysAdminUsersColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysAdminUsersDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SysAdminUsersDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SysAdminUsersDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
