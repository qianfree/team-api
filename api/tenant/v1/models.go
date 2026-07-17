package v1

import "github.com/gogf/gf/v2/frame/g"

// TenantAvailableModelsReq 租户可用模型列表请求
type TenantAvailableModelsReq struct {
	g.Meta   `path:"/models" method:"get" mime:"json" tags:"租户控制台-模型" summary:"租户可用模型列表"`
	Category string `json:"category" dc:"模型分类筛选：chat/embedding/image/audio/rerank"`
	Search   string `json:"search" dc:"搜索关键词（模型名或显示名）"`
}

type TenantAvailableModelsRes struct {
	List []TenantAvailableModelItem `json:"list"`
}

// PricingTierItem 阶梯定价项
type PricingTierItem struct {
	MinTokens   int64   `json:"min_tokens"`
	MaxTokens   *int64  `json:"max_tokens"`
	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`
}

// TenantAvailableModelItem 租户可用模型信息
type TenantAvailableModelItem struct {
	ID                 int64             `json:"id"`
	ModelId            string            `json:"model_id"`
	ModelName          string            `json:"model_name"`
	Category           string            `json:"category"`
	MaxContext         int               `json:"max_context_tokens"`
	MaxOutput          int               `json:"max_output_tokens"`
	Description        string            `json:"description"`
	Tags               string            `json:"tags"`
	Capabilities       string            `json:"capabilities"`
	BillingMode        *string           `json:"billing_mode"`
	PerRequestPrice    *float64          `json:"per_request_price"`
	DiscountRatio      *float64          `json:"discount_ratio"`
	MaxConcurrency     *int              `json:"max_concurrency"`
	InputPrice         *float64          `json:"input_price"`
	OutputPrice        *float64          `json:"output_price"`
	CacheReadPrice     *float64          `json:"cache_read_price"`
	CacheCreationPrice *float64          `json:"cache_creation_price"`
	PricingTiers       []PricingTierItem `json:"pricing_tiers"`
	// AsyncImage 图片模型是否必须走异步端点（提交 /v1/images/generations/async 后轮询取图）。
	// 仅图片分类模型有意义；DashScope 等异步图片 provider 为 true。
	AsyncImage bool `json:"async_image"`
}
