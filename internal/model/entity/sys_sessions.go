// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysSessions is the golang structure for table sys_sessions.
type SysSessions struct {
	Id               int64       `json:"id"                 orm:"id"                 description:"主键ID"`                                     // 主键ID
	UserType         string      `json:"user_type"          orm:"user_type"          description:"用户类型：admin（管理后台）/ tenant（租户控制台）"`          // 用户类型：admin（管理后台）/ tenant（租户控制台）
	UserId           int64       `json:"user_id"            orm:"user_id"            description:"用户ID"`                                     // 用户ID
	TenantId         int64       `json:"tenant_id"          orm:"tenant_id"          description:"租户ID（admin类型时为NULL）"`                      // 租户ID（admin类型时为NULL）
	RefreshTokenHash string      `json:"refresh_token_hash" orm:"refresh_token_hash" description:"Refresh Token 哈希值"`                        // Refresh Token 哈希值
	DeviceInfo       string      `json:"device_info"        orm:"device_info"        description:"设备信息（JSONB：浏览器、操作系统等）"`                    // 设备信息（JSONB：浏览器、操作系统等）
	IpAddress        string      `json:"ip_address"         orm:"ip_address"         description:"登录IP地址"`                                   // 登录IP地址
	ExpiresAt        *gtime.Time `json:"expires_at"         orm:"expires_at"         description:"Token 过期时间"`                               // Token 过期时间
	CreatedAt        *gtime.Time `json:"created_at"         orm:"created_at"         description:"创建时间"`                                     // 创建时间
	UpdatedAt        *gtime.Time `json:"updated_at"         orm:"updated_at"         description:"更新时间"`                                     // 更新时间
	Jti              string      `json:"jti"                orm:"jti"                description:"JWT ID (jti)，会话唯一标识符（UUID），用于 Redis 吊销缓存"` // JWT ID (jti)，会话唯一标识符（UUID），用于 Redis 吊销缓存
}
