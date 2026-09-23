package common

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// TaskBillingProvider 异步任务计费接口
// 异步任务计费基于时长/分辨率等比率，非 token 数。
// ratios 键约定见 TaskAdaptor.EstimateBilling（float64 乘数 + spec.* 规格事实值）
type TaskBillingProvider interface {
	// EstimateTaskCost 估算任务费用。
	// taskBody 为归一化后的任务体（转换后通用格式）：定价配置了参数倍率（param_multipliers）时，
	// 实现方在函数内求值并把命中倍率就地注入 ratios（param_multiplier / param_matched 键，
	// 随任务持久化保证预扣/结算/重算三口径同源）；nil body = 不求值。
	EstimateTaskCost(ctx context.Context, tenantID int64, modelName string, ratios map[string]any, taskBody []byte) (decimal.Decimal, error)

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

	// SettleTaskSuccess 任务成功结算（含计费快照）。billAt 为任务受理时刻（时段定价按该时刻评估，
	// 异步任务结算滞后数分钟~小时，按结算时刻会丢失受理时的时段价）
	SettleTaskSuccess(ctx context.Context, tenantID, userID, apiKeyID, channelID int64, modelName, requestID string, actualCost, preDeductAmount decimal.Decimal, totalTokens, completionTokens int, ratios map[string]any, taskID string, billAt time.Time) (*SettlementResult, error)

	// SettleTaskFailed 任务失败退还预扣
	SettleTaskFailed(ctx context.Context, tenantID int64, requestID string, preDeductAmount decimal.Decimal) error

	// IncrApiKeyQuotaUsed 结算后累加 API Key 已用额度
	IncrApiKeyQuotaUsed(ctx context.Context, apiKeyID int64, amount decimal.Decimal)

	// AdjustTaskBilling 调整预扣金额（提交后上游确认了新参数）
	AdjustTaskBilling(ctx context.Context, tenantID int64, requestID string, preDeductAmount, newEstimatedCost decimal.Decimal) (decimal.Decimal, error)

	// RecalculateByTokens 根据上游返回的 total_tokens 重算费用（billAt 为任务受理时刻，时段定价用）
	// totalTokens: 上游返回的 token 计费单位
	// ratios: 提交时保存的计费上下文（如 video_input 折扣）
	RecalculateByTokens(ctx context.Context, tenantID int64, modelName string, totalTokens int, ratios map[string]any, billAt time.Time) (decimal.Decimal, error)

	// RecalculateByMaterials 按上游 usage 的素材计量重算任务费用（素材计费方案的结算依据）。
	// 仅模型配置了素材计费方案（pricing JSONB scheme）时适用：ok=true 返回按方案结算口径
	// 重算的最终费用；未配置方案、取价失败或 usage 为 nil 时 ok=false，调用方保持既有结算来源
	// （预扣金额 / token 重算）。billAt 为任务受理时刻（时段定价用），ratios 为提交时持久化的计费上下文
	RecalculateByMaterials(ctx context.Context, tenantID int64, modelName string, ratios map[string]any, usage *TaskMaterialUsage, billAt time.Time) (cost decimal.Decimal, ok bool, err error)
}
