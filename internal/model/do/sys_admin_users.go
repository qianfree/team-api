// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAdminUsers is the golang structure of table sys_admin_users for DAO operations like Where/Data.
type SysAdminUsers struct {
	g.Meta         `orm:"table:sys_admin_users, do:true"`
	Id             any         // 主键ID
	Username       any         // 登录用户名
	PasswordHash   any         // 密码哈希（bcrypt）
	Email          any         // 邮箱地址（可选，可为NULL）
	DisplayName    any         // 显示名称
	Role           any         // 角色：super_admin（全权限）/ admin（可配置权限）
	Status         any         // 状态：active（启用）/ disabled（禁用）
	LastLoginAt    *gtime.Time // 最后登录时间
	LastLoginIp    any         // 最后登录IP
	CreatedAt      *gtime.Time // 创建时间
	UpdatedAt      *gtime.Time // 更新时间
	TotpSecret     any         // TOTP 密钥（AES-256 加密存储）
	TotpEnabled    any         // 是否启用双因素认证
	BackupCodes    any         // 备用恢复码（bcrypt 哈希存储）
	FailedAttempts any         // 连续登录失败次数（成功登录后归零）
	LockedUntil    *gtime.Time // 锁定截止时间（连续5次失败后锁定30分钟）
}
