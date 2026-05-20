package common

import "context"

// TaskBillingProvider 异步任务计费接口
// 异步任务计费基于时长/分辨率等比率，非 token 数
type TaskBillingProvider interface {
	// EstimateTaskCost 估算任务费用
	EstimateTaskCost(ctx context.Context, tenantID int64, modelName string, ratios map[string]float64) (float64, error)

	// PreDeductTask 预扣任务费用
	PreDeductTask(ctx context.Context, tenantID int64, requestID string, estimatedCost float64) (float64, error)

	// SettleTaskSuccess 任务成功结算（含计费快照）
	SettleTaskSuccess(ctx context.Context, tenantID, userID, apiKeyID, channelID int64, modelName, requestID string, actualCost, preDeductAmount float64, totalTokens, completionTokens int, ratios map[string]float64) (*SettlementResult, error)

	// SettleTaskFailed 任务失败退还预扣
	SettleTaskFailed(ctx context.Context, tenantID int64, requestID string, preDeductAmount float64) error

	// AdjustTaskBilling 调整预扣金额（提交后上游确认了新参数）
	AdjustTaskBilling(ctx context.Context, tenantID int64, requestID string, preDeductAmount, newEstimatedCost float64) (float64, error)

	// RecalculateByTokens 根据上游返回的 total_tokens 重算费用
	// totalTokens: 上游返回的 token 计费单位
	// ratios: 提交时保存的计费比率（如 video_input 折扣）
	RecalculateByTokens(ctx context.Context, tenantID int64, modelName string, totalTokens int, ratios map[string]float64) (float64, error)
}
