// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysEmailVerifyCodes is the golang structure of table sys_email_verify_codes for DAO operations like Where/Data.
type SysEmailVerifyCodes struct {
	g.Meta    `orm:"table:sys_email_verify_codes, do:true"`
	Id        any         // 主键ID
	Email     any         // 目标邮箱地址
	Code      any         // 验证码（6位数字）
	Purpose   any         // 用途：register（注册）/ reset_password（重置密码）/ change_email（更换邮箱）
	ExpiresAt *gtime.Time // 过期时间（10分钟有效）
	UsedAt    *gtime.Time // 使用时间（NULL表示未使用）
	CreatedAt *gtime.Time // 创建时间
	UpdatedAt *gtime.Time // 更新时间
}
