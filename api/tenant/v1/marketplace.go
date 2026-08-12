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
	ModelId          string   `json:"model_id"`           // 模型标识
	ModelName        string   `json:"model_name"`         // 模型显示名称
	Category         string   `json:"category"`           // 分类（chat/embedding/image等）
	Description      string   `json:"description"`        // 描述
	MaxContextTokens int      `json:"max_context_tokens"` // 最大上下文 tokens
	MaxOutputTokens  int      `json:"max_output_tokens"`  // 最大输出 tokens
	InputPrice       float64  `json:"input_price"`        // 输入价格（每百万 token，USD）
	OutputPrice      float64  `json:"output_price"`       // 输出价格（每百万 token，USD）
	Tags             []string `json:"tags"`               // 标签
	Capabilities     g.Map    `json:"capabilities"`       // 能力标签
}

type MarketplaceDetailReq struct {
	g.Meta  `path:"/marketplace/models/{model_id}" method:"get" tags:"公开-模型广场" summary:"获取模型详情" group:"public" middleware:"-"`
	ModelId string `json:"model_id" in:"path" v:"required" dc:"模型标识"`
}

type MarketplaceDetailRes struct {
	MarketplaceModelItem
}
