// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TntUsersDao is the data access object for the table tnt_users.
type TntUsersDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  TntUsersColumns    // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// TntUsersColumns defines and stores column names for the table tnt_users.
type TntUsersColumns struct {
	Id             string // 主键ID
	TenantId       string // 所属租户ID
	Username       string // 用户名（租户内唯一）
	Email          string // 邮箱地址（租户内唯一）
	PasswordHash   string // 密码哈希（bcrypt）
	DisplayName    string // 显示名称
	Role           string // 角色：owner（所有者）/ admin（管理员）/ member（成员）
	Status         string // 状态：active（正常）/ disabled（禁用）/ locked（锁定）
	LastLoginAt    string // 最后登录时间
	LastLoginIp    string // 最后登录IP
	FailedAttempts string // 连续登录失败次数（成功登录后归零）
	LockedUntil    string // 锁定截止时间（连续5次失败后锁定30分钟）
	CreatedAt      string // 创建时间
	UpdatedAt      string // 更新时间
	TotpSecret     string // TOTP 密钥（AES-256 加密存储）
	TotpEnabled    string // 是否启用双因素认证
	BackupCodes    string // 备用恢复码（bcrypt 哈希存储）
	QuotaType      string // 额度限制类型：none（不限）/ total（总额）/ periodic（周期性）
	QuotaLimit     string // 额度上限（USD），quota_type 为 none 时忽略
	QuotaUsed      string // 已使用额度（USD）
	QuotaPeriod    string // 周期类型：day / week / month（仅 periodic 时有效）
	QuotaResetAt   string // 上次额度重置时间（懒重置用）
}

// tntUsersColumns holds the columns for the table tnt_users.
var tntUsersColumns = TntUsersColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	Username:       "username",
	Email:          "email",
	PasswordHash:   "password_hash",
	DisplayName:    "display_name",
	Role:           "role",
	Status:         "status",
	LastLoginAt:    "last_login_at",
	LastLoginIp:    "last_login_ip",
	FailedAttempts: "failed_attempts",
	LockedUntil:    "locked_until",
	CreatedAt:      "created_at",
	UpdatedAt:      "updated_at",
	TotpSecret:     "totp_secret",
	TotpEnabled:    "totp_enabled",
	BackupCodes:    "backup_codes",
	QuotaType:      "quota_type",
	QuotaLimit:     "quota_limit",
	QuotaUsed:      "quota_used",
	QuotaPeriod:    "quota_period",
	QuotaResetAt:   "quota_reset_at",
}

// NewTntUsersDao creates and returns a new DAO object for table data access.
func NewTntUsersDao(handlers ...gdb.ModelHandler) *TntUsersDao {
	return &TntUsersDao{
		group:    "default",
		table:    "tnt_users",
		columns:  tntUsersColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TntUsersDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TntUsersDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TntUsersDao) Columns() TntUsersColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TntUsersDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TntUsersDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TntUsersDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
