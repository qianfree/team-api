package v1

import "github.com/gogf/gf/v2/frame/g"

// ModelListReq 模型列表请求
type ModelListReq struct {
	g.Meta   `path:"/models" method:"get" mime:"json" tags:"管理后台-模型" summary:"模型列表"`
	Page     int    `json:"page" d:"1" v:"min:1" dc:"页码"`
	PageSize int    `json:"page_size" d:"20" v:"min:1|max:100" dc:"每页数量"`
	Category string `json:"category" dc:"模型分类筛选：chat/embedding/image/audio/rerank"`
	Status   string `json:"status" dc:"状态筛选：active/deprecated/offline"`
	Search   string `json:"search" dc:"搜索关键词（模型名或显示名）"`
}

// ModelListRes 模型列表响应
type ModelListRes struct {
	List     []ModelItem `json:"list"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// ModelItem 模型信息
type ModelItem struct {
	ID               int64           `json:"id"`
	ModelId          string          `json:"model_id"`
	ModelName        string          `json:"model_name"`
	Category         string          `json:"category"`
	Status           string          `json:"status"`
	MaxContext       int             `json:"max_context_tokens"`
	MaxOutput        int             `json:"max_output_tokens"`
	Capabilities     map[string]bool `json:"capabilities"`
	Description      string          `json:"description"`
	Tags             []string        `json:"tags"`
	CreatedAt        string          `json:"created_at"`
	UpdatedAt        string          `json:"updated_at"`
	DeprecatedAt     *string         `json:"deprecated_at"`
	SunsetDate       *string         `json:"sunset_date"`
	ReplacementModel string          `json:"replacement_model"`
}

// ModelCreateReq 创建模型请求
type ModelCreateReq struct {
	g.Meta       `path:"/models" method:"post" mime:"json" tags:"管理后台-模型" summary:"创建模型"`
	ModelId      string          `json:"model_id" v:"required|length:1,100#请输入模型标识|模型标识长度1-100" dc:"模型唯一标识"`
	ModelName    string          `json:"model_name" dc:"模型显示名称"`
	Category     string          `json:"category" v:"required|in:chat,embedding,image,audio,rerank#请选择分类|分类必须是 chat/embedding/image/audio/rerank" dc:"模型分类"`
	MaxContext   int             `json:"max_context_tokens" dc:"最大上下文 token 数"`
	MaxOutput    int             `json:"max_output_tokens" dc:"最大输出 token 数"`
	Capabilities map[string]bool `json:"capabilities" dc:"模型能力特性"`
	Description  string          `json:"description" dc:"模型描述"`
	Tags         []string        `json:"tags" dc:"标签列表"`
}

// ModelCreateRes 创建模型响应
type ModelCreateRes struct {
	ID int64 `json:"id"`
}

// ModelUpdateReq 更新模型请求
type ModelUpdateReq struct {
	g.Meta           `path:"/models/{id}" method:"put" mime:"json" tags:"管理后台-模型" summary:"更新模型"`
	ID               int64           `json:"id" in:"path" v:"required" dc:"模型ID"`
	ModelName        string          `json:"model_name" dc:"模型显示名称"`
	Category         string          `json:"category" v:"in:chat,embedding,image,audio,rerank" dc:"模型分类"`
	MaxContext       int             `json:"max_context_tokens" dc:"最大上下文 token 数"`
	MaxOutput        int             `json:"max_output_tokens" dc:"最大输出 token 数"`
	Capabilities     map[string]bool `json:"capabilities" dc:"模型能力特性"`
	Description      string          `json:"description" dc:"模型描述"`
	Tags             []string        `json:"tags" dc:"标签列表"`
	Status           string          `json:"status" v:"in:active,deprecated,offline" dc:"状态"`
	SunsetDate       *string         `json:"sunset_date" dc:"下线日期（格式：YYYY-MM-DD，仅 deprecated 状态有效）"`
	ReplacementModel string          `json:"replacement_model" dc:"推荐替代模型名"`
}

// ModelDeleteReq 删除模型请求
type ModelDeleteReq struct {
	g.Meta `path:"/models/{id}" method:"delete" mime:"json" tags:"管理后台-模型" summary:"删除模型"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"模型ID"`
}

// PricingItem 定价项（支持按次/按量/阶梯）
type PricingItem struct {
	BillingMode        string   `json:"billing_mode" v:"required|in:token,per_request,tiered" dc:"计费模式"`
	MinTokens          int64    `json:"min_tokens" dc:"阶梯起始 token 数"`
	MaxTokens          *int64   `json:"max_tokens" dc:"阶梯结束 token 数（NULL=无上限）"`
	InputPrice         float64  `json:"input_price" dc:"每 1M input token 价格"`
	OutputPrice        float64  `json:"output_price" dc:"每 1M output token 价格"`
	PerRequestPrice    *float64 `json:"per_request_price" dc:"按次单价（仅 per_request）"`
	CacheReadPrice     float64  `json:"cache_read_price" dc:"缓存读取每 1M token 价格"`
	CacheCreationPrice float64  `json:"cache_creation_price" dc:"缓存创建每 1M token 价格"`
}

// PricingListReq 定价列表请求（模型定价页面专用）
type PricingListReq struct {
	g.Meta   `path:"/models/pricing" method:"get" mime:"json" tags:"管理后台-模型" summary:"模型定价列表"`
	Page     int    `json:"page" d:"1" v:"min:1" dc:"页码"`
	PageSize int    `json:"page_size" d:"20" v:"min:1|max:100" dc:"每页数量"`
	Category string `json:"category" dc:"模型分类筛选"`
	Search   string `json:"search" dc:"搜索关键词（模型名或显示名）"`
}

// PricingListRes 定价列表响应
type PricingListRes struct {
	List     []PricingListItem `json:"list"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// PricingListItem 定价列表项（模型基础信息 + 定价摘要）
type PricingListItem struct {
	ID              int64   `json:"id"`
	ModelId         string  `json:"model_id"`
	ModelName       string  `json:"model_name"`
	Category        string  `json:"category"`
	PricingMode     string  `json:"pricing_mode"`
	InputPrice      float64 `json:"input_price"`
	OutputPrice     float64 `json:"output_price"`
	PerRequestPrice float64 `json:"per_request_price"`
}

// PricingGetReq 获取模型定价
type PricingGetReq struct {
	g.Meta  `path:"/models/{model_id}/pricing" method:"get" mime:"json" tags:"管理后台-模型" summary:"获取模型定价"`
	ModelID int64 `json:"model_id" in:"path" v:"required" dc:"模型ID"`
}

type PricingGetRes struct {
	List []PricingItem `json:"list"`
}

// PricingSetReq 设置模型定价（全量替换）
type PricingSetReq struct {
	g.Meta  `path:"/models/{model_id}/pricing" method:"put" mime:"json" tags:"管理后台-模型" summary:"设置模型定价"`
	ModelID int64         `json:"model_id" in:"path" v:"required" dc:"模型ID"`
	Items   []PricingItem `json:"items" v:"required" dc:"定价列表"`
}

type PricingSetRes struct{}

// ModelUpdateRes 更新模型响应
type ModelUpdateRes struct{}

// ModelDeleteRes 删除模型响应
type ModelDeleteRes struct{}

// ModelExportReq 导出模型列表请求
type ModelExportReq struct {
	g.Meta   `path:"/models/export" method:"get" mime:"json" tags:"管理后台-模型" summary:"导出模型列表"`
	Format   string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	Category string `json:"category" in:"query" dc:"模型分类筛选：chat/embedding/image/audio/rerank"`
	Status   string `json:"status" in:"query" dc:"状态筛选：active/deprecated/offline"`
	Search   string `json:"search" in:"query" dc:"搜索关键词（模型名或显示名）"`
}

type ModelExportRes struct{}

// PricingFetchOfficialReq 拉取模型官方定价
type PricingFetchOfficialReq struct {
	g.Meta  `path:"/models/{model_id}/official-pricing" method:"get" mime:"json" tags:"管理后台-模型" summary:"拉取模型官方定价"`
	ModelID int64 `json:"model_id" in:"path" v:"required" dc:"模型ID"`
}

// PricingFetchOfficialRes 拉取官方定价响应
type PricingFetchOfficialRes struct {
	ModelName string                   `json:"model_name"` // 数据库中的模型名
	Sources   []*OfficialPricingSource `json:"sources"`    // 多来源定价数据
}

// OfficialPricingSource 单个来源的官方定价数据
type OfficialPricingSource struct {
	Source     string               `json:"source"`                       // 数据来源（"litellm" / "models.dev"）
	Found      bool                 `json:"found"`                        // 是否找到
	Error      string               `json:"error,omitempty"`              // 获取远程数据失败时的错误信息
	Provider   string               `json:"provider,omitempty"`           // 供应商（anthropic/openai/...）
	Mode       string               `json:"mode,omitempty"`               // 模型类型（chat/embedding/...）
	MaxContext int                  `json:"max_context_tokens,omitempty"` // 最大上下文
	MaxOutput  int                  `json:"max_output_tokens,omitempty"`  // 最大输出
	Pricing    *OfficialPricingItem `json:"pricing,omitempty"`            // 定价信息
}

// OfficialPricingItem 官方定价项
type OfficialPricingItem struct {
	InputPrice         float64 `json:"input_price"`          // $/1M tokens
	OutputPrice        float64 `json:"output_price"`         // $/1M tokens
	CacheReadPrice     float64 `json:"cache_read_price"`     // $/1M tokens
	CacheCreationPrice float64 `json:"cache_creation_price"` // $/1M tokens
	BillingMode        string  `json:"billing_mode"`         // 建议计费模式
}

// ModelFetchOfficialInfoReq 按名称拉取官方模型信息（上下文长度+能力特性）
type ModelFetchOfficialInfoReq struct {
	g.Meta    `path:"/models/official-info" method:"get" mime:"json" tags:"管理后台-模型" summary:"拉取官方模型信息"`
	ModelName string `json:"model_name" in:"query" v:"required" dc:"模型名称"`
}

// ModelFetchOfficialInfoRes 官方模型信息响应
type ModelFetchOfficialInfoRes struct {
	Found            bool            `json:"found"`
	Error            string          `json:"error,omitempty"` // 获取远程数据失败时的错误信息
	Provider         string          `json:"provider,omitempty"`
	MaxContextTokens int             `json:"max_context_tokens,omitempty"`
	MaxOutputTokens  int             `json:"max_output_tokens,omitempty"`
	Capabilities     map[string]bool `json:"capabilities,omitempty"`
}
