// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// ApiKeys is the golang structure of table api_keys for DAO operations like Where/Data.
type ApiKeys struct {
	g.Meta               `orm:"table:api_keys, do:true"`
	Id                   any              // 主键ID
	TenantId             any              // 所属租户ID
	UserId               any              // 创建者用户ID
	Name                 any              // Key 名称（如 "生产环境"、"测试用"）
	EncryptedKey         any              // 加密存储的完整 API Key（AES-256）
	KeyPrefix            any              // Key 前缀（用于快速查找，明文存储，如 sk-a1b2c3d4）
	Scope                any              // 权限范围：full（全部）/ chat_only（仅对话）/ embeddings_only（仅嵌入）/ images_only（仅图像）/ read_only（只读）/ custom（自定义）
	Status               any              // 状态：active（正常）/ disabled（禁用）/ expired（已过期）
	ExpiresAt            *gtime.Time      // 过期时间（NULL 表示永不过期）
	RateLimitQps         any              // QPS 限流阈值（NULL 表示使用默认值）
	RateLimitConcurrency any              // 并发限制阈值（NULL 表示使用默认值）
	IpWhitelist          []string         // IP 白名单数组（NULL 或空数组表示不限制）
	TotalQuota           *decimal.Decimal // 额度上限（NULL 表示不限制）
	UsedQuota            any              // 已使用额度
	ProjectId            any              // 关联项目ID（NULL 表示不属于任何项目）
	CreatedAt            *gtime.Time      // 创建时间
	UpdatedAt            *gtime.Time      // 更新时间
	KeyType              any              // 密钥类型：personal（个人密钥）/ project（项目密钥）
}
