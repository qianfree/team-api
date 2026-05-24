// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// BilRecords is the golang structure of table bil_records for DAO operations like Where/Data.
type BilRecords struct {
	g.Meta                  `orm:"table:bil_records, do:true"`
	Id                      any         // 主键ID
	TenantId                any         // 租户ID
	UserId                  any         // 用户ID
	ApiKeyId                any         // 使用的 API Key ID
	ChannelId               any         // 使用的渠道ID
	ModelName               any         // 调用的模型名
	RequestId               any         // 请求唯一ID（关联全链路追踪）
	RelayMode               any         // 代理模式：chat_completions / embeddings / images_generations 等
	InputTokens             any         // 输入 token 数
	OutputTokens            any         // 输出 token 数
	InputPrice              any         // 计费时输入单价（快照，防止价格变更影响历史记录）
	OutputPrice             any         // 计费时输出单价（快照）
	TotalCost               any         // 最终费用 = 基础价格 × 模型倍率 × 租户倍率
	Currency                any         // 货币（USD）
	Status                  any         // 状态：pre_deducted（已预扣）/ settled（已结算）/ refunded（已退款）
	SettledAt               *gtime.Time // 结算时间
	CreatedAt               *gtime.Time // 创建时间
	UpdatedAt               *gtime.Time // 更新时间
	BillingMode             any         // 实际计费模式
	EffectiveInputPrice     any         // 实际生效的输入单价（快照）
	EffectiveOutputPrice    any         // 实际生效的输出单价（快照）
	DiscountRatio           any         // 实际折扣比例（快照）
	BillingInputMultiplier  any         // 梯度定价输入乘数（快照）
	BillingOutputMultiplier any         // 梯度定价输出乘数（快照）
	CacheCreationTokens     any         // 缓存创建 token 数
	CacheReadTokens         any         // 缓存读取 token 数
	CacheCreationCost       any         // 缓存创建费用
	CacheReadCost           any         // 缓存读取费用
	ModelMultiplier         any         // 模型倍率（快照）
	TenantMultiplier        any         // 租户倍率（快照）
	BaseInputPrice          any         // 基础模型输入单价（快照，应用倍率前）
	BaseOutputPrice         any         // 基础模型输出单价（快照，应用倍率前）
	BillingSnapshot         any         // 完整计费计算过程快照（JSONB）
}
