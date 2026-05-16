package v1

import "github.com/gogf/gf/v2/frame/g"

// TenantModelListReq 租户模型分配列表
type TenantModelListReq struct {
	g.Meta   `path:"/tenants/{tenant_id}/models" method:"get" mime:"json" tags:"管理后台-租户模型" summary:"租户模型分配列表"`
	TenantID int64 `json:"tenant_id" in:"path" v:"required" dc:"租户ID"`
}

type TenantModelListRes struct {
	List []TenantModelItem `json:"list"`
}

type TenantModelItem struct {
	ID                       int64          `json:"id"`
	TenantID                 int64          `json:"tenant_id"`
	ModelID                  int64          `json:"model_id"`
	ModelCode                string         `json:"model_code"`
	ModelName                string         `json:"model_name"`
	Category                 string         `json:"category"`
	Enabled                  bool           `json:"enabled"`
	BillingMode              *string        `json:"billing_mode"`
	PerRequestPrice          *float64       `json:"per_request_price"`
	DiscountRatio            *float64       `json:"discount_ratio"`
	MaxConcurrency           *int           `json:"max_concurrency"`
	ChannelScope             string         `json:"channel_scope"`
	CustomInputPrice         *float64       `json:"custom_input_price"`
	CustomOutputPrice        *float64       `json:"custom_output_price"`
	CustomCacheReadPrice     *float64       `json:"custom_cache_read_price"`
	CustomCacheCreationPrice *float64       `json:"custom_cache_creation_price"`
	CustomPricingTiers       []*PricingTier `json:"custom_pricing_tiers"`
	Multiplier               float64        `json:"multiplier"`
}

// PricingTier 阶梯定价项（租户自定义阶梯时使用）
type PricingTier struct {
	MinTokens          int64   `json:"min_tokens"`
	MaxTokens          *int64  `json:"max_tokens"`
	InputPrice         float64 `json:"input_price"`
	OutputPrice        float64 `json:"output_price"`
	CacheReadPrice     float64 `json:"cache_read_price"`
	CacheCreationPrice float64 `json:"cache_creation_price"`
}

// TenantModelBatchAssignReq 批量分配模型给租户
type TenantModelBatchAssignReq struct {
	g.Meta      `path:"/tenants/{tenant_id}/models" method:"post" mime:"json" tags:"管理后台-租户模型" summary:"批量分配模型"`
	TenantID    int64             `json:"tenant_id" in:"path" v:"required" dc:"租户ID"`
	Assignments []ModelAssignment `json:"assignments" v:"required" dc:"模型分配列表"`
}

type ModelAssignment struct {
	ModelID                  int64          `json:"model_id" v:"required" dc:"模型ID"`
	Enabled                  bool           `json:"enabled" d:"true" dc:"是否启用"`
	BillingMode              *string        `json:"billing_mode" dc:"覆盖计费模式"`
	PerRequestPrice          *float64       `json:"per_request_price" dc:"按次单价"`
	DiscountRatio            *float64       `json:"discount_ratio" dc:"折扣比例(0-1)"`
	MaxConcurrency           *int           `json:"max_concurrency" dc:"单模型并发上限"`
	ChannelScope             *string        `json:"channel_scope" dc:"渠道范围JSON数组"`
	CustomInputPrice         *float64       `json:"custom_input_price" dc:"自定义输入价格"`
	CustomOutputPrice        *float64       `json:"custom_output_price" dc:"自定义输出价格"`
	CustomCacheReadPrice     *float64       `json:"custom_cache_read_price" dc:"自定义缓存读取价格"`
	CustomCacheCreationPrice *float64       `json:"custom_cache_creation_price" dc:"自定义缓存创建价格"`
	CustomPricingTiers       []*PricingTier `json:"custom_pricing_tiers" dc:"自定义阶梯定价"`
}

type TenantModelBatchAssignRes struct {
	Assigned int `json:"assigned"`
}

// TenantModelUpdateReq 更新租户模型分配配置
type TenantModelUpdateReq struct {
	g.Meta                   `path:"/tenants/{tenant_id}/models/{model_id}" method:"put" mime:"json" tags:"管理后台-租户模型" summary:"更新租户模型配置"`
	TenantID                 int64           `json:"tenant_id" in:"path" v:"required" dc:"租户ID"`
	ModelID                  int64           `json:"model_id" in:"path" v:"required" dc:"模型ID"`
	Enabled                  *bool           `json:"enabled" dc:"是否启用"`
	BillingMode              *string         `json:"billing_mode" v:"in:token,per_request,tiered" dc:"计费模式"`
	PerRequestPrice          **float64       `json:"per_request_price" dc:"按次单价"`
	DiscountRatio            **float64       `json:"discount_ratio" dc:"折扣比例"`
	MaxConcurrency           **int           `json:"max_concurrency" dc:"单模型并发上限"`
	ChannelScope             **string        `json:"channel_scope" dc:"渠道范围JSON数组"`
	CustomInputPrice         **float64       `json:"custom_input_price" dc:"自定义输入价格"`
	CustomOutputPrice        **float64       `json:"custom_output_price" dc:"自定义输出价格"`
	CustomCacheReadPrice     **float64       `json:"custom_cache_read_price" dc:"自定义缓存读取价格"`
	CustomCacheCreationPrice **float64       `json:"custom_cache_creation_price" dc:"自定义缓存创建价格"`
	CustomPricingTiers       *[]*PricingTier `json:"custom_pricing_tiers" dc:"自定义阶梯定价"`
}

type TenantModelUpdateRes struct{}

// TenantModelDeleteReq 移除租户模型分配
type TenantModelDeleteReq struct {
	g.Meta   `path:"/tenants/{tenant_id}/models/{model_id}" method:"delete" mime:"json" tags:"管理后台-租户模型" summary:"移除租户模型"`
	TenantID int64 `json:"tenant_id" in:"path" v:"required" dc:"租户ID"`
	ModelID  int64 `json:"model_id" in:"path" v:"required" dc:"模型ID"`
}

type TenantModelDeleteRes struct{}
