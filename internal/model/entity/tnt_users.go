// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// TntUsers is the golang structure for table tnt_users.
type TntUsers struct {
	Id             int64       `json:"id"              orm:"id"              description:"主键ID"`                                      // 主键ID
	TenantId       int64       `json:"tenant_id"       orm:"tenant_id"       description:"所属租户ID"`                                    // 所属租户ID
	Username       string      `json:"username"        orm:"username"        description:"用户名（租户内唯一）"`                                // 用户名（租户内唯一）
	Email          string      `json:"email"           orm:"email"           description:"邮箱地址（租户内唯一）"`                               // 邮箱地址（租户内唯一）
	PasswordHash   string      `json:"password_hash"   orm:"password_hash"   description:"密码哈希（bcrypt）"`                              // 密码哈希（bcrypt）
	DisplayName    string      `json:"display_name"    orm:"display_name"    description:"显示名称"`                                      // 显示名称
	Role           string      `json:"role"            orm:"role"            description:"角色：owner（所有者）/ admin（管理员）/ member（成员）"`     // 角色：owner（所有者）/ admin（管理员）/ member（成员）
	Status         string      `json:"status"          orm:"status"          description:"状态：active（正常）/ disabled（禁用）/ locked（锁定）"`   // 状态：active（正常）/ disabled（禁用）/ locked（锁定）
	LastLoginAt    *gtime.Time `json:"last_login_at"   orm:"last_login_at"   description:"最后登录时间"`                                    // 最后登录时间
	LastLoginIp    string      `json:"last_login_ip"   orm:"last_login_ip"   description:"最后登录IP"`                                    // 最后登录IP
	FailedAttempts int         `json:"failed_attempts" orm:"failed_attempts" description:"连续登录失败次数（成功登录后归零）"`                         // 连续登录失败次数（成功登录后归零）
	LockedUntil    *gtime.Time `json:"locked_until"    orm:"locked_until"    description:"锁定截止时间（连续5次失败后锁定30分钟）"`                     // 锁定截止时间（连续5次失败后锁定30分钟）
	CreatedAt      *gtime.Time `json:"created_at"      orm:"created_at"      description:"创建时间"`                                      // 创建时间
	UpdatedAt      *gtime.Time `json:"updated_at"      orm:"updated_at"      description:"更新时间"`                                      // 更新时间
	TotpSecret     string      `json:"totp_secret"     orm:"totp_secret"     description:"TOTP 密钥（AES-256 加密存储）"`                     // TOTP 密钥（AES-256 加密存储）
	TotpEnabled    bool        `json:"totp_enabled"    orm:"totp_enabled"    description:"是否启用双因素认证"`                                 // 是否启用双因素认证
	BackupCodes    string      `json:"backup_codes"    orm:"backup_codes"    description:"备用恢复码（bcrypt 哈希存储）"`                        // 备用恢复码（bcrypt 哈希存储）
	QuotaType      string      `json:"quota_type"      orm:"quota_type"      description:"额度限制类型：none（不限）/ total（总额）/ periodic（周期性）"` // 额度限制类型：none（不限）/ total（总额）/ periodic（周期性）
	QuotaLimit     float64     `json:"quota_limit"     orm:"quota_limit"     description:"额度上限（USD），quota_type 为 none 时忽略"`           // 额度上限（USD），quota_type 为 none 时忽略
	QuotaUsed      float64     `json:"quota_used"      orm:"quota_used"      description:"已使用额度（USD）"`                                // 已使用额度（USD）
	QuotaPeriod    string      `json:"quota_period"    orm:"quota_period"    description:"周期类型：day / week / month（仅 periodic 时有效）"`   // 周期类型：day / week / month（仅 periodic 时有效）
	QuotaResetAt   *gtime.Time `json:"quota_reset_at"  orm:"quota_reset_at"  description:"上次额度重置时间（懒重置用）"`                            // 上次额度重置时间（懒重置用）
}
