// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ChnChannelKeys is the golang structure of table chn_channel_keys for DAO operations like Where/Data.
type ChnChannelKeys struct {
	g.Meta         `orm:"table:chn_channel_keys, do:true"`
	Id             any         // 主键ID
	ChannelId      any         // 关联渠道ID
	Name           any         // Key 别名（用于管理标识，如"主力Key"、"备用Key"）
	EncryptedKey   any         // 加密存储的 API Key 原值（AES-256）
	Status         any         // 状态：active（可用）/ disabled（禁用）/ exhausted（额度耗尽）
	LastUsedAt     *gtime.Time // 最后使用时间
	LastError      any         // 最后一次错误信息
	CreatedAt      *gtime.Time // 创建时间
	KeyType        any         // Key 类型：apikey（传统静态密钥）/ oauth（OAuth 令牌）
	TokenExpiresAt *gtime.Time // OAuth access_token 过期时间（仅 key_type=oauth 时有值）
	UpdatedAt      *gtime.Time // 更新时间
}
