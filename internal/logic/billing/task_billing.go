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
// ratios 包含计费比率（如 duration_ratio, resolution_ratio）
func (b *TaskBillingProviderImpl) EstimateTaskCost(ctx context.Context, tenantID int64, modelName string, ratios map[string]float64) (float64, error) {
	// 基础价格从模型定价表获取
	pricing, err := GetModelPrice(ctx, tenantID, modelName)
	if err != nil {
		return 0.01, nil
	}
	baseCost := pricing.PerRequestPrice
	if err != nil {
		return 0, fmt.Errorf("estimate task cost: %w", err)
	}

	// 应用比率乘数
	for _, ratio := range ratios {
		baseCost *= ratio
	}

	// 最低消费保障
	if baseCost < 0.01 {
		baseCost = 0.01
	}
	return baseCost, nil
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
