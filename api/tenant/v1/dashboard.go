package v1

import "github.com/gogf/gf/v2/frame/g"

// === 租户仪表盘 ===

// ---------- 1. 概览 ----------

type TenantDashboardReq struct {
	g.Meta `path:"/dashboard" method:"get" mime:"json" tags:"租户控制台-仪表盘" summary:"仪表盘概览"`
}

type TenantDashboardRes struct {
	Today       map[string]any  `json:"today"`
	Month       map[string]any  `json:"month"`
	Wallet      map[string]any  `json:"wallet"`
	Rpm         int64           `json:"rpm" dc:"最近60秒滑动窗口请求数（本租户）"`
	Tpm         int64           `json:"tpm" dc:"最近60秒滑动窗口token数（本租户）"`
	ActiveKeys  int             `json:"active_keys"`
	MemberCount int             `json:"member_count"`
	CostTrend   TenantCostTrend `json:"cost_trend" dc:"本月消费与上月同期对比"`
	Waste       TenantWasteStat `json:"waste" dc:"本月失败请求造成的无效支出"`
}

type TenantCostTrend struct {
	CurrentCost  float64 `json:"current_cost" dc:"本月至今消费"`
	PreviousCost float64 `json:"previous_cost" dc:"上月同期（相同已过天数）消费"`
	DeltaPercent float64 `json:"delta_percent" dc:"环比变化百分比；上月同期为 0 时记 0"`
	HasPrevious  bool    `json:"has_previous" dc:"上月同期是否有数据，false 时前端不显示环比"`
}

type TenantWasteStat struct {
	WastedCost      float64 `json:"wasted_cost" dc:"本月失败请求已产生的费用"`
	SharePercent    float64 `json:"share_percent" dc:"占本月总消费的百分比"`
	FailedRequests  int64   `json:"failed_requests" dc:"本月失败请求数"`
	RetriedRequests int64   `json:"retried_requests" dc:"本月触发过重试的请求数"`
}

// ---------- 2. 趋势与分布 ----------

type TenantTokenTrendsReq struct {
	g.Meta `path:"/dashboard/token-trends" method:"get" mime:"json" tags:"租户控制台-仪表盘" summary:"Token趋势"`
	Days   int `json:"days" in:"query" d:"7"`
}

type TenantTokenTrendsRes struct {
	List []map[string]any `json:"list"`
}

type TenantModelDistributionReq struct {
	g.Meta `path:"/dashboard/model-distribution" method:"get" mime:"json" tags:"租户控制台-仪表盘" summary:"模型分布"`
	Days   int `json:"days" in:"query" d:"7"`
}

type TenantModelDistributionRes struct {
	List []map[string]any `json:"list"`
}

// ---------- 3. 余额预测 ----------

type TenantBalancePredictionReq struct {
	g.Meta `path:"/dashboard/balance-prediction" method:"get" mime:"json" tags:"租户控制台-仪表盘" summary:"余额预测"`
}

type TenantBalancePredictionRes struct {
	DailyAvgCost     float64 `json:"daily_avg_cost"`
	AvailableBalance float64 `json:"available_balance"`
	WillExhaust      bool    `json:"will_exhaust"`
	DaysUntilExhaust *int    `json:"days_until_exhaust,omitempty"`
	ExhaustDate      *string `json:"exhaust_date,omitempty"`
	Message          *string `json:"message,omitempty"`
}

// ---------- 4. 预算告警 ----------

type TenantBudgetAlertsReq struct {
	g.Meta `path:"/dashboard/budget-alerts" method:"get" mime:"json" tags:"租户控制台-仪表盘" summary:"预算告警"`
}

type TenantBudgetAlertsRes struct {
	Members  []TenantMemberAlert  `json:"members"`
	Projects []TenantProjectAlert `json:"projects"`
}

type TenantMemberAlert struct {
	Id           int64   `json:"id"`
	Username     string  `json:"username"`
	DisplayName  string  `json:"display_name"`
	QuotaLimit   float64 `json:"quota_limit"`
	QuotaUsed    float64 `json:"quota_used"`
	UsagePercent float64 `json:"usage_percent"`
	NextResetAt  string  `json:"next_reset_at,omitempty" dc:"周期额度的下次重置时间"`
}

type TenantProjectAlert struct {
	Id           int64   `json:"id"`
	Name         string  `json:"name"`
	Budget       float64 `json:"budget"`
	Used         float64 `json:"used"`
	UsagePercent float64 `json:"usage_percent"`
}

// ---------- 5. 成员用量排行 ----------

type TenantMemberUsageRankingReq struct {
	g.Meta `path:"/dashboard/member-usage-ranking" method:"get" mime:"json" tags:"租户控制台-仪表盘" summary:"成员用量排名"`
	Days   int `json:"days" in:"query" d:"7"`
	Limit  int `json:"limit" in:"query" d:"10"`
}

type TenantMemberUsageRankingRes struct {
	List          []TenantMemberUsageItem `json:"list"`
	PrevAvailable bool                    `json:"prev_available" dc:"对比周期是否仍在用量日志保留期内；false 时前端应隐藏环比列"`
}

type TenantMemberUsageItem struct {
	UserId       int64   `json:"user_id"`
	Username     string  `json:"username"`
	DisplayName  string  `json:"display_name"`
	Requests     int64   `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalCost    float64 `json:"total_cost"`
	DeltaPercent float64 `json:"delta_percent" dc:"与上一同长周期相比的消费变化百分比"`
	HasPrevious  bool    `json:"has_previous" dc:"上一周期是否有数据，false 时前端不显示环比"`
	QuotaPercent float64 `json:"quota_percent" dc:"个人额度使用百分比；未设额度时为 0"`
	HasQuota     bool    `json:"has_quota" dc:"是否设置了个人额度"`
}

// ---------- 6. 团队运行质量 ----------

type TenantTeamHealthReq struct {
	g.Meta `path:"/dashboard/team-health" method:"get" mime:"application/json" tags:"租户控制台-仪表盘" summary:"团队运行质量"`
}

type TenantTeamHealthRes struct {
	TotalRequests   int64   `json:"total_requests" dc:"本月请求总数"`
	SuccessRate     float64 `json:"success_rate" dc:"成功率（0-1）"`
	P95Ms           float64 `json:"p95_ms" dc:"成功请求的 P95 延迟"`
	AvgFirstTokenMs float64 `json:"avg_first_token_ms" dc:"平均首 Token 延迟"`
	CacheHitRatio   float64 `json:"cache_hit_ratio" dc:"缓存命中率（0-1）"`
	CacheReadTokens int64   `json:"cache_read_tokens" dc:"缓存命中的 Token 数"`
}

// ---------- 7. 项目预算 ----------

type TenantProjectBudgetReq struct {
	g.Meta `path:"/dashboard/project-budget" method:"get" mime:"application/json" tags:"租户控制台-仪表盘" summary:"项目预算使用情况"`
}

type TenantProjectBudgetRes struct {
	List []TenantProjectBudgetItem `json:"list"`
}

type TenantProjectBudgetItem struct {
	Id           int64   `json:"id"`
	Name         string  `json:"name"`
	Budget       float64 `json:"budget" dc:"累计预算上限；0 表示不限"`
	Used         float64 `json:"used" dc:"累计已用（与预算执行同源，取自 bil_transactions）"`
	UsagePercent float64 `json:"usage_percent" dc:"已用百分比；未设预算时为 0"`
	HasBudget    bool    `json:"has_budget" dc:"是否设置了预算"`
}
