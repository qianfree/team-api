package v1

import "github.com/gogf/gf/v2/frame/g"

// ModelComparisonReq compares 2-4 models across cost, latency, and success rate.
type ModelComparisonReq struct {
	g.Meta `path:"/model-comparison" method:"get" tags:"TenantService" summary:"模型成本对比"`
	Models string `json:"models" in:"query" v:"required" dc:"对比的模型名称，英文逗号分隔，2-4 个"`
	Days   int    `json:"days" in:"query" v:"between:1,90" dc:"统计天数，默认 7" d:"7"`
}

type ModelComparisonRes struct {
	Summary ModelComparisonSummary `json:"summary"`
	Items   []ModelComparisonItem  `json:"items"`
	Trends  []ModelTrendDay        `json:"trends"`
}

type ModelComparisonSummary struct {
	TotalRequests int64   `json:"total_requests"`
	TotalCost     float64 `json:"total_cost"`
	Recommended   string  `json:"recommended"`
	Reason        string  `json:"reason"`
}

type ModelComparisonItem struct {
	ModelName         string  `json:"model_name"`
	Requests          int64   `json:"requests"`
	SuccessRate       float64 `json:"success_rate"`
	AvgLatencyMs      float64 `json:"avg_latency_ms"`
	P95LatencyMs      float64 `json:"p95_latency_ms"`
	TotalCost         float64 `json:"total_cost"`
	AvgCostPerRequest float64 `json:"avg_cost_per_request"`
	InputTokens       int64   `json:"input_tokens"`
	OutputTokens      int64   `json:"output_tokens"`
	Score             float64 `json:"score"`
	IsRecommended     bool    `json:"is_recommended"`
}

type ModelTrendDay struct {
	Date    string              `json:"date"`
	Details []ModelTrendDayItem `json:"details"`
}

type ModelTrendDayItem struct {
	ModelName string  `json:"model_name"`
	Requests  int64   `json:"requests"`
	Cost      float64 `json:"cost"`
	LatencyMs float64 `json:"latency_ms"`
}
