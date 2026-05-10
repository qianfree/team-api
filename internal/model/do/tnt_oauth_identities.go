// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// TntOauthIdentities is the golang structure of table tnt_oauth_identities for DAO operations like Where/Data.
type TntOauthIdentities struct {
	g.Meta           `orm:"table:tnt_oauth_identities, do:true"`
	Id               any         // 主键ID
	TenantId         any         // 所属租户ID
	UserId           any         // 关联的用户ID
	Provider         any         // OAuth 供应商：github / google
	ProviderUserId   any         // 供应商用户ID
	ProviderUsername any         // 供应商用户名
	Email            any         // 供应商返回的邮箱
	AvatarUrl        any         // 供应商返回的头像URL
	AccessToken      any         // 加密存储的 access_token
	RefreshToken     any         // 加密存储的 refresh_token
	TokenExpiresAt   *gtime.Time // Token 过期时间
	RawData          any         // 供应商原始返回数据
	CreatedAt        *gtime.Time //
	UpdatedAt        *gtime.Time //
}
