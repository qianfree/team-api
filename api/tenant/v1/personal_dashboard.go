package v1

import "github.com/gogf/gf/v2/frame/g"

// === 租户个人看板 ===

// ---------- 1. Overview ----------

type PersonalDashboardReq struct {
	g.Meta `path:"/personal-dashboard" method:"get" mime:"application/json" tags:"租户控制台-个人看板" summary:"个人看板概览"`
}

type PersonalDashboardRes struct {
	Today        PersonalDayStats      `json:"today"`
	Month        PersonalDayStats      `json:"month"`
	ErrorRate    PersonalErrorRate     `json:"error_rate"`
	Latency      PersonalLatency       `json:"latency"`
	Cache        PersonalCache         `json:"cache"`
	RequestTypes []PersonalReqTypeItem `json:"request_types"`
	Quota        *PersonalQuotaStatus  `json:"quota,omitempty"`
}

type PersonalDayStats struct {
	Requests     int     `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalCost    float64 `json:"total_cost"`
}

type PersonalErrorRate struct {
	Total     int     `json:"total"`
	Success   int     `json:"success"`
	Error     int     `json:"error"`
	Timeout   int     `json:"timeout"`
	Cancelled int     `json:"cancelled"`
	Rate      float64 `json:"rate"`
}

type PersonalLatency struct {
	AvgMs           float64 `json:"avg_ms"`
	P50Ms           float64 `json:"p50_ms"`
	P95Ms           float64 `json:"p95_ms"`
	P99Ms           float64 `json:"p99_ms"`
	AvgFirstTokenMs float64 `json:"avg_first_token_ms"`
}

type PersonalCache struct {
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalInputTokens    int64   `json:"total_input_tokens"`
	HitRatio            float64 `json:"hit_ratio"`
}

type PersonalReqTypeItem struct {
	Type       string  `json:"type"`
	Label      string  `json:"label"`
	Requests   int     `json:"requests"`
	Percentage float64 `json:"percentage"`
}

type PersonalQuotaStatus struct {
	QuotaType    string  `json:"quota_type"`
	QuotaLimit   float64 `json:"quota_limit"`
	QuotaUsed    float64 `json:"quota_used"`
	Period       string  `json:"period"`
	UsagePercent float64 `json:"usage_percent"`
	NextResetAt  string  `json:"next_reset_at,omitempty"`
}

// ---------- 2. Token Trends ----------

type PersonalTokenTrendsReq struct {
	g.Meta `path:"/personal-dashboard/trends" method:"get" mime:"application/json" tags:"租户控制台-个人看板" summary:"个人Token趋势"`
	Days   int `json:"days" in:"query" d:"7"`
}

type PersonalTokenTrendsRes struct {
	List []PersonalTrendPoint `json:"list"`
}

type PersonalTrendPoint struct {
	Date         string  `json:"date"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	Requests     int     `json:"requests"`
	TotalCost    float64 `json:"total_cost"`
}

// ---------- 3. Model Distribution ----------

type PersonalModelDistReq struct {
	g.Meta `path:"/personal-dashboard/models" method:"get" mime:"application/json" tags:"租户控制台-个人看板" summary:"个人模型分布"`
	Days   int `json:"days" in:"query" d:"7"`
}

type PersonalModelDistRes struct {
	List []PersonalModelItem `json:"list"`
}

type PersonalModelItem struct {
	ModelName    string  `json:"model_name"`
	Requests     int     `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalCost    float64 `json:"total_cost"`
}

// ---------- 4. API Key Usage ----------

type PersonalApiKeyUsageReq struct {
	g.Meta `path:"/personal-dashboard/api-key-usage" method:"get" mime:"application/json" tags:"租户控制台-个人看板" summary:"个人API Key用量"`
	Days   int `json:"days" in:"query" d:"7"`
}

type PersonalApiKeyUsageRes struct {
	List []PersonalApiKeyItem `json:"list"`
}

type PersonalApiKeyItem struct {
	ApiKeyId     int64   `json:"api_key_id"`
	KeyName      string  `json:"key_name"`
	KeyPrefix    string  `json:"key_prefix"`
	Requests     int     `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalCost    float64 `json:"total_cost"`
}
