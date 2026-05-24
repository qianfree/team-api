package common

import "context"

// BillingProvider 计费接口
// 实现在 internal/logic/billing/ 中，通过接口解耦 relay 层和 GoFrame
type BillingProvider interface {
	// PreDeduct 预扣费用
	// 检查余额 → 冻结预扣金额 → 返回是否成功
	PreDeduct(ctx context.Context, tenantID int64, modelName string, inputTokens, maxTokens int, isStream bool, requestID string) (preDeductAmount float64, err error)

	// Settle 结算费用（成功请求）
	Settle(ctx context.Context, tenantID, userID, apiKeyID, channelID int64, modelName, requestID, relayMode string, usage *Usage, preDeductAmount float64, projectID int64) error

	// SettleWithUsage 完整 Usage 结算（含 cache token + 计费快照）
	SettleWithUsage(ctx context.Context, tenantID, userID, apiKeyID, channelID int64,
		modelName, requestID, relayMode string,
		usage *Usage, preDeductAmount float64, relayInfo *RelayInfo) *SettlementResult

	// SettleFailed 失败请求结算（退还预扣）
	SettleFailed(ctx context.Context, tenantID int64, requestID string, preDeductAmount float64) error

	// SettleStreamInterrupted 流式中断结算
	SettleStreamInterrupted(ctx context.Context, tenantID, userID, apiKeyID, channelID int64, modelName, requestID, relayMode string, usage *Usage, preDeductAmount float64, projectID int64) error

	// CheckRateLimit QPS 限流检查
	CheckRateLimit(ctx context.Context, tenantID, userID, apiKeyID int64) (allowed bool, limitLevel string, remaining int, resetAt int64)

	// AcquireConcurrent 并发限制检查（含租户级 + 模型级）
	AcquireConcurrent(ctx context.Context, tenantID, userID, apiKeyID int64, modelName string) bool

	// ReleaseConcurrent 释放并发限制（含租户级 + 模型级）
	ReleaseConcurrent(ctx context.Context, tenantID, userID, apiKeyID int64, modelName string)

	// CheckScope 检查 API Key scope
	CheckScope(scope string, relayMode string) bool

	// CheckIPWhitelist 检查 IP 白名单
	CheckIPWhitelist(whitelist string, clientIP string) bool

	// CheckMemberQuota 检查成员额度是否足够
	CheckMemberQuota(ctx context.Context, tenantID, userID int64, preDeductAmount float64) error

	// IncrMemberQuotaUsed 结算后累加成员已用额度
	IncrMemberQuotaUsed(ctx context.Context, tenantID, userID int64, amount float64)
}

// RateLimitInfo 限流信息（用于设置响应头）
type RateLimitInfo struct {
	Limit     int
	Remaining int
	ResetAt   int64
}

// SettlementResult 结算结果（从 billing 层返回）
type SettlementResult struct {
	PreDeductAmount   float64
	BaseCost          float64
	ActualCost        float64
	RefundAmount      float64
	SupplementAmount  float64
	BillingRecordID   int64
	BillingSnapshot   string
	BillingSummary    string
	InputCost         float64
	OutputCost        float64
	CacheCreationCost float64
	CacheReadCost     float64
	TotalCost         float64
	BillingMode       string
	BillingSource     string
	RateMultiplier    float64
	PlanID            int64
	PlanDeduction     float64
	WalletDeduction   float64
}
