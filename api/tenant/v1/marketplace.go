package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

type MarketplaceListReq struct {
	g.Meta   `path:"/marketplace/models" method:"get" tags:"公开-模型广场" summary:"获取模型广场列表" group:"public" middleware:"-"`
	Keyword  string `json:"keyword" dc:"搜索关键词"`
	Category string `json:"category" dc:"模型类别筛选（chat/embedding/image/audio等）"`
	Page     int    `json:"page" d:"1" dc:"页码"`
	PageSize int    `json:"page_size" d:"20" v:"between:1,100" dc:"每页数量"`
}

type MarketplaceListRes struct {
	List     []MarketplaceModelItem `json:"list"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

type MarketplaceModelItem struct {
	ModelId            string          `json:"model_id"`             // 模型标识
	ModelName          string          `json:"model_name"`           // 模型显示名称
	Category           string          `json:"category"`             // 分类（chat/embedding/image等）
	Description        string          `json:"description"`          // 描述
	MaxContextTokens   int             `json:"max_context_tokens"`   // 最大上下文 tokens
	MaxOutputTokens    int             `json:"max_output_tokens"`    // 最大输出 tokens
	BillingMode        string          `json:"billing_mode"`         // 计费模式：token/per_request/tiered
	InputPrice         float64         `json:"input_price"`          // 输入价格（每百万 token，USD；tiered=首档）
	OutputPrice        float64         `json:"output_price"`         // 输出价格（每百万 token，USD；tiered=首档）
	PerRequestPrice    float64         `json:"per_request_price"`    // 按次单价（USD/次，仅 per_request）
	CacheReadPrice     float64         `json:"cache_read_price"`     // 缓存读取价格（每百万 token，USD）
	CacheCreationPrice float64         `json:"cache_creation_price"` // 缓存创建价格（每百万 token，USD）
	DiscountLabel      string          `json:"discount_label"`       // 折扣标签（营销展示，空=不展示）
	PriceChangeNote    string          `json:"price_change_note"`    // 价格调整说明（对外提示，空=不展示）
	TimePrices         []TimePriceItem `json:"time_prices"`          // 时段价目（平台基础价 × 时段乘数，无配置=空）
	Tags               []string        `json:"tags"`                 // 标签
	Capabilities       g.Map           `json:"capabilities"`         // 能力标签
}

type MarketplaceDetailReq struct {
	g.Meta  `path:"/marketplace/models/{model_id}" method:"get" tags:"公开-模型广场" summary:"获取模型详情" group:"public" middleware:"-"`
	ModelId string `json:"model_id" in:"path" v:"required" dc:"模型标识"`
}

type MarketplaceDetailRes struct {
	MarketplaceModelItem
}
