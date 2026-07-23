package common

import (
	"context"

	"github.com/shopspring/decimal"
)

// TaskBillingProvider 异步任务计费接口
// 异步任务计费基于时长/分辨率等比率，非 token 数
type TaskBillingProvider interface {
	// EstimateTaskCost 估算任务费用
	EstimateTaskCost(ctx context.Context, tenantID int64, modelName string, ratios map[string]float64) (decimal.Decimal, error)

	// PreDeductTask 预扣任务费用
	PreDeductTask(ctx context.Context, tenantID int64, requestID string, estimatedCost decimal.Decimal, modelName string) (decimal.Decimal, error)

	// CheckRateLimit QPS 限流检查
	CheckRateLimit(ctx context.Context, tenantID, userID, apiKeyID int64, keyQPS int) (allowed bool, limitLevel string, limit int, remaining int, resetAt int64)

	// AcquireApiKeyConcurrent 获取 API Key 级并发许可
	AcquireApiKeyConcurrent(ctx context.Context, apiKeyID int64, limit int) bool

	// ReleaseApiKeyConcurrent 释放 API Key 级并发许可
	ReleaseApiKeyConcurrent(ctx context.Context, apiKeyID int64)

	// CheckApiKeyQuota 检查 API Key 额度是否足够
	CheckApiKeyQuota(ctx context.Context, apiKeyID int64, preDeductAmount decimal.Decimal) error

	// SettleTaskSuccess 任务成功结算（含计费快照）
	SettleTaskSuccess(ctx context.Context, tenantID, userID, apiKeyID, channelID int64, modelName, requestID string, actualCost, preDeductAmount decimal.Decimal, totalTokens, completionTokens int, ratios map[string]float64, taskID string) (*SettlementResult, error)

	// SettleTaskFailed 任务失败退还预扣
	SettleTaskFailed(ctx context.Context, tenantID int64, requestID string, preDeductAmount decimal.Decimal) error

	// IncrApiKeyQuotaUsed 结算后累加 API Key 已用额度
	IncrApiKeyQuotaUsed(ctx context.Context, apiKeyID int64, amount decimal.Decimal)

	// AdjustTaskBilling 调整预扣金额（提交后上游确认了新参数）
	AdjustTaskBilling(ctx context.Context, tenantID int64, requestID string, preDeductAmount, newEstimatedCost decimal.Decimal) (decimal.Decimal, error)

	// RecalculateByTokens 根据上游返回的 total_tokens 重算费用
	// totalTokens: 上游返回的 token 计费单位
	// ratios: 提交时保存的计费比率（如 video_input 折扣）
	RecalculateByTokens(ctx context.Context, tenantID int64, modelName string, totalTokens int, ratios map[string]float64) (decimal.Decimal, error)
}
