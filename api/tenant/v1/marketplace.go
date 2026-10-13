package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

type MarketplaceListReq struct {
	g.Meta       `path:"/marketplace/models" method:"get" tags:"公开-模型广场" summary:"获取模型广场列表" group:"public" middleware:"-"`
	Keyword      string `json:"keyword" dc:"搜索关键词"`
	Category     string `json:"category" dc:"模型类别筛选（chat/embedding/image/audio等），与 categories 多选互斥使用（两者都传时本字段忽略）"`
	Categories   string `json:"categories" dc:"模型类别多选筛选（逗号分隔，如 chat,image），空=不过滤"`
	Vendor       string `json:"vendor" dc:"研发厂商筛选（openai/anthropic/alibaba等），空=不过滤；与 vendors 多选同传时本字段并入多选集合"`
	Vendors      string `json:"vendors" dc:"研发厂商多选筛选（逗号分隔，如 openai,anthropic），空=不过滤"`
	Capabilities string `json:"capabilities" dc:"能力多选筛选（逗号分隔，如 function_calling,vision，AND 语义：需全部具备），空=不过滤"`
	BillingMode  string `json:"billing_mode" dc:"计费模式筛选（token/per_request/tiered/per_second/special），空=不过滤"`
	Sort         string `json:"sort" v:"in:,newest,price_asc,price_desc,context_desc" dc:"排序：空=默认（类别+模型ID）/newest=最新上架/price_asc=价格升序/price_desc=价格降序/context_desc=上下文最长"`
	Page         int    `json:"page" d:"1" dc:"页码"`
	PageSize     int    `json:"page_size" d:"20" v:"between:1,100" dc:"每页数量"`
}

// MarketplaceFacets 析取分面计数：每个维度在「应用其余维度筛选后」的剩余集合内计数，
// 自身维度的已选值不影响本维度计数（勾选 OpenAI 后仍能看到 Anthropic 的真实剩余数），
// 计数为 0 的选项前端禁用置灰，保证点任意可选项都有结果。
type MarketplaceFacets struct {
	Categories   map[string]int `json:"categories"`    // 类别 → 计数（键同 category 枚举）
	Vendors      map[string]int `json:"vendors"`       // 厂商 → 计数（空串厂商归入 "uncategorized"）
	Capabilities map[string]int `json:"capabilities"`  // 能力键 → 计数（键同 capabilities JSONB，仅统计为真的项）
	BillingModes map[string]int `json:"billing_modes"` // 计费模式 → 计数（pricing 行缺失/空模式归入 "token"）
}

type MarketplaceListRes struct {
	List     []MarketplaceModelItem `json:"list"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	// Vendors 当前筛选条件（关键词/分类，忽略厂商筛选本身）下有模型的厂商去重列表，
	// 供前端只渲染实际有模型的厂商筛选项；顺序不保证，前端按本地字典序展示
	Vendors []string `json:"vendors"`
	// Facets 析取分面计数（目录总量超阈值时为空 map，前端隐藏计数只渲染选项）
	Facets MarketplaceFacets `json:"facets"`
}

type MarketplaceModelItem struct {
	ModelId            string             `json:"model_id"`                    // 模型标识
	ModelName          string             `json:"model_name"`                  // 模型显示名称
	Category           string             `json:"category"`                    // 分类（chat/embedding/image等）
	Vendor             string             `json:"vendor"`                      // 研发厂商（openai/anthropic/alibaba等，空=未分类）
	Description        string             `json:"description"`                 // 描述
	MaxContextTokens   int                `json:"max_context_tokens"`          // 最大上下文 tokens
	MaxOutputTokens    int                `json:"max_output_tokens"`           // 最大输出 tokens
	BillingMode        string             `json:"billing_mode"`                // 计费模式：token/per_request/tiered/per_second
	InputPrice         float64            `json:"input_price"`                 // 输入价格（每百万 token，USD；tiered=首档）
	OutputPrice        float64            `json:"output_price"`                // 输出价格（每百万 token，USD；tiered=首档）
	PerRequestPrice    float64            `json:"per_request_price"`           // 按次单价（USD/次，仅 per_request）
	PerSecondPrices    map[string]float64 `json:"per_second_prices,omitempty"` // 按秒单价矩阵（规格→USD/秒，仅 per_second），"*"为兜底价
	CacheReadPrice     float64            `json:"cache_read_price"`            // 缓存读取价格（每百万 token，USD）
	CacheCreationPrice float64            `json:"cache_creation_price"`        // 缓存创建/写入价格（每百万 token，USD；Claude creation 与 OpenAI write 合并）
	DiscountLabel      string             `json:"discount_label"`              // 折扣标签（营销展示，空=不展示）
	PriceChangeNote    string             `json:"price_change_note"`           // 价格调整说明（对外提示，空=不展示）
	TimePrices         []TimePriceItem    `json:"time_prices"`                 // 时段价目（平台基础价 × 时段乘数，无配置=空）
	Tags               []string           `json:"tags"`                        // 标签
	Capabilities       g.Map              `json:"capabilities"`                // 能力标签
	IsRecommended      bool               `json:"is_recommended"`              // 推荐位（运营手动勾选）
	IsNew              bool               `json:"is_new"`                      // 上新位（运营手动勾选，不按时间窗自动判定）
	CreatedAt          string             `json:"created_at"`                  // 上架日期（YYYY-MM-DD，新上线条排序与详情展示用）
}

type MarketplaceDetailReq struct {
	g.Meta  `path:"/marketplace/models/{model_id}" method:"get" tags:"公开-模型广场" summary:"获取模型详情" group:"public" middleware:"-"`
	ModelId string `json:"model_id" in:"path" v:"required" dc:"模型标识"`
}

type MarketplaceDetailRes struct {
	MarketplaceModelItem
}

type MarketplaceShowcaseReq struct {
	g.Meta `path:"/marketplace/showcase" method:"get" tags:"公开-模型广场" summary:"获取模型广场展示带（推荐+上新）" group:"public" middleware:"-"`
}

type MarketplaceShowcaseRes struct {
	// Recommended 推荐位模型（is_recommended=true，display_order 升序、model_id 兜底，上限 8）
	Recommended []MarketplaceModelItem `json:"recommended"`
	// Newest 上新位模型（is_new=true，created_at 倒序，上限 8；成员运营手动勾选）
	Newest []MarketplaceModelItem `json:"newest"`
}
