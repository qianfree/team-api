package common

import "context"

// TaskBillingProvider 异步任务计费接口
// 异步任务计费基于时长/分辨率等比率，非 token 数
type TaskBillingProvider interface {
	// EstimateTaskCost 估算任务费用
	EstimateTaskCost(ctx context.Context, tenantID int64, modelName string, ratios map[string]float64) (float64, error)

	// PreDeductTask 预扣任务费用
	PreDeductTask(ctx context.Context, tenantID int64, requestID string, estimatedCost float64) (float64, error)

	// SettleTaskSuccess 任务成功结算
	SettleTaskSuccess(ctx context.Context, tenantID, userID, apiKeyID, channelID int64, modelName, requestID string, actualCost, preDeductAmount float64) error

	// SettleTaskFailed 任务失败退还预扣
	SettleTaskFailed(ctx context.Context, tenantID int64, requestID string, preDeductAmount float64) error

	// AdjustTaskBilling 调整预扣金额（提交后上游确认了新参数）
	AdjustTaskBilling(ctx context.Context, tenantID int64, requestID string, preDeductAmount, newEstimatedCost float64) (float64, error)
}
