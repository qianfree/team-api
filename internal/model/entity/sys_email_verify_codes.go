// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysEmailVerifyCodes is the golang structure for table sys_email_verify_codes.
type SysEmailVerifyCodes struct {
	Id        int64       `json:"id"         orm:"id"         description:"主键ID"`                                                      // 主键ID
	Email     string      `json:"email"      orm:"email"      description:"目标邮箱地址"`                                                    // 目标邮箱地址
	Code      string      `json:"code"       orm:"code"       description:"验证码（6位数字）"`                                                 // 验证码（6位数字）
	Purpose   string      `json:"purpose"    orm:"purpose"    description:"用途：register（注册）/ reset_password（重置密码）/ change_email（更换邮箱）"` // 用途：register（注册）/ reset_password（重置密码）/ change_email（更换邮箱）
	ExpiresAt *gtime.Time `json:"expires_at" orm:"expires_at" description:"过期时间（10分钟有效）"`                                              // 过期时间（10分钟有效）
	UsedAt    *gtime.Time `json:"used_at"    orm:"used_at"    description:"使用时间（NULL表示未使用）"`                                           // 使用时间（NULL表示未使用）
	CreatedAt *gtime.Time `json:"created_at" orm:"created_at" description:"创建时间"`                                                      // 创建时间
	UpdatedAt *gtime.Time `json:"updated_at" orm:"updated_at" description:"更新时间"`                                                      // 更新时间
}
