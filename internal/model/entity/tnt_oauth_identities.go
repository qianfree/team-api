// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// TntOauthIdentities is the golang structure for table tnt_oauth_identities.
type TntOauthIdentities struct {
	Id               int64       `json:"id"                orm:"id"                description:"主键ID"`                      // 主键ID
	TenantId         int64       `json:"tenant_id"         orm:"tenant_id"         description:"所属租户ID"`                    // 所属租户ID
	UserId           int64       `json:"user_id"           orm:"user_id"           description:"关联的用户ID"`                   // 关联的用户ID
	Provider         string      `json:"provider"          orm:"provider"          description:"OAuth 供应商：github / google"` // OAuth 供应商：github / google
	ProviderUserId   string      `json:"provider_user_id"  orm:"provider_user_id"  description:"供应商用户ID"`                   // 供应商用户ID
	ProviderUsername string      `json:"provider_username" orm:"provider_username" description:"供应商用户名"`                    // 供应商用户名
	Email            string      `json:"email"             orm:"email"             description:"供应商返回的邮箱"`                  // 供应商返回的邮箱
	AvatarUrl        string      `json:"avatar_url"        orm:"avatar_url"        description:"供应商返回的头像URL"`               // 供应商返回的头像URL
	AccessToken      string      `json:"access_token"      orm:"access_token"      description:"加密存储的 access_token"`        // 加密存储的 access_token
	RefreshToken     string      `json:"refresh_token"     orm:"refresh_token"     description:"加密存储的 refresh_token"`       // 加密存储的 refresh_token
	TokenExpiresAt   *gtime.Time `json:"token_expires_at"  orm:"token_expires_at"  description:"Token 过期时间"`                // Token 过期时间
	RawData          string      `json:"raw_data"          orm:"raw_data"          description:"供应商原始返回数据"`                 // 供应商原始返回数据
	CreatedAt        *gtime.Time `json:"created_at"        orm:"created_at"        description:""`                          //
	UpdatedAt        *gtime.Time `json:"updated_at"        orm:"updated_at"        description:""`                          //
}
