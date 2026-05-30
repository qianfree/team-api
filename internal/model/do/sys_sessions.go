// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysSessions is the golang structure of table sys_sessions for DAO operations like Where/Data.
type SysSessions struct {
	g.Meta           `orm:"table:sys_sessions, do:true"`
	Id               any         // 主键ID
	UserType         any         // 用户类型：admin（管理后台）/ tenant（租户控制台）
	UserId           any         // 用户ID
	TenantId         any         // 租户ID（admin类型时为NULL）
	RefreshTokenHash any         // Refresh Token 哈希值
	DeviceInfo       any         // 设备信息（JSONB：浏览器、操作系统等）
	IpAddress        any         // 登录IP地址
	ExpiresAt        *gtime.Time // Token 过期时间
	CreatedAt        *gtime.Time // 创建时间
	UpdatedAt        *gtime.Time // 更新时间
	Jti              any         // JWT ID (jti)，会话唯一标识符（UUID），用于 Redis 吊销缓存
}
