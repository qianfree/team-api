// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// BilRecords is the golang structure for table bil_records.
type BilRecords struct {
	Id                      int64       `json:"id"                        orm:"id"                        description:"主键ID"`                                                      // 主键ID
	TenantId                int64       `json:"tenant_id"                 orm:"tenant_id"                 description:"租户ID"`                                                      // 租户ID
	UserId                  int64       `json:"user_id"                   orm:"user_id"                   description:"用户ID"`                                                      // 用户ID
	ApiKeyId                int64       `json:"api_key_id"                orm:"api_key_id"                description:"使用的 API Key ID"`                                            // 使用的 API Key ID
	ChannelId               int64       `json:"channel_id"                orm:"channel_id"                description:"使用的渠道ID"`                                                   // 使用的渠道ID
	ModelName               string      `json:"model_name"                orm:"model_name"                description:"调用的模型名"`                                                    // 调用的模型名
	RequestId               string      `json:"request_id"                orm:"request_id"                description:"请求唯一ID（关联全链路追踪）"`                                           // 请求唯一ID（关联全链路追踪）
	RelayMode               string      `json:"relay_mode"                orm:"relay_mode"                description:"代理模式：chat_completions / embeddings / images_generations 等"` // 代理模式：chat_completions / embeddings / images_generations 等
	InputTokens             int         `json:"input_tokens"              orm:"input_tokens"              description:"输入 token 数"`                                                // 输入 token 数
	OutputTokens            int         `json:"output_tokens"             orm:"output_tokens"             description:"输出 token 数"`                                                // 输出 token 数
	InputPrice              float64     `json:"input_price"               orm:"input_price"               description:"计费时输入单价（快照，防止价格变更影响历史记录）"`                                  // 计费时输入单价（快照，防止价格变更影响历史记录）
	OutputPrice             float64     `json:"output_price"              orm:"output_price"              description:"计费时输出单价（快照）"`                                               // 计费时输出单价（快照）
	TotalCost               float64     `json:"total_cost"                orm:"total_cost"                description:"最终费用 = 基础价格 × 模型倍率 × 租户倍率"`                                 // 最终费用 = 基础价格 × 模型倍率 × 租户倍率
	Currency                string      `json:"currency"                  orm:"currency"                  description:"货币（USD）"`                                                   // 货币（USD）
	Status                  string      `json:"status"                    orm:"status"                    description:"状态：pre_deducted（已预扣）/ settled（已结算）/ refunded（已退款）"`         // 状态：pre_deducted（已预扣）/ settled（已结算）/ refunded（已退款）
	SettledAt               *gtime.Time `json:"settled_at"                orm:"settled_at"                description:"结算时间"`                                                      // 结算时间
	CreatedAt               *gtime.Time `json:"created_at"                orm:"created_at"                description:"创建时间"`                                                      // 创建时间
	UpdatedAt               *gtime.Time `json:"updated_at"                orm:"updated_at"                description:"更新时间"`                                                      // 更新时间
	BillingMode             string      `json:"billing_mode"              orm:"billing_mode"              description:"实际计费模式"`                                                    // 实际计费模式
	EffectiveInputPrice     float64     `json:"effective_input_price"     orm:"effective_input_price"     description:"实际生效的输入单价（快照）"`                                             // 实际生效的输入单价（快照）
	EffectiveOutputPrice    float64     `json:"effective_output_price"    orm:"effective_output_price"    description:"实际生效的输出单价（快照）"`                                             // 实际生效的输出单价（快照）
	DiscountRatio           float64     `json:"discount_ratio"            orm:"discount_ratio"            description:"实际折扣比例（快照）"`                                                // 实际折扣比例（快照）
	BillingInputMultiplier  float64     `json:"billing_input_multiplier"  orm:"billing_input_multiplier"  description:"梯度定价输入乘数（快照）"`                                              // 梯度定价输入乘数（快照）
	BillingOutputMultiplier float64     `json:"billing_output_multiplier" orm:"billing_output_multiplier" description:"梯度定价输出乘数（快照）"`                                              // 梯度定价输出乘数（快照）
	CacheCreationTokens     int         `json:"cache_creation_tokens"     orm:"cache_creation_tokens"     description:"缓存创建 token 数"`                                              // 缓存创建 token 数
	CacheReadTokens         int         `json:"cache_read_tokens"         orm:"cache_read_tokens"         description:"缓存读取 token 数"`                                              // 缓存读取 token 数
	CacheCreationCost       float64     `json:"cache_creation_cost"       orm:"cache_creation_cost"       description:"缓存创建费用"`                                                    // 缓存创建费用
	CacheReadCost           float64     `json:"cache_read_cost"           orm:"cache_read_cost"           description:"缓存读取费用"`                                                    // 缓存读取费用
	ModelMultiplier         float64     `json:"model_multiplier"          orm:"model_multiplier"          description:"模型倍率（快照）"`                                                  // 模型倍率（快照）
	TenantMultiplier        float64     `json:"tenant_multiplier"         orm:"tenant_multiplier"         description:"租户倍率（快照）"`                                                  // 租户倍率（快照）
	BaseInputPrice          float64     `json:"base_input_price"          orm:"base_input_price"          description:"基础模型输入单价（快照，应用倍率前）"`                                        // 基础模型输入单价（快照，应用倍率前）
	BaseOutputPrice         float64     `json:"base_output_price"         orm:"base_output_price"         description:"基础模型输出单价（快照，应用倍率前）"`                                        // 基础模型输出单价（快照，应用倍率前）
	BillingSnapshot         string      `json:"billing_snapshot"          orm:"billing_snapshot"          description:"完整计费计算过程快照（JSONB）"`                                         // 完整计费计算过程快照（JSONB）
	TenantPlanId            int64       `json:"tenant_plan_id"            orm:"tenant_plan_id"            description:""`                                                          //
	PlanDeduction           float64     `json:"plan_deduction"            orm:"plan_deduction"            description:""`                                                          //
	WalletDeduction         float64     `json:"wallet_deduction"          orm:"wallet_deduction"          description:""`                                                          //
}
