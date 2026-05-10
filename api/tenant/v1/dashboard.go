package v1

import "github.com/gogf/gf/v2/frame/g"

// === 租户仪表盘 ===

type TenantDashboardReq struct {
	g.Meta `path:"/dashboard" method:"get" mime:"json" tags:"租户控制台-仪表盘" summary:"仪表盘概览"`
}

type TenantDashboardRes struct {
	Today       map[string]any `json:"today"`
	Month       map[string]any `json:"month"`
	Wallet      map[string]any `json:"wallet"`
	ActiveKeys  int            `json:"active_keys"`
	MemberCount int            `json:"member_count"`
}

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

type TenantBudgetAlertsReq struct {
	g.Meta `path:"/dashboard/budget-alerts" method:"get" mime:"json" tags:"租户控制台-仪表盘" summary:"预算告警"`
}

type TenantBudgetAlertsRes struct {
	Members  []map[string]any `json:"members"`
	Projects []map[string]any `json:"projects"`
}

type TenantMemberUsageRankingReq struct {
	g.Meta `path:"/dashboard/member-usage-ranking" method:"get" mime:"json" tags:"租户控制台-仪表盘" summary:"成员用量排名"`
	Days   int `json:"days" in:"query" d:"7"`
	Limit  int `json:"limit" in:"query" d:"10"`
}

type TenantMemberUsageRankingRes struct {
	List []map[string]any `json:"list"`
}
