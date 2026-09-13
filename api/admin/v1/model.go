package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

// ModelListReq 模型列表请求
type ModelListReq struct {
	g.Meta        `path:"/models" method:"get" mime:"json" tags:"管理后台-模型" summary:"模型列表"`
	Page          int    `json:"page" d:"1" v:"min:1" dc:"页码"`
	PageSize      int    `json:"page_size" d:"20" v:"min:1|max:100" dc:"每页数量"`
	Category      string `json:"category" dc:"模型分类筛选：chat/embedding/image/audio/rerank/video"`
	Vendor        string `json:"vendor" dc:"研发厂商筛选：openai/anthropic/google/alibaba/deepseek等，空=不过滤"`
	Status        string `json:"status" dc:"状态筛选：active/deprecated/offline"`
	Search        string `json:"search" dc:"搜索关键词（模型名或显示名）"`
	PricingStatus string `json:"pricing_status" dc:"定价状态筛选：priced/unpriced"`
}

// ModelListRes 模型列表响应
type ModelListRes struct {
	List     []ModelItem `json:"list"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// ModelChannelInfo 模型关联的渠道信息
type ModelChannelInfo struct {
	ChannelID   int64  `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Type        int    `json:"type"`
}

// ModelItem 模型信息
type ModelItem struct {
	ID               int64           `json:"id"`
	ModelId          string          `json:"model_id"`
	ModelName        string          `json:"model_name"`
	Category         string          `json:"category"`
	Vendor           string          `json:"vendor"` // 研发厂商（空=未分类）
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
	// 定价摘要（来自 mdl_pricing 的 pricing JSONB）
	PricingMode     string             `json:"pricing_mode"`                // "" | "token" | "per_request" | "tiered" | "per_second"
	InputPrice      float64            `json:"input_price"`                 // $/1M tokens
	PerSecondPrices map[string]float64 `json:"per_second_prices,omitempty"` // 规格→$/秒（仅 per_second 模式）
	OutputPrice     float64            `json:"output_price"`                // $/1M tokens
	PerRequestPrice float64            `json:"per_request_price"`           // $/request（按次计费模式）
	// 可用渠道列表（来自 chn_abilities + chn_channels）
	Channels []ModelChannelInfo `json:"channels"`
}

// ModelCreateReq 创建模型请求
type ModelCreateReq struct {
	g.Meta       `path:"/models" method:"post" mime:"json" tags:"管理后台-模型" summary:"创建模型"`
	ModelId      string          `json:"model_id" v:"required|length:1,100#请输入模型标识|模型标识长度1-100" dc:"模型唯一标识"`
	ModelName    string          `json:"model_name" dc:"模型显示名称"`
	Category     string          `json:"category" v:"required|in:chat,embedding,image,audio,rerank,video#请选择分类|分类必须是 chat/embedding/image/audio/rerank/video" dc:"模型分类"`
	Vendor       string          `json:"vendor" v:"in:openai,anthropic,google,xai,mistral,cohere,meta,alibaba,bytedance,deepseek,zhipu,moonshot,minimax,baidu,tencent,xunfei,kuaishou,midjourney,suno#厂商值不合法" dc:"研发厂商（空=未分类）"`
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
	g.Meta    `path:"/models/{id}" method:"put" mime:"json" tags:"管理后台-模型" summary:"更新模型"`
	ID        int64  `json:"id" in:"path" v:"required" dc:"模型ID"`
	ModelName string `json:"model_name" dc:"模型显示名称"`
	Category  string `json:"category" v:"in:chat,embedding,image,audio,rerank,video" dc:"模型分类"`
	// Vendor 研发厂商；指针语义：nil=不更新，空串=清空回未分类，非空=设置（枚举校验在 logic 层做）
	Vendor           *string         `json:"vendor" dc:"研发厂商"`
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

// PricingItem 定价项（支持按次/按量/阶梯/按秒）
type PricingItem struct {
	BillingMode        string             `json:"billing_mode" v:"required|in:token,per_request,tiered,per_second" dc:"计费模式"`
	MinTokens          int64              `json:"min_tokens" dc:"阶梯起始 token 数"`
	MaxTokens          *int64             `json:"max_tokens" dc:"阶梯结束 token 数（NULL=无上限）"`
	InputPrice         float64            `json:"input_price" dc:"每 1M input token 价格"`
	OutputPrice        float64            `json:"output_price" dc:"每 1M output token 价格"`
	PerRequestPrice    *float64           `json:"per_request_price" dc:"按次单价（仅 per_request）"`
	CacheReadPrice     float64            `json:"cache_read_price" dc:"缓存读取每 1M token 价格"`
	CacheCreationPrice float64            `json:"cache_creation_price" dc:"缓存创建每 1M token 价格"`
	PerSecondPrices    map[string]float64 `json:"per_second_prices" dc:"按秒单价矩阵（仅 per_second）：分辨率规格→每秒单价（本位币），\"*\"为兜底价，如 {\"480p\":0.25,\"720p\":0.5,\"*\":0.5}"`
}

// TimeSegmentItem 时段定价项（mdl_pricing.pricing JSONB 顶层 time_segments 数组元素）。
// 语义：按数组顺序先命中先生效（促销时段放前面覆盖常驻时段），未命中=默认价（乘数 1.0）；
// 最终费用 = 各项小计 × 租户乘数 × 时段乘数。
type TimeSegmentItem struct {
	Name       string  `json:"name" v:"required" dc:"时段名（写入计费快照）"`
	Days       []int   `json:"days" dc:"适用星期 1=周一..7=周日；空=每天"`
	StartTime  string  `json:"start_time" dc:"每日开始时刻 HH:MM；与 end_time 均空=全天"`
	EndTime    string  `json:"end_time" dc:"每日结束时刻 HH:MM；end<start 表示跨零点（如 22:00~06:00）"`
	ValidFrom  string  `json:"valid_from" dc:"生效起始日期 YYYY-MM-DD（含端点）；空=长期，促销/定时调价用"`
	ValidTo    string  `json:"valid_to" dc:"生效结束日期 YYYY-MM-DD（含端点）"`
	Multiplier float64 `json:"multiplier" v:"required|min:0.0001|max:10" dc:"价格乘数，0.5=半价"`
}

// ParamConditionItem 参数倍率匹配条件（mdl_pricing.pricing JSONB 顶层 param_multipliers 数组元素）。
// 匹配对象为网关归一化后的任务体（{model, prompt, seconds, metadata{...}}），
// 支持数组通配（[*]，任一元素命中即命中），字符串数字自动强转参与比较。
type ParamConditionItem struct {
	Path  string `json:"path" v:"required" dc:"任务体 JSON 路径，如 metadata.image / seconds / metadata.content[*].video_url"`
	Match string `json:"match" v:"required|in:has_value,equals,gt,gte,lt,lte,between" dc:"匹配方式"`
	Value any    `json:"value,omitempty" dc:"比对值：equals/gt/gte/lt/lte 单值；between 为 [min,max]；has_value 不填"`
}

// ParamMultiplierItem 参数倍率规则：conditions 全部满足（隐式与）时倍率计入连乘；规则间连乘。
type ParamMultiplierItem struct {
	Conditions []ParamConditionItem `json:"conditions" v:"required" dc:"条件列表（全部满足才命中）"`
	Multiplier float64              `json:"multiplier" v:"required|min:0.0001|max:100" dc:"命中倍率，1.5=加价50%"`
	Note       string               `json:"note" dc:"命中说明（写入计费上下文供对账解释）"`
}

// PricingGetReq 获取模型定价
type PricingGetReq struct {
	g.Meta  `path:"/models/{model_id}/pricing" method:"get" mime:"json" tags:"管理后台-模型" summary:"获取模型定价"`
	ModelID int64 `json:"model_id" in:"path" v:"required" dc:"模型ID"`
}

type PricingGetRes struct {
	List             []PricingItem         `json:"list"`
	TimeSegments     []TimeSegmentItem     `json:"time_segments" dc:"时段定价列表（pricing JSONB 顶层）"`
	ParamMultipliers []ParamMultiplierItem `json:"param_multipliers" dc:"参数倍率规则（pricing JSONB 顶层，横切所有计费模式）"`
	// 官方参考定价（official_pricing JSONB 还原，结构与计费定价完全一致；OfficialItems nil=未配置）。
	// 非计费依据，管理端按官方价 × 折扣快速定价 / 计算现价几折以此为准
	OfficialItems            []PricingItem         `json:"official_items" dc:"官方参考定价项（tiered 展开为多行；nil=未配置）"`
	OfficialTimeSegments     []TimeSegmentItem     `json:"official_time_segments" dc:"官方参考-时段定价"`
	OfficialParamMultipliers []ParamMultiplierItem `json:"official_param_multipliers" dc:"官方参考-参数倍率规则"`
	// 以下为定价行展示字段
	PriceNote       string `json:"price_note" dc:"价格说明（仅管理后台可见的内部备注）"`
	DiscountLabel   string `json:"discount_label" dc:"折扣标签（对外展示，如 7折起）"`
	PriceChangeNote string `json:"price_change_note" dc:"价格调整说明（对外展示，提示价格有变动）"`
	// 计费方案：空 = 通用引擎（前端渲染内置通用表单）；非空时前端分发到方案专属编辑器
	Scheme string `json:"scheme" dc:"计费方案名（空=通用引擎）"`
	// 方案私有配置（不透明容器）：schema 由方案自定义，通用表单不理解其内容
	SchemeConfig json.RawMessage `json:"scheme_config,omitempty" dc:"计费方案私有配置（随方案编辑器解析）"`
}

// PricingSetReq 设置模型定价（全量替换）
type PricingSetReq struct {
	g.Meta           `path:"/models/{model_id}/pricing" method:"put" mime:"json" tags:"管理后台-模型" summary:"设置模型定价"`
	ModelID          int64                 `json:"model_id" in:"path" v:"required" dc:"模型ID"`
	Items            []PricingItem         `json:"items" v:"required" dc:"定价列表"`
	TimeSegments     []TimeSegmentItem     `json:"time_segments" dc:"时段定价（可选，全量替换；空数组清除时段配置）"`
	ParamMultipliers []ParamMultiplierItem `json:"param_multipliers" dc:"参数倍率规则（可选，全量替换；空数组清除）"`
	// 官方参考定价（可选，结构与计费定价完全一致，支持全部四种计费模式 + 时段/倍率）：
	// OfficialItems nil=本次不动库内官方定价；非 nil=全量替换（三个数组全空=清除）。金额本位币，非计费依据
	OfficialItems            []PricingItem         `json:"official_items" dc:"官方参考定价项（nil=不修改；非 nil=全量替换，与另两个官方数组全空=清除）"`
	OfficialTimeSegments     []TimeSegmentItem     `json:"official_time_segments" dc:"官方参考-时段定价（随 official_items 一并全量替换）"`
	OfficialParamMultipliers []ParamMultiplierItem `json:"official_param_multipliers" dc:"官方参考-参数倍率规则（随 official_items 一并全量替换）"`
	// 以下为定价行展示字段（全量替换语义：空=清除；price_note 不透出到租户端）
	PriceNote       string `json:"price_note" dc:"价格说明（仅内部可见）"`
	DiscountLabel   string `json:"discount_label" dc:"折扣标签（对外展示）"`
	PriceChangeNote string `json:"price_change_note" dc:"价格调整说明（对外展示）"`
	// 计费方案（全量替换语义：空串=回落通用引擎并清除 scheme_config）。
	// 必须是已注册方案（后端校验），自定义方案建议 custom: 前缀命名
	Scheme string `json:"scheme" dc:"计费方案名（空=通用引擎并清除私有配置）"`
	// 方案私有配置：仅 scheme 非空时允许携带，由方案实现的校验规则把关
	SchemeConfig json.RawMessage `json:"scheme_config,omitempty" dc:"计费方案私有配置（方案自定义 schema）"`
}

type PricingSetRes struct{}

// ModelUpdateRes 更新模型响应
type ModelUpdateRes struct{}

// ModelDeleteRes 删除模型响应
type ModelDeleteRes struct{}

// ModelOptionsReq 模型选项列表请求（下拉选择专用，不分页）
type ModelOptionsReq struct {
	g.Meta   `path:"/models/options" method:"get" mime:"json" tags:"管理后台-模型" summary:"模型选项列表（不分页）"`
	Status   string `json:"status" in:"query" dc:"状态筛选：active/deprecated/offline"`
	Category string `json:"category" in:"query" dc:"模型分类筛选：chat/embedding/image/audio/rerank/video"`
}

// ModelOptionsRes 模型选项列表响应
type ModelOptionsRes struct {
	List []ModelOptionItem `json:"list"`
}

// ModelOptionItem 模型选项项（精简字段）
type ModelOptionItem struct {
	ID        int64  `json:"id"`
	ModelId   string `json:"model_id"`
	ModelName string `json:"model_name"`
	Category  string `json:"category"`
}

// ModelExportReq 导出模型列表请求
type ModelExportReq struct {
	g.Meta   `path:"/models/export" method:"get" mime:"json" tags:"管理后台-模型" summary:"导出模型列表"`
	Format   string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	Category string `json:"category" in:"query" dc:"模型分类筛选：chat/embedding/image/audio/rerank"`
	Vendor   string `json:"vendor" in:"query" dc:"研发厂商筛选，空=不过滤"`
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

// ModelExportJsonReq 导出模型配置（JSON，用于跨环境迁移）
type ModelExportJsonReq struct {
	g.Meta   `path:"/models/export-json" method:"post" mime:"json" tags:"管理后台-模型" summary:"导出模型配置JSON"`
	ModelIds []string `json:"model_ids" v:"required#请至少选择一个模型" dc:"要导出的model_id列表"`
}

// ModelExportJsonRes 导出模型配置JSON响应（直接写入响应流）
type ModelExportJsonRes struct{}

// ModelImportPreviewReq 导入模型预览请求
type ModelImportPreviewReq struct {
	g.Meta `path:"/models/import-preview" method:"post" tags:"管理后台-模型" summary:"导入模型预览"`
}

// ModelImportPreviewRes 导入模型预览响应
type ModelImportPreviewRes struct {
	Models []ModelImportPreviewItem `json:"models"`
}

// ModelImportPreviewItem 导入预览项（模型信息 + 冲突标记）
type ModelImportPreviewItem struct {
	ModelId          string            `json:"model_id"`
	ModelName        string            `json:"model_name"`
	Category         string            `json:"category"`
	Vendor           string            `json:"vendor"` // 研发厂商（空=未分类）
	Status           string            `json:"status"`
	MaxContextTokens int               `json:"max_context_tokens"`
	MaxOutputTokens  int               `json:"max_output_tokens"`
	Capabilities     map[string]bool   `json:"capabilities"`
	Description      string            `json:"description"`
	Tags             []string          `json:"tags"`
	SunsetDate       string            `json:"sunset_date"`
	ReplacementModel string            `json:"replacement_model"`
	Pricing          []PricingItem     `json:"pricing"`
	TimeSegments     []TimeSegmentItem `json:"time_segments" dc:"时段定价列表"`
	Conflict         string            `json:"conflict"` // "" 或 "exists"
}

// ModelImportReq 确认导入模型请求
type ModelImportReq struct {
	g.Meta `path:"/models/import" method:"post" mime:"json" tags:"管理后台-模型" summary:"确认导入模型"`
	Models []ModelImportItem `json:"models" v:"required#请至少导入一个模型"`
}

// ModelImportItem 导入模型项（含定价/时段/参数倍率，全量替换语义）
type ModelImportItem struct {
	ModelId          string                `json:"model_id" v:"required" dc:"模型唯一标识"`
	ModelName        string                `json:"model_name" dc:"模型显示名称"`
	Category         string                `json:"category" v:"required|in:chat,embedding,image,audio,rerank,video#请选择分类|分类无效" dc:"模型分类"`
	Vendor           string                `json:"vendor" v:"in:openai,anthropic,google,xai,mistral,cohere,meta,alibaba,bytedance,deepseek,zhipu,moonshot,minimax,baidu,tencent,xunfei,kuaishou,midjourney,suno#厂商值不合法" dc:"研发厂商（空=未分类）"`
	Status           string                `json:"status" dc:"状态"`
	MaxContextTokens int                   `json:"max_context_tokens" dc:"最大上下文 token 数"`
	MaxOutputTokens  int                   `json:"max_output_tokens" dc:"最大输出 token 数"`
	Capabilities     map[string]bool       `json:"capabilities" dc:"模型能力特性"`
	Description      string                `json:"description" dc:"模型描述"`
	Tags             []string              `json:"tags" dc:"标签列表"`
	SunsetDate       string                `json:"sunset_date" dc:"下线日期"`
	ReplacementModel string                `json:"replacement_model" dc:"推荐替代模型名"`
	Pricing          []PricingItem         `json:"pricing" dc:"定价列表"`
	TimeSegments     []TimeSegmentItem     `json:"time_segments" dc:"时段定价列表（可选）"`
	ParamMultipliers []ParamMultiplierItem `json:"param_multipliers" dc:"参数倍率规则（可选）"`
	ConflictAction   string                `json:"conflict_action" v:"in:skip,overwrite" dc:"冲突处理策略：skip/overwrite"`
}

// ModelImportRes 导入结果响应
type ModelImportRes struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}
