// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// TntUsers is the golang structure of table tnt_users for DAO operations like Where/Data.
type TntUsers struct {
	g.Meta         `orm:"table:tnt_users, do:true"`
	Id             any         // 主键ID
	TenantId       any         // 所属租户ID
	Username       any         // 用户名（租户内唯一）
	Email          any         // 邮箱地址（租户内唯一）
	PasswordHash   any         // 密码哈希（bcrypt）
	DisplayName    any         // 显示名称
	Role           any         // 角色：owner（所有者）/ admin（管理员）/ member（成员）
	Status         any         // 状态：active（正常）/ disabled（禁用）/ locked（锁定）
	LastLoginAt    *gtime.Time // 最后登录时间
	LastLoginIp    any         // 最后登录IP
	FailedAttempts any         // 连续登录失败次数（成功登录后归零）
	LockedUntil    *gtime.Time // 锁定截止时间（连续5次失败后锁定30分钟）
	CreatedAt      *gtime.Time // 创建时间
	UpdatedAt      *gtime.Time // 更新时间
	TotpSecret     any         // TOTP 密钥（AES-256 加密存储）
	TotpEnabled    any         // 是否启用双因素认证
	BackupCodes    any         // 备用恢复码（bcrypt 哈希存储）
	QuotaType      any         // 额度限制类型：none（不限）/ total（总额）/ periodic（周期性）
	QuotaLimit     any         // 额度上限（USD），quota_type 为 none 时忽略
	QuotaUsed      any         // 已使用额度（USD）
	QuotaPeriod    any         // 周期类型：day / week / month（仅 periodic 时有效）
	QuotaResetAt   *gtime.Time // 上次额度重置时间（懒重置用）
}
