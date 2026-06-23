// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OpnApps is the golang structure of table opn_apps for DAO operations like Where/Data.
type OpnApps struct {
	g.Meta          `orm:"table:opn_apps, do:true"`
	Id              any         // 主键ID
	TenantId        any         // 所属租户ID
	Name            any         // 应用名称
	Description     any         // 应用描述
	AppId           any         // 应用标识（opn_xxx 格式）
	AppSecretHash   any         // App Secret 哈希（bcrypt）
	Permissions     any         // 权限范围（JSON 数组）
	IpWhitelist     any         // IP 白名单（JSON 数组，为空则不限制）
	CallbackUrl     any         // OAuth 回调 URL
	IsSandbox       any         // 是否沙箱应用
	Status          any         // 状态：active（启用）/ disabled（禁用）
	RateLimit       any         // 每分钟请求上限
	LastUsedAt      *gtime.Time // 最后使用时间
	CreatedAt       *gtime.Time // 创建时间
	UpdatedAt       *gtime.Time // 更新时间
	EncryptedSecret any         // AES-256 encrypted App Secret for HMAC verification
}
