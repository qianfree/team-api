package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

// ============================================================
// 工作台（运营收件箱）
//
// 与仪表盘（/dashboard）的分工：仪表盘回答「现在怎么样」，工作台回答「我该做什么」。
// 只有满足「有人要处理 + 不处理会恶化 + 能被清空归零」三条的事才进工作台，
// 纯统计一律留在仪表盘。
// ============================================================

// === 严重度 / 域 常量（与前端约定，勿随意改动字面量）===
const (
	WorkbenchSeverityP0 = "p0" // 紧急：正在影响服务可用性或资金安全
	WorkbenchSeverityP1 = "p1" // 今日：当天要处理
	WorkbenchSeverityP2 = "p2" // 本周：可排期

	WorkbenchDomainAvailability = "availability" // 服务可用性
	WorkbenchDomainMoney        = "money"        // 资金安全
	WorkbenchDomainCustomer     = "customer"     // 客户运营
	WorkbenchDomainSystem       = "system"       // 系统安全
)

// WorkbenchItem 一条待办。
//
// Key 是稳定可重算的业务标识（如 model_no_channel:claude-sonnet-4），
// 供前端做列表 key 与去重 —— 同一异常在两次收集之间必须算出同一个 Key，
// 所以禁止把时间戳、随机数或行 ID 拼进 Key（除非该行本身就是待办对象）。
type WorkbenchItem struct {
	Key      string `json:"key" dc:"待办唯一标识（稳定可重算，用于静音匹配）"`
	Severity string `json:"severity" dc:"严重度：p0=紧急 / p1=今日 / p2=本周"`
	Domain   string `json:"domain" dc:"所属域：availability / money / customer / system"`
	Title    string `json:"title" dc:"标题（一句话说清是什么问题）"`
	Desc     string `json:"desc" dc:"补充说明（影响面、判断依据）"`
	// OccurredAt 为空表示该待办是聚合计数（如「5 条反馈待处理」）而非单点事件。
	OccurredAt  string            `json:"occurred_at" dc:"发生时间（RFC3339），聚合类待办为空"`
	ActionText  string            `json:"action_text" dc:"跳转按钮文案"`
	ActionRoute string            `json:"action_route" dc:"跳转目标前端路由名"`
	ActionQuery map[string]string `json:"action_query" dc:"跳转时附带的查询参数（自动带上筛选条件）"`
}

// WorkbenchMetric 首屏关键数字。
type WorkbenchMetric struct {
	Key   string  `json:"key" dc:"指标标识"`
	Label string  `json:"label" dc:"展示名"`
	Value float64 `json:"value" dc:"数值"`
	// Unit 决定前端格式化方式：money=本位币金额 / percent=百分比 / count=计数
	Unit string `json:"unit" dc:"单位类型：money / percent / count"`
	// Growth 为 nil 表示该指标没有同比口径（如「余额告警租户数」）
	Growth *float64 `json:"growth" dc:"环比变化百分比；null 表示无对比口径"`
	Sub    string   `json:"sub" dc:"副标题（说明对比口径或统计窗口）"`
}

// WorkbenchDomainStat 分域计数。
type WorkbenchDomainStat struct {
	Domain string `json:"domain"`
	Total  int    `json:"total" dc:"该域待办总数"`
	Urgent int    `json:"urgent" dc:"该域 P0 数量"`
}

// WorkbenchModelAvail 模型可用渠道速览。
type WorkbenchModelAvail struct {
	Model     string `json:"model"`
	Available int    `json:"available" dc:"熔断均为 CLOSED 的可用渠道数"`
	Total     int    `json:"total" dc:"承载该模型的 active 渠道数"`
}

// WorkbenchBreaker 正在熔断的渠道×模型。
type WorkbenchBreaker struct {
	ChannelId    int64  `json:"channel_id"`
	ChannelName  string `json:"channel_name"`
	Model        string `json:"model"`
	ChannelLevel bool   `json:"channel_level" dc:"true=渠道级熔断（该渠道所有模型受影响）；false=仅该模型被隔离"`
	HalfOpen     bool   `json:"half_open" dc:"是否处于半开探测态"`
}

// === 工作台汇总 ===

type AdminWorkbenchSummaryReq struct {
	g.Meta `path:"/workbench/summary" method:"get" mime:"json" tags:"管理后台-工作台" summary:"工作台汇总"`
}

type AdminWorkbenchSummaryRes struct {
	Metrics     []WorkbenchMetric     `json:"metrics" dc:"首屏关键数字"`
	Items       []WorkbenchItem       `json:"items" dc:"待办列表（已按 p0→p1→p2 排序，已剔除静音项）"`
	Domains     []WorkbenchDomainStat `json:"domains" dc:"分域计数"`
	Models      []WorkbenchModelAvail `json:"models" dc:"模型可用渠道速览"`
	Breakers    []WorkbenchBreaker    `json:"breakers" dc:"正在熔断的渠道×模型"`
	GeneratedAt string                `json:"generated_at" dc:"数据生成时间（RFC3339），命中缓存时为缓存写入时刻"`
}

// === 菜单红点 ===

// AdminWorkbenchBadgeReq 轻量计数接口，供左侧菜单红点轮询。
// 与 summary 共用同一份缓存，不会额外压库。
type AdminWorkbenchBadgeReq struct {
	g.Meta `path:"/workbench/badges" method:"get" mime:"json" tags:"管理后台-工作台" summary:"菜单红点计数"`
}

type AdminWorkbenchBadgeRes struct {
	Total   int            `json:"total" dc:"待办总数"`
	Urgent  int            `json:"urgent" dc:"P0 数量"`
	Domains map[string]int `json:"domains" dc:"各域待办数，键为域标识"`
	// Menus 前端菜单路由名 → 待办数，直接挂角标，前端不需要自己映射域到菜单。
	Menus map[string]int `json:"menus" dc:"菜单路由名 → 待办数"`
}
