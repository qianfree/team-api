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
	ID             int64    `json:"id"`
	TenantID       int64    `json:"tenant_id"`
	ModelID        int64    `json:"model_id"`
	ModelCode      string   `json:"model_code"`
	ModelName      string   `json:"model_name"`
	Category       string   `json:"category"`
	Enabled        bool     `json:"enabled"`
	DiscountRatio  *float64 `json:"discount_ratio"`
	MaxConcurrency *int     `json:"max_concurrency"`
	ChannelScope   string   `json:"channel_scope"`
	Multiplier     float64  `json:"multiplier"`
	// Pricing 定价覆盖（custom_pricing blob 回显，PricingBlob 形状子集；
	// nil/无键 = 继承平台）。与 DiscountRatio 互斥（专属价一口价不再打折）
	Pricing *TenantPricingOverride `json:"pricing"`
	// 展示字段覆盖：nil=继承平台
	PriceNote       *string `json:"price_note"`
	DiscountLabel   *string `json:"discount_label"`
	PriceChangeNote *string `json:"price_change_note"`
}

// TenantPricingOverride 租户定价覆盖对象（PricingBlob 形状的 API 子集，
// 与平台 mdl_pricing.pricing 共用 schema；全量替换语义：提交整个对象，
// 键缺失=继承平台，「恢复继承」=提交时剔除该键）。
// 与 discount_ratio 互斥（写侧校验拒绝并存）
type TenantPricingOverride struct {
	BillingMode          *string                `json:"billing_mode" dc:"覆盖计费模式（token/tiered/per_request）；平台 formula/special 模式下读侧忽略"`
	InputPrice           *float64               `json:"input_price" dc:"输入价覆盖"`
	OutputPrice          *float64               `json:"output_price" dc:"输出价覆盖"`
	CacheReadPrice       *float64               `json:"cache_read_price" dc:"缓存读价覆盖"`
	CacheCreationPrice   *float64               `json:"cache_creation_price" dc:"缓存创建/写入价覆盖（短 TTL 桶）"`
	CacheCreation1hPrice *float64               `json:"cache_creation_1h_price" dc:"长时缓存创建价覆盖（长 TTL 桶：Claude 1h / GPT 12h）"`
	Price                *float64               `json:"price" dc:"按次单价覆盖"`
	Tiers                []*PricingTier         `json:"tiers" dc:"阶梯整体替换；空=继承平台阶梯"`
	Prices               *map[string]float64    `json:"prices" dc:"按秒矩阵整体替换（仅平台 per_second/special 模型）；nil=继承"`
	TimeSegments         *[]TimeSegmentItem     `json:"time_segments" dc:"时段定价：null=继承；[]=显式关闭平台配置；非空=整体替换"`
	ParamMultipliers     *[]ParamMultiplierItem `json:"param_multipliers" dc:"参数倍率：三态同 time_segments"`
}

// PricingTier 阶梯定价项（租户自定义阶梯时使用）
type PricingTier struct {
	MinTokens            int64   `json:"min_tokens"`
	MaxTokens            *int64  `json:"max_tokens"`
	InputPrice           float64 `json:"input_price"`
	OutputPrice          float64 `json:"output_price"`
	CacheReadPrice       float64 `json:"cache_read_price"`
	CacheCreationPrice   float64 `json:"cache_creation_price"`
	CacheCreation1hPrice float64 `json:"cache_creation_1h_price"` // 长 TTL 桶价（Claude 1h / GPT 12h）；0=未配置回退锚点
}

// TenantModelBatchAssignReq 批量分配模型给租户
type TenantModelBatchAssignReq struct {
	g.Meta      `path:"/tenants/{tenant_id}/models" method:"post" mime:"json" tags:"管理后台-租户模型" summary:"批量分配模型"`
	TenantID    int64             `json:"tenant_id" in:"path" v:"required" dc:"租户ID"`
	Assignments []ModelAssignment `json:"assignments" v:"required" dc:"模型分配列表"`
}

type ModelAssignment struct {
	ModelID        int64                  `json:"model_id" v:"required" dc:"模型ID"`
	Enabled        bool                   `json:"enabled" d:"true" dc:"是否启用"`
	DiscountRatio  *float64               `json:"discount_ratio" dc:"折扣比例(0-1)，与 Pricing 价格键互斥"`
	MaxConcurrency *int                   `json:"max_concurrency" dc:"单模型并发上限"`
	ChannelScope   *string                `json:"channel_scope" dc:"渠道范围JSON数组"`
	Pricing        *TenantPricingOverride `json:"pricing" dc:"定价覆盖对象（全量写入 custom_pricing）"`
}

type TenantModelBatchAssignRes struct {
	Assigned int `json:"assigned"`
}

// TenantModelUpdateReq 更新租户模型分配配置。
// 扩展计费覆盖与展示字段用双层指针区分四态：省略=不动；null=清除恢复继承；
// 空数组=显式关闭平台配置（时段/参数倍率）；非空=整体覆盖
// TenantModelUpdateReq 更新租户模型配置。
// 定价覆盖走 Pricing 对象全量替换（PricingBlob 形状子集，键缺失=继承平台，
// 恢复继承=剔除该键提交）；与折扣互斥（写侧校验）。省略 Pricing=不动定价覆盖，
// 提交空对象 {}=清除全部覆盖恢复继承。展示字段仍用双层指针四态
type TenantModelUpdateReq struct {
	g.Meta         `path:"/tenants/{tenant_id}/models/{model_id}" method:"put" mime:"json" tags:"管理后台-租户模型" summary:"更新租户模型配置"`
	TenantID       int64                  `json:"tenant_id" in:"path" v:"required" dc:"租户ID"`
	ModelID        int64                  `json:"model_id" in:"path" v:"required" dc:"模型ID"`
	Enabled        *bool                  `json:"enabled" dc:"是否启用"`
	DiscountRatio  **float64              `json:"discount_ratio" dc:"折扣比例：省略=不动；null=清除（与 Pricing 价格键互斥）"`
	MaxConcurrency **int                  `json:"max_concurrency" dc:"单模型并发上限"`
	ChannelScope   **string               `json:"channel_scope" dc:"渠道范围JSON数组"`
	Pricing        *TenantPricingOverride `json:"pricing" dc:"定价覆盖（全量替换 custom_pricing）：省略=不动；空对象=清除恢复继承"`
	// 展示字段覆盖
	PriceNote       **string `json:"price_note" dc:"价格说明覆盖（内部）：省略=不动；null=清除恢复继承"`
	DiscountLabel   **string `json:"discount_label" dc:"折扣标签覆盖（对外）：省略=不动；null=清除恢复继承"`
	PriceChangeNote **string `json:"price_change_note" dc:"调价说明覆盖（对外）：省略=不动；null=清除恢复继承"`
}

type TenantModelUpdateRes struct{}

// TenantModelDeleteReq 移除租户模型分配
type TenantModelDeleteReq struct {
	g.Meta   `path:"/tenants/{tenant_id}/models/{model_id}" method:"delete" mime:"json" tags:"管理后台-租户模型" summary:"移除租户模型"`
	TenantID int64 `json:"tenant_id" in:"path" v:"required" dc:"租户ID"`
	ModelID  int64 `json:"model_id" in:"path" v:"required" dc:"模型ID"`
}

type TenantModelDeleteRes struct{}

// TenantAvailableModelsPreviewReq 预览租户实际可用的所有模型（显式分配 + 分组来源，去重）
type TenantAvailableModelsPreviewReq struct {
	g.Meta   `path:"/tenants/{tenant_id}/available-models" method:"get" mime:"json" tags:"管理后台-租户模型" summary:"预览租户可用模型"`
	TenantID int64 `json:"tenant_id" in:"path" v:"required" dc:"租户ID"`
}

type TenantAvailableModelsPreviewRes struct {
	List []TenantAvailableModelPreview `json:"list"`
}

// TenantAvailableModelPreview 租户可用模型预览项
type TenantAvailableModelPreview struct {
	ModelID          string `json:"model_id"`
	ModelName        string `json:"model_name"`
	Category         string `json:"category"`
	MaxContextTokens int    `json:"max_context_tokens"`
	MaxOutputTokens  int    `json:"max_output_tokens"`
	Description      string `json:"description"`
	Source           string `json:"source"` // "explicit" 或 "group"
}
