package billing

import (
	"context"

	"github.com/qianfree/team-api/relay/common"
)

// BillingProviderImpl 实现 relay/common.BillingProvider 接口
type BillingProviderImpl struct{}

// NewBillingProvider 创建 BillingProvider 实例
func NewBillingProvider() common.BillingProvider {
	return &BillingProviderImpl{}
}

func (b *BillingProviderImpl) PreDeduct(ctx context.Context, tenantID int64, modelName string, inputTokens, maxTokens int, isStream bool, requestID string) (float64, error) {
	amount, err := EstimatePreDeductAmount(ctx, tenantID, modelName, inputTokens, maxTokens, isStream)
	if err != nil {
		return 0, err
	}

	ok, err := PreDeduct(ctx, tenantID, amount, requestID, modelName)
	if !ok {
		return 0, err
	}

	return amount, nil
}

func (b *BillingProviderImpl) Settle(ctx context.Context, tenantID, userID, apiKeyID, channelID int64, modelName, requestID, relayMode string, usage *common.Usage, preDeductAmount float64) error {
	inputTokens, outputTokens := 0, 0
	if usage != nil {
		inputTokens = usage.PromptTokens
		outputTokens = usage.CompletionTokens
	}
	_, err := Settle(ctx, tenantID, userID, apiKeyID, channelID, modelName, requestID, relayMode, inputTokens, outputTokens, preDeductAmount)
	return err
}

func (b *BillingProviderImpl) SettleWithUsage(ctx context.Context, tenantID, userID, apiKeyID, channelID int64,
	modelName, requestID, relayMode string,
	usage *common.Usage, preDeductAmount float64, relayInfo *common.RelayInfo) *common.SettlementResult {

	result, err := SettleWithUsage(ctx, tenantID, userID, apiKeyID, channelID, modelName, requestID, relayMode, usage, preDeductAmount, relayInfo)
	if err != nil {
		return nil
	}
	var inputCost, outputCost, cacheCreationCost, cacheReadCost float64
	if result.CostBreakdown != nil {
		inputCost = result.CostBreakdown.InputCost
		outputCost = result.CostBreakdown.OutputCost
		cacheCreationCost = result.CostBreakdown.CacheCreationCost
		cacheReadCost = result.CostBreakdown.CacheReadCost
	}
	return &common.SettlementResult{
		PreDeductAmount:   result.PreDeductAmount,
		BaseCost:          result.BaseCost,
		ActualCost:        result.ActualCost,
		RefundAmount:      result.RefundAmount,
		SupplementAmount:  result.SupplementAmount,
		BillingRecordID:   result.BillingRecordID,
		BillingSnapshot:   result.BillingSnapshot,
		BillingSummary:    result.BillingSummary,
		InputCost:         inputCost,
		OutputCost:        outputCost,
		CacheCreationCost: cacheCreationCost,
		CacheReadCost:     cacheReadCost,
		TotalCost:         result.ActualCost,
		BillingMode:       result.BillingMode,
		BillingSource:     result.BillingSource,
		RateMultiplier:    result.RateMultiplier,
	}
}

func (b *BillingProviderImpl) SettleFailed(ctx context.Context, tenantID int64, requestID string, preDeductAmount float64) error {
	return SettleFailed(ctx, tenantID, requestID, preDeductAmount)
}

func (b *BillingProviderImpl) SettleStreamInterrupted(ctx context.Context, tenantID, userID, apiKeyID, channelID int64, modelName, requestID, relayMode string, usage *common.Usage, preDeductAmount float64) error {
	confirmedInput, confirmedOutput := 0, 0
	if usage != nil {
		confirmedInput = usage.PromptTokens
		confirmedOutput = usage.CompletionTokens
	}
	_, err := SettleStreamInterrupted(ctx, tenantID, userID, apiKeyID, channelID, modelName, requestID, relayMode, confirmedInput, confirmedOutput, preDeductAmount)
	return err
}

func (b *BillingProviderImpl) CheckRateLimit(ctx context.Context, tenantID, userID, apiKeyID int64) (bool, string, int, int64) {
	result := CheckRateLimit(ctx, LoadRateLimitConfig(ctx), tenantID, userID, apiKeyID)
	return result.Allowed, result.LimitLevel, result.Remaining, result.ResetAt
}

func (b *BillingProviderImpl) AcquireConcurrent(ctx context.Context, tenantID, userID, apiKeyID int64, modelName string) bool {
	return AcquireConcurrent(ctx, LoadRateLimitConfig(ctx), tenantID, userID, apiKeyID, modelName)
}

func (b *BillingProviderImpl) ReleaseConcurrent(ctx context.Context, tenantID, userID, apiKeyID int64, modelName string) {
	ReleaseConcurrent(ctx, tenantID, userID, apiKeyID, modelName)
}

func (b *BillingProviderImpl) CheckScope(scope string, relayMode string) bool {
	return CheckScope(scope, relayMode)
}

func (b *BillingProviderImpl) CheckIPWhitelist(whitelist string, clientIP string) bool {
	return CheckIPWhitelist(whitelist, clientIP)
}

func (b *BillingProviderImpl) CheckMemberQuota(ctx context.Context, tenantID, userID int64, preDeductAmount float64) error {
	return CheckMemberQuota(ctx, tenantID, userID, preDeductAmount)
}

func (b *BillingProviderImpl) IncrMemberQuotaUsed(ctx context.Context, tenantID, userID int64, amount float64) {
	IncrMemberQuotaUsed(ctx, tenantID, userID, amount)
}
