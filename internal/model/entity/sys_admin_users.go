// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAdminUsers is the golang structure for table sys_admin_users.
type SysAdminUsers struct {
	Id           int64       `json:"id"            orm:"id"            description:"主键ID"`                              // 主键ID
	Username     string      `json:"username"      orm:"username"      description:"登录用户名"`                             // 登录用户名
	PasswordHash string      `json:"password_hash" orm:"password_hash" description:"密码哈希（bcrypt）"`                      // 密码哈希（bcrypt）
	Email        string      `json:"email"         orm:"email"         description:"邮箱地址（可选，可为NULL）"`                   // 邮箱地址（可选，可为NULL）
	DisplayName  string      `json:"display_name"  orm:"display_name"  description:"显示名称"`                              // 显示名称
	Role         string      `json:"role"          orm:"role"          description:"角色：super_admin（全权限）/ admin（可配置权限）"` // 角色：super_admin（全权限）/ admin（可配置权限）
	Status       string      `json:"status"        orm:"status"        description:"状态：active（启用）/ disabled（禁用）"`       // 状态：active（启用）/ disabled（禁用）
	LastLoginAt  *gtime.Time `json:"last_login_at" orm:"last_login_at" description:"最后登录时间"`                            // 最后登录时间
	LastLoginIp  string      `json:"last_login_ip" orm:"last_login_ip" description:"最后登录IP"`                            // 最后登录IP
	CreatedAt    *gtime.Time `json:"created_at"    orm:"created_at"    description:"创建时间"`                              // 创建时间
	UpdatedAt    *gtime.Time `json:"updated_at"    orm:"updated_at"    description:"更新时间"`                              // 更新时间
	TotpSecret   string      `json:"totp_secret"   orm:"totp_secret"   description:"TOTP 密钥（AES-256 加密存储）"`             // TOTP 密钥（AES-256 加密存储）
	TotpEnabled  bool        `json:"totp_enabled"  orm:"totp_enabled"  description:"是否启用双因素认证"`                         // 是否启用双因素认证
	BackupCodes  string      `json:"backup_codes"  orm:"backup_codes"  description:"备用恢复码（bcrypt 哈希存储）"`                // 备用恢复码（bcrypt 哈希存储）
}
