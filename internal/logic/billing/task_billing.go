package billing

import (
	"context"
	"fmt"

	"github.com/qianfree/team-api/relay/common"
)

// TaskBillingProviderImpl 异步任务计费实现
type TaskBillingProviderImpl struct{}

// NewTaskBillingProvider 创建 TaskBillingProvider 实例
func NewTaskBillingProvider() common.TaskBillingProvider {
	return &TaskBillingProviderImpl{}
}

// EstimateTaskCost 估算任务费用
// ratios 包含计费比率（如 video_input 折扣）和预估参数（如 duration 秒数、resolution 乘数）
func (b *TaskBillingProviderImpl) EstimateTaskCost(ctx context.Context, tenantID int64, modelName string, ratios map[string]float64) (float64, error) {
	pricing, err := GetModelPrice(ctx, tenantID, modelName)
	if err != nil {
		return 0.01, nil
	}

	var cost float64

	// 如果配了 token 单价且有 duration 预估，用 token 模式估算
	if pricing.OutputPrice > 0 {
		duration := 5.0 // 默认 5 秒
		if d, ok := ratios["duration"]; ok && d > 0 {
			duration = d
		}
		resolutionMul := 2.25 // 默认 720p
		if r, ok := ratios["resolution"]; ok && r > 0 {
			resolutionMul = r
		}

		// 预估 tokens ≈ base_tokens_per_second × duration × resolution_multiplier
		// base_tokens_per_second 根据模型定价反推：
		// 火山方舟 completion_tokens 约 100 tokens/s (480p)，用于预扣估算
		estimatedTokens := 100.0 * duration * resolutionMul
		cost = estimatedTokens / 1_000_000.0 * pricing.OutputPrice * pricing.TenantMultiplier
	} else {
		cost = pricing.PerRequestPrice
	}

	// 应用附加比率（video_input 折扣等）
	for k, ratio := range ratios {
		if k == "duration" || k == "resolution" {
			continue
		}
		cost *= ratio
	}

	if cost < 0.01 {
		cost = 0.01
	}
	return cost, nil
}

// PreDeductTask 预扣任务费用
func (b *TaskBillingProviderImpl) PreDeductTask(ctx context.Context, tenantID int64, requestID string, estimatedCost float64) (float64, error) {
	ok, err := PreDeduct(ctx, tenantID, estimatedCost, requestID)
	if !ok {
		return 0, fmt.Errorf("pre-deduct task failed: %w", err)
	}
	return estimatedCost, nil
}

// SettleTaskSuccess 任务成功结算
func (b *TaskBillingProviderImpl) SettleTaskSuccess(ctx context.Context, tenantID, userID, apiKeyID, channelID int64, modelName, requestID string, actualCost, preDeductAmount float64) error {
	diff := actualCost - preDeductAmount

	// actualCost > preDeductAmount: 补扣差额
	// actualCost < preDeductAmount: 退还差额
	// 这里复用 Settle 方法，以 token 等价方式记录
	// 用实际费用/预扣费用差值来结算
	_, err := Settle(ctx, tenantID, userID, apiKeyID, channelID, modelName, requestID, "task", 0, 0, preDeductAmount)
	if err != nil {
		return fmt.Errorf("settle task success: %w", err)
	}

	// 如果有差额需要补扣或退还
	if diff > 0.001 {
		if ok, err := PreDeduct(ctx, tenantID, diff, requestID+"_adjust"); !ok {
			return fmt.Errorf("settle task adjust pre-deduct: %w", err)
		}
	} else if diff < -0.001 {
		if err := SettleFailed(ctx, tenantID, requestID+"_adjust", -diff); err != nil {
			return fmt.Errorf("settle task adjust refund: %w", err)
		}
	}
	return nil
}

// SettleTaskFailed 任务失败退还预扣
func (b *TaskBillingProviderImpl) SettleTaskFailed(ctx context.Context, tenantID int64, requestID string, preDeductAmount float64) error {
	return SettleFailed(ctx, tenantID, requestID, preDeductAmount)
}

// AdjustTaskBilling 调整预扣金额
func (b *TaskBillingProviderImpl) AdjustTaskBilling(ctx context.Context, tenantID int64, requestID string, preDeductAmount, newEstimatedCost float64) (float64, error) {
	diff := newEstimatedCost - preDeductAmount
	if diff < 0.001 && diff > -0.001 {
		return preDeductAmount, nil
	}

	if diff > 0 {
		// 需要补扣
		ok, err := PreDeduct(ctx, tenantID, diff, requestID+"_adjust")
		if !ok {
			return preDeductAmount, fmt.Errorf("adjust task billing: %w", err)
		}
		return newEstimatedCost, nil
	}

	// 需要退还部分
	if err := SettleFailed(ctx, tenantID, requestID+"_adjust", -diff); err != nil {
		return preDeductAmount, fmt.Errorf("adjust task billing refund: %w", err)
	}
	return newEstimatedCost, nil
}

// RecalculateByTokens 根据上游返回的 total_tokens 重算费用。
// 公式：totalTokens / 1M × output_price × ratios_product
// 如果模型没有配置 token 单价（纯按次计费），返回 0 表示不做 token 重算。
func (b *TaskBillingProviderImpl) RecalculateByTokens(ctx context.Context, tenantID int64, modelName string, totalTokens int, ratios map[string]float64) (float64, error) {
	if totalTokens <= 0 {
		return 0, nil
	}

	pricing, err := GetModelPrice(ctx, tenantID, modelName)
	if err != nil {
		return 0, nil
	}

	// 需要 output_price > 0 才能做 token 重算
	if pricing.OutputPrice <= 0 {
		return 0, nil
	}

	// 基础费用 = tokens × 单价
	cost := float64(totalTokens) / 1_000_000.0 * pricing.OutputPrice

	// 应用租户倍率
	cost *= pricing.TenantMultiplier

	// 应用附加比率（视频输入折扣等）
	for _, ratio := range ratios {
		cost *= ratio
	}

	// 最低消费
	if cost < 0.01 {
		cost = 0.01
	}

	return cost, nil
}
