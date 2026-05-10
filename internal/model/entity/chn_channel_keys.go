// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// ChnChannelKeys is the golang structure for table chn_channel_keys.
type ChnChannelKeys struct {
	Id             int64       `json:"id"               orm:"id"               description:"主键ID"`                                          // 主键ID
	ChannelId      int64       `json:"channel_id"       orm:"channel_id"       description:"关联渠道ID"`                                        // 关联渠道ID
	Name           string      `json:"name"             orm:"name"             description:"Key 别名（用于管理标识，如\"主力Key\"、\"备用Key\"）"`           // Key 别名（用于管理标识，如"主力Key"、"备用Key"）
	EncryptedKey   string      `json:"encrypted_key"    orm:"encrypted_key"    description:"加密存储的 API Key 原值（AES-256）"`                     // 加密存储的 API Key 原值（AES-256）
	Status         string      `json:"status"           orm:"status"           description:"状态：active（可用）/ disabled（禁用）/ exhausted（额度耗尽）"`  // 状态：active（可用）/ disabled（禁用）/ exhausted（额度耗尽）
	LastUsedAt     *gtime.Time `json:"last_used_at"     orm:"last_used_at"     description:"最后使用时间"`                                        // 最后使用时间
	LastError      string      `json:"last_error"       orm:"last_error"       description:"最后一次错误信息"`                                      // 最后一次错误信息
	CreatedAt      *gtime.Time `json:"created_at"       orm:"created_at"       description:"创建时间"`                                          // 创建时间
	UpdatedAt      *gtime.Time `json:"updated_at"       orm:"updated_at"       description:"更新时间"`                                          // 更新时间
	KeyType        string      `json:"key_type"         orm:"key_type"         description:"Key 类型：apikey（传统静态密钥）/ oauth（OAuth 令牌）"`        // Key 类型：apikey（传统静态密钥）/ oauth（OAuth 令牌）
	TokenExpiresAt *gtime.Time `json:"token_expires_at" orm:"token_expires_at" description:"OAuth access_token 过期时间（仅 key_type=oauth 时有值）"` // OAuth access_token 过期时间（仅 key_type=oauth 时有值）
}
