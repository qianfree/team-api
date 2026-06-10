package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 仪表盘 ===

type AdminDashboardReq struct {
	g.Meta `path:"/dashboard" method:"get" mime:"json" tags:"管理后台-仪表盘" summary:"仪表盘统计"`
}

type AdminDashboardRes struct {
	Tenants        int         `json:"tenants"`
	Members        int         `json:"members"`
	ActiveChannels int         `json:"active_channels"`
	Today          *DayStats   `json:"today"`
	Yesterday      *DayStats   `json:"yesterday"`
	Month          *MonthStats `json:"month"`
}

type DayStats struct {
	Requests      int     `json:"requests"`
	ActiveTenants int     `json:"active_tenants"`
	InputTokens   int     `json:"input_tokens"`
	OutputTokens  int     `json:"output_tokens"`
	TotalCost     float64 `json:"total_cost"`
	SuccessRate   float64 `json:"success_rate"`
}

type MonthStats struct {
	Requests     int     `json:"requests"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalCost    float64 `json:"total_cost"`
	Revenue      float64 `json:"revenue"`
}

type AdminDashboardTrendsReq struct {
	g.Meta `path:"/dashboard/trends" method:"get" mime:"json" tags:"管理后台-仪表盘" summary:"趋势数据"`
	Days   int `json:"days" d:"30" v:"between:1,90" dc:"天数"`
}

type AdminDashboardTrendsRes struct {
	List []map[string]any `json:"list"`
}

type AdminDashboardTopTenantsReq struct {
	g.Meta `path:"/dashboard/top-tenants" method:"get" mime:"json" tags:"管理后台-仪表盘" summary:"TOP租户"`
	Days   int `json:"days" d:"30" v:"between:1,90" dc:"天数"`
}

type AdminDashboardTopTenantsRes struct {
	List []map[string]any `json:"list"`
}

type AdminDashboardModelDistributionReq struct {
	g.Meta `path:"/dashboard/model-distribution" method:"get" mime:"json" tags:"管理后台-仪表盘" summary:"模型分布"`
	Days   int `json:"days" d:"30" v:"between:1,90" dc:"天数"`
}

type AdminDashboardModelDistributionRes struct {
	List []map[string]any `json:"list"`
}

type AdminDashboardChannelHealthReq struct {
	g.Meta `path:"/dashboard/channel-health" method:"get" mime:"json" tags:"管理后台-仪表盘" summary:"渠道健康概览"`
}

type ChannelHealthItem struct {
	ChannelId   int64   `json:"channel_id"`
	ChannelName string  `json:"channel_name"`
	Status      string  `json:"status"`
	HealthScore float64 `json:"health_score"`
	SuccessRate float64 `json:"success_rate"`
	LatencyMs   int     `json:"latency_ms"`
}

type AdminDashboardChannelHealthRes struct {
	List []ChannelHealthItem `json:"list"`
}

type AdminDashboardRecentAlertsReq struct {
	g.Meta `path:"/dashboard/recent-alerts" method:"get" mime:"json" tags:"管理后台-仪表盘" summary:"最近告警"`
}

type RecentAlertItem struct {
	Id             int64  `json:"id"`
	RuleName       string `json:"rule_name"`
	Level          string `json:"level"`
	Status         string `json:"status"`
	TriggerMessage string `json:"trigger_message"`
	CreatedAt      string `json:"created_at"`
}

type AdminDashboardRecentAlertsRes struct {
	List []RecentAlertItem `json:"list"`
}

// === 用量日志 ===

type AdminUsageLogListReq struct {
	g.Meta      `path:"/usage-logs" method:"get" mime:"json" tags:"管理后台-用量" summary:"用量日志列表"`
	Page        int    `json:"page" d:"1" dc:"页码"`
	PageSize    int    `json:"page_size" d:"20" v:"between:1,100" dc:"每页数量"`
	TenantID    int64  `json:"tenant_id" dc:"租户ID"`
	Username    string `json:"username" dc:"用户名（模糊匹配）"`
	Model       string `json:"model" dc:"模型名称"`
	Status      string `json:"status" dc:"状态"`
	RequestType int    `json:"request_type" dc:"请求类型: 1=同步, 2=流式, 3=异步, 4=WebSocket"`
	StartDate   string `json:"start_date" dc:"开始日期"`
	EndDate     string `json:"end_date" dc:"结束日期"`
}

type AdminUsageLogItem struct {
	Id                    int64       `json:"id"`
	TenantId              int64       `json:"tenant_id"`
	TenantName            string      `json:"tenant_name"`
	UserId                int64       `json:"user_id"`
	Username              string      `json:"username"`
	ProjectId             int64       `json:"project_id"`
	ProjectName           string      `json:"project_name"`
	ApiKeyId              int64       `json:"api_key_id"`
	ApiKeyName            string      `json:"api_key_name"`
	ChannelId             int64       `json:"channel_id"`
	ChannelName           string      `json:"channel_name"`
	ChannelType           int         `json:"channel_type"`
	ModelName             string      `json:"model_name"`
	RequestedModel        string      `json:"requested_model"`
	UpstreamModel         string      `json:"upstream_model"`
	RelayMode             string      `json:"relay_mode"`
	RequestType           int         `json:"request_type"`
	InputTokens           int         `json:"input_tokens"`
	OutputTokens          int         `json:"output_tokens"`
	CacheCreationTokens   int         `json:"cache_creation_tokens"`
	CacheReadTokens       int         `json:"cache_read_tokens"`
	CacheCreation5mTokens int         `json:"cache_creation_5m_tokens"`
	CacheCreation1hTokens int         `json:"cache_creation_1h_tokens"`
	ReasoningTokens       int         `json:"reasoning_tokens"`
	AudioInputTokens      int         `json:"audio_input_tokens"`
	AudioOutputTokens     int         `json:"audio_output_tokens"`
	ImageOutputTokens     int         `json:"image_output_tokens"`
	InputCost             float64     `json:"input_cost"`
	OutputCost            float64     `json:"output_cost"`
	CacheCreationCost     float64     `json:"cache_creation_cost"`
	CacheReadCost         float64     `json:"cache_read_cost"`
	TotalCost             float64     `json:"total_cost"`
	ActualCost            float64     `json:"actual_cost"`
	Currency              string      `json:"currency"`
	BillingMode           string      `json:"billing_mode"`
	BillingSource         string      `json:"billing_source"`
	RateMultiplier        float64     `json:"rate_multiplier"`
	LatencyMs             int         `json:"latency_ms"`
	FirstTokenMs          int         `json:"first_token_ms"`
	Status                string      `json:"status"`
	ErrorMessage          string      `json:"error_message"`
	RetryIndex            int         `json:"retry_index"`
	ClientIp              string      `json:"client_ip"`
	UserAgent             string      `json:"user_agent"`
	ServiceTier           string      `json:"service_tier"`
	ReasoningEffort       string      `json:"reasoning_effort"`
	StreamEndReason       string      `json:"stream_end_reason"`
	ImageCount            int         `json:"image_count"`
	ImageSize             string      `json:"image_size"`
	PreDeductAmount       float64     `json:"pre_deduct_amount"`
	RefundAmount          float64     `json:"refund_amount"`
	SupplementAmount      float64     `json:"supplement_amount"`
	BillingSummary        string      `json:"billing_summary"`
	BillingSnapshot       string      `json:"billing_snapshot"`
	InboundEndpoint       string      `json:"inbound_endpoint"`
	RequestId             string      `json:"request_id"`
	CreatedAt             *gtime.Time `json:"created_at"`
}

type AdminUsageLogListRes struct {
	List     []*AdminUsageLogItem `json:"list"`
	Total    int                  `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

// === 计费记录 ===

type AdminBillingRecordListReq struct {
	g.Meta   `path:"/billing-records" method:"get" mime:"json" tags:"管理后台-用量" summary:"计费记录列表"`
	Page     int   `json:"page" d:"1" dc:"页码"`
	PageSize int   `json:"page_size" d:"20" v:"between:1,100" dc:"每页数量"`
	TenantID int64 `json:"tenant_id" dc:"租户ID"`
}

type AdminBillingRecordItem struct {
	Id           int64       `json:"id"`
	TenantId     int64       `json:"tenant_id"`
	TenantName   string      `json:"tenant_name"`
	UserId       int64       `json:"user_id"`
	UserName     string      `json:"user_name"`
	ChannelId    int64       `json:"channel_id"`
	ChannelName  string      `json:"channel_name"`
	ModelName    string      `json:"model_name"`
	RelayMode    string      `json:"relay_mode"`
	InputTokens  int         `json:"input_tokens"`
	OutputTokens int         `json:"output_tokens"`
	InputPrice   float64     `json:"input_price"`
	OutputPrice  float64     `json:"output_price"`
	TotalCost    float64     `json:"total_cost"`
	Currency     string      `json:"currency"`
	Status       string      `json:"status"`
	SettledAt    *gtime.Time `json:"settled_at"`
	CreatedAt    *gtime.Time `json:"created_at"`
}

type AdminBillingRecordListRes struct {
	List     []*AdminBillingRecordItem `json:"list"`
	Total    int                       `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
}

// === 钱包管理 ===

type AdminWalletListReq struct {
	g.Meta   `path:"/wallets" method:"get" mime:"json" tags:"管理后台-钱包" summary:"钱包列表"`
	Page     int `json:"page" d:"1" dc:"页码"`
	PageSize int `json:"page_size" d:"20" v:"between:1,100" dc:"每页数量"`
}

type AdminWalletItem struct {
	Id               int64       `json:"id"`
	TenantId         int64       `json:"tenant_id"`
	Balance          float64     `json:"balance"`
	FrozenBalance    float64     `json:"frozen_balance"`
	WarningThreshold float64     `json:"warning_threshold"`
	Currency         string      `json:"currency"`
	CreatedAt        *gtime.Time `json:"created_at"`
	UpdatedAt        *gtime.Time `json:"updated_at"`
}

type AdminWalletListRes struct {
	List     []*AdminWalletItem `json:"list"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

type AdminWalletInfoReq struct {
	g.Meta   `path:"/wallets/{tenant_id}" method:"get" mime:"json" tags:"管理后台-钱包" summary:"钱包详情"`
	TenantID int64 `json:"tenant_id" in:"path" v:"required|min:1" dc:"租户ID"`
}

type AdminWalletInfoRes struct {
	Balance          float64  `json:"balance"`
	FrozenBalance    float64  `json:"frozen_balance"`
	WarningThreshold *float64 `json:"warning_threshold"`
}

type AdminWalletAdjustReq struct {
	g.Meta      `path:"/wallets/{tenant_id}/adjust" method:"post" mime:"json" tags:"管理后台-钱包" summary:"调整余额"`
	TenantID    int64   `json:"tenant_id" in:"path" v:"required|min:1" dc:"租户ID"`
	Amount      float64 `json:"amount" v:"required" dc:"调整金额（正数充值，负数扣减）"`
	Description string  `json:"description" dc:"调整说明"`
}

type AdminWalletAdjustRes struct{}

type AdminWalletTransactionListReq struct {
	g.Meta   `path:"/wallets/{tenant_id}/transactions" method:"get" mime:"json" tags:"管理后台-钱包" summary:"交易流水"`
	TenantID int64  `json:"tenant_id" in:"path" v:"required|min:1" dc:"租户ID"`
	Page     int    `json:"page" d:"1" dc:"页码"`
	PageSize int    `json:"page_size" d:"20" v:"between:1,100" dc:"每页数量"`
	Type     string `json:"type" dc:"交易类型"`
}

type AdminWalletTransactionItem struct {
	ID           int64   `json:"id"`
	Type         string  `json:"type"`
	Amount       float64 `json:"amount"`
	BalanceAfter float64 `json:"balance_after"`
	FrozenAfter  float64 `json:"frozen_after"`
	Description  string  `json:"description"`
	UserId       int64   `json:"user_id"`
	Username     string  `json:"username"`
	RequestId    string  `json:"request_id"`
	ModelName    string  `json:"model_name"`
	CreatedAt    string  `json:"created_at"`
}

type AdminWalletTransactionListRes struct {
	List     []*AdminWalletTransactionItem `json:"list"`
	Total    int                           `json:"total"`
	Page     int                           `json:"page"`
	PageSize int                           `json:"page_size"`
}

type AdminWalletSetWarningThresholdReq struct {
	g.Meta    `path:"/wallets/{tenant_id}/warning-threshold" method:"put" mime:"json" tags:"管理后台-钱包" summary:"设置预警阈值"`
	TenantID  int64   `json:"tenant_id" in:"path" v:"required|min:1" dc:"租户ID"`
	Threshold float64 `json:"threshold" v:"min:0" dc:"预警阈值"`
}

type AdminWalletSetWarningThresholdRes struct{}

// AdminUsageLogExportReq 导出用量日志请求
type AdminUsageLogExportReq struct {
	g.Meta      `path:"/usage-logs/export" method:"get" mime:"json" tags:"管理后台-用量" summary:"导出用量日志"`
	Format      string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	TenantID    int64  `json:"tenant_id" in:"query" dc:"租户ID"`
	Username    string `json:"username" in:"query" dc:"用户名（模糊匹配）"`
	Model       string `json:"model" in:"query" dc:"模型名称"`
	Status      string `json:"status" in:"query" dc:"状态"`
	RequestType int    `json:"request_type" in:"query" dc:"请求类型: 1=同步, 2=流式, 3=异步, 4=WebSocket"`
	StartDate   string `json:"start_date" in:"query" dc:"开始日期"`
	EndDate     string `json:"end_date" in:"query" dc:"结束日期"`
}

type AdminUsageLogExportRes struct{}

// AdminBillingRecordExportReq 导出计费记录请求
type AdminBillingRecordExportReq struct {
	g.Meta   `path:"/billing-records/export" method:"get" mime:"json" tags:"管理后台-用量" summary:"导出计费记录"`
	Format   string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	TenantID int64  `json:"tenant_id" in:"query" dc:"租户ID"`
}

type AdminBillingRecordExportRes struct{}

// === 交易流水 ===

type AdminTransactionListReq struct {
	g.Meta    `path:"/transactions" method:"get" mime:"json" tags:"管理后台-交易" summary:"交易流水列表"`
	Page      int    `json:"page" d:"1" dc:"页码"`
	PageSize  int    `json:"page_size" d:"20" v:"between:1,100" dc:"每页数量"`
	TenantID  int64  `json:"tenant_id" dc:"租户ID"`
	Type      string `json:"type" dc:"交易类型：consume/recharge/adjust"`
	Username  string `json:"username" dc:"用户名（模糊匹配）"`
	ModelName string `json:"model_name" dc:"模型名称（模糊匹配）"`
	StartDate string `json:"start_date" dc:"开始日期"`
	EndDate   string `json:"end_date" dc:"结束日期"`
}

type AdminTransactionItem struct {
	Id           int64   `json:"id"`
	TenantId     int64   `json:"tenant_id"`
	TenantName   string  `json:"tenant_name"`
	Type         string  `json:"type"`
	Amount       float64 `json:"amount"`
	BalanceAfter float64 `json:"balance_after"`
	Description  string  `json:"description"`
	UserId       int64   `json:"user_id"`
	Username     string  `json:"username"`
	RequestId    string  `json:"request_id"`
	ModelName    string  `json:"model_name"`
	CreatedAt    string  `json:"created_at"`
}

type AdminTransactionListRes struct {
	List     []*AdminTransactionItem `json:"list"`
	Total    int                     `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}
