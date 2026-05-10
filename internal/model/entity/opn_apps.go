// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// OpnApps is the golang structure for table opn_apps.
type OpnApps struct {
	Id            int64       `json:"id"              orm:"id"              description:"主键ID"`                        // 主键ID
	TenantId      int64       `json:"tenant_id"       orm:"tenant_id"       description:"所属租户ID"`                      // 所属租户ID
	Name          string      `json:"name"            orm:"name"            description:"应用名称"`                        // 应用名称
	Description   string      `json:"description"     orm:"description"     description:"应用描述"`                        // 应用描述
	AppId         string      `json:"app_id"          orm:"app_id"          description:"应用标识（opn_xxx 格式）"`            // 应用标识（opn_xxx 格式）
	AppSecretHash string      `json:"app_secret_hash" orm:"app_secret_hash" description:"App Secret 哈希（bcrypt）"`       // App Secret 哈希（bcrypt）
	Permissions   string      `json:"permissions"     orm:"permissions"     description:"权限范围（JSON 数组）"`               // 权限范围（JSON 数组）
	IpWhitelist   string      `json:"ip_whitelist"    orm:"ip_whitelist"    description:"IP 白名单（JSON 数组，为空则不限制）"`      // IP 白名单（JSON 数组，为空则不限制）
	CallbackUrl   string      `json:"callback_url"    orm:"callback_url"    description:"OAuth 回调 URL"`                // OAuth 回调 URL
	IsSandbox     bool        `json:"is_sandbox"      orm:"is_sandbox"      description:"是否沙箱应用"`                      // 是否沙箱应用
	Status        string      `json:"status"          orm:"status"          description:"状态：active（启用）/ disabled（禁用）"` // 状态：active（启用）/ disabled（禁用）
	RateLimit     int         `json:"rate_limit"      orm:"rate_limit"      description:"每分钟请求上限"`                     // 每分钟请求上限
	LastUsedAt    *gtime.Time `json:"last_used_at"    orm:"last_used_at"    description:"最后使用时间"`                      // 最后使用时间
	CreatedAt     *gtime.Time `json:"created_at"      orm:"created_at"      description:"创建时间"`                        // 创建时间
	UpdatedAt     *gtime.Time `json:"updated_at"      orm:"updated_at"      description:"更新时间"`                        // 更新时间
}
