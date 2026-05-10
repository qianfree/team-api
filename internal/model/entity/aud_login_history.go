// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// AudLoginHistory is the golang structure for table aud_login_history.
type AudLoginHistory struct {
	Id                int64       `json:"id"                 orm:"id"                 description:"主键ID"`                                                      // 主键ID
	UserType          string      `json:"user_type"          orm:"user_type"          description:"用户类型：admin（管理后台）/ tenant（租户控制台）"`                           // 用户类型：admin（管理后台）/ tenant（租户控制台）
	UserId            int64       `json:"user_id"            orm:"user_id"            description:"用户ID"`                                                      // 用户ID
	TenantId          int64       `json:"tenant_id"          orm:"tenant_id"          description:"租户ID（仅 tenant 类型用户有值）"`                                     // 租户ID（仅 tenant 类型用户有值）
	LoginMethod       string      `json:"login_method"       orm:"login_method"       description:"登录方式：password（密码）/ totp（双因素）/ sso（单点登录）/ backup_code（恢复码）"` // 登录方式：password（密码）/ totp（双因素）/ sso（单点登录）/ backup_code（恢复码）
	IpAddress         string      `json:"ip_address"         orm:"ip_address"         description:"登录IP地址"`                                                    // 登录IP地址
	UserAgent         string      `json:"user_agent"         orm:"user_agent"         description:"浏览器 User-Agent"`                                            // 浏览器 User-Agent
	DeviceFingerprint string      `json:"device_fingerprint" orm:"device_fingerprint" description:"设备指纹（用于检测新设备登录）"`                                           // 设备指纹（用于检测新设备登录）
	Location          string      `json:"location"           orm:"location"           description:"IP 地理位置"`                                                   // IP 地理位置
	IsNewDevice       bool        `json:"is_new_device"      orm:"is_new_device"      description:"是否为新设备"`                                                    // 是否为新设备
	Success           bool        `json:"success"            orm:"success"            description:"登录是否成功"`                                                    // 登录是否成功
	FailReason        string      `json:"fail_reason"        orm:"fail_reason"        description:"登录失败原因"`                                                    // 登录失败原因
	CreatedAt         *gtime.Time `json:"created_at"         orm:"created_at"         description:"登录时间"`                                                      // 登录时间
}
