// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// ApiKeys is the golang structure for table api_keys.
type ApiKeys struct {
	Id                   int64            `json:"id"                     orm:"id"                     description:"主键ID"`                                                                                              // 主键ID
	TenantId             int64            `json:"tenant_id"              orm:"tenant_id"              description:"所属租户ID"`                                                                                            // 所属租户ID
	UserId               int64            `json:"user_id"                orm:"user_id"                description:"创建者用户ID"`                                                                                           // 创建者用户ID
	Name                 string           `json:"name"                   orm:"name"                   description:"Key 名称（如 \"生产环境\"、\"测试用\"）"`                                                                        // Key 名称（如 "生产环境"、"测试用"）
	EncryptedKey         string           `json:"encrypted_key"          orm:"encrypted_key"          description:"加密存储的完整 API Key（AES-256）"`                                                                          // 加密存储的完整 API Key（AES-256）
	KeyPrefix            string           `json:"key_prefix"             orm:"key_prefix"             description:"Key 前缀（用于快速查找，明文存储，如 sk-a1b2c3d4）"`                                                                 // Key 前缀（用于快速查找，明文存储，如 sk-a1b2c3d4）
	Scope                string           `json:"scope"                  orm:"scope"                  description:"权限范围：full（全部）/ chat_only（仅对话）/ embeddings_only（仅嵌入）/ images_only（仅图像）/ read_only（只读）/ custom（自定义）"` // 权限范围：full（全部）/ chat_only（仅对话）/ embeddings_only（仅嵌入）/ images_only（仅图像）/ read_only（只读）/ custom（自定义）
	Status               string           `json:"status"                 orm:"status"                 description:"状态：active（正常）/ disabled（禁用）/ expired（已过期）"`                                                         // 状态：active（正常）/ disabled（禁用）/ expired（已过期）
	ExpiresAt            *gtime.Time      `json:"expires_at"             orm:"expires_at"             description:"过期时间（NULL 表示永不过期）"`                                                                                 // 过期时间（NULL 表示永不过期）
	RateLimitQps         int              `json:"rate_limit_qps"         orm:"rate_limit_qps"         description:"QPS 限流阈值（NULL 表示使用默认值）"`                                                                            // QPS 限流阈值（NULL 表示使用默认值）
	RateLimitConcurrency int              `json:"rate_limit_concurrency" orm:"rate_limit_concurrency" description:"并发限制阈值（NULL 表示使用默认值）"`                                                                              // 并发限制阈值（NULL 表示使用默认值）
	IpWhitelist          []string         `json:"ip_whitelist"           orm:"ip_whitelist"           description:"IP 白名单数组（NULL 或空数组表示不限制）"`                                                                          // IP 白名单数组（NULL 或空数组表示不限制）
	TotalQuota           *decimal.Decimal `json:"total_quota"            orm:"total_quota"            description:"额度上限（NULL 表示不限制）"`                                                                                  // 额度上限（NULL 表示不限制）
	UsedQuota            decimal.Decimal  `json:"used_quota"             orm:"used_quota"             description:"已使用额度"`                                                                                             // 已使用额度
	ProjectId            int64            `json:"project_id"             orm:"project_id"             description:"关联项目ID（NULL 表示不属于任何项目）"`                                                                            // 关联项目ID（NULL 表示不属于任何项目）
	CreatedAt            *gtime.Time      `json:"created_at"             orm:"created_at"             description:"创建时间"`                                                                                              // 创建时间
	UpdatedAt            *gtime.Time      `json:"updated_at"             orm:"updated_at"             description:"更新时间"`                                                                                              // 更新时间
	KeyType              string           `json:"key_type"               orm:"key_type"               description:"密钥类型：personal（个人密钥）/ project（项目密钥）"`                                                                // 密钥类型：personal（个人密钥）/ project（项目密钥）
}
