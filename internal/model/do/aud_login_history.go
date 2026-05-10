// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// AudLoginHistory is the golang structure of table aud_login_history for DAO operations like Where/Data.
type AudLoginHistory struct {
	g.Meta            `orm:"table:aud_login_history, do:true"`
	Id                any         // 主键ID
	UserType          any         // 用户类型：admin（管理后台）/ tenant（租户控制台）
	UserId            any         // 用户ID
	TenantId          any         // 租户ID（仅 tenant 类型用户有值）
	LoginMethod       any         // 登录方式：password（密码）/ totp（双因素）/ sso（单点登录）/ backup_code（恢复码）
	IpAddress         any         // 登录IP地址
	UserAgent         any         // 浏览器 User-Agent
	DeviceFingerprint any         // 设备指纹（用于检测新设备登录）
	Location          any         // IP 地理位置
	IsNewDevice       any         // 是否为新设备
	Success           any         // 登录是否成功
	FailReason        any         // 登录失败原因
	CreatedAt         *gtime.Time // 登录时间
}
