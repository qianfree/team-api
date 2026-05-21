package billing

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	do "github.com/qianfree/team-api/internal/model/do"
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

	// 按次计费：直接用单价
	if pricing.BillingMode == "per_request" {
		cost = pricing.PerRequestPrice
	} else if pricing.OutputPrice > 0 {
		duration := 5.0 // 默认 5 秒
		if d, ok := ratios["duration"]; ok && d > 0 {
			duration = d
		}
		resolutionMul := 2.25 // 默认 720p
		if r, ok := ratios["resolution"]; ok && r > 0 {
			resolutionMul = r
		}

		// 预估 tokens ≈ base_tokens_per_second × duration × resolution_multiplier
		// 火山方舟视频生成约 10000 tokens/s (480p 基准)，用于预扣估算
		estimatedTokens := 10000.0 * duration * resolutionMul
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
func (b *TaskBillingProviderImpl) PreDeductTask(ctx context.Context, tenantID int64, requestID string, estimatedCost float64, modelName string) (float64, error) {
	ok, err := PreDeduct(ctx, tenantID, estimatedCost, requestID, modelName)
	if !ok {
		return 0, fmt.Errorf("pre-deduct task failed: %w", err)
	}
	return estimatedCost, nil
}

// SettleTaskSuccess 任务成功结算（含计费快照）
// totalTokens/completionTokens: 上游返回的 token 用量
// ratios: 提交时保存的计费比率（如 video_input 折扣）
func (b *TaskBillingProviderImpl) SettleTaskSuccess(ctx context.Context, tenantID, userID, apiKeyID, channelID int64, modelName, requestID string, actualCost, preDeductAmount float64, totalTokens, completionTokens int, ratios map[string]float64, taskID string) (*common.SettlementResult, error) {
	diff := actualCost - preDeductAmount

	// 1. 获取钱包
	wallet, err := GetWallet(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("settle task: get wallet: %w", err)
	}

	// 2. 获取定价（事务外只读）
	pricing, _ := GetModelPrice(ctx, tenantID, modelName)
	breakdown := buildTaskCostBreakdown(pricing, actualCost, totalTokens, completionTokens)

	var billingMode string
	var discountRatio, effectiveOutputPrice float64
	if pricing != nil {
		billingMode = pricing.BillingMode
		discountRatio = pricing.DiscountRatio
		effectiveOutputPrice = pricing.OutputPrice
	}

	// 3. 事务内执行结算（钱包扣款 + 计费记录）
	var balanceAfter, frozenAfter float64
	var taskBillingID int64
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 3a. 更新钱包
		now := time.Now()
		_, err := tx.Ctx(ctx).Exec(
			"UPDATE bil_wallets SET frozen_balance = GREATEST(frozen_balance - ?, 0), balance = balance - ?, updated_at = ? WHERE id = ?",
			preDeductAmount, actualCost, now, wallet.ID)
		if err != nil {
			return fmt.Errorf("settle task: update wallet: %w", err)
		}

		// 3b. 事务内读取准确余额
		type balRow struct {
			Balance       float64 `json:"balance"`
			FrozenBalance float64 `json:"frozen_balance"`
		}
		var br *balRow
		err = tx.Model("bil_wallets").Ctx(ctx).
			Where("id", wallet.ID).
			Fields("balance, frozen_balance").
			Scan(&br)
		if err == nil && br != nil {
			balanceAfter = br.Balance
			frozenAfter = br.FrozenBalance
		}

		// 3c. 创建计费记录
		var billingResult sql.Result
		billingResult, err = tx.Model("bil_records").Ctx(ctx).Data(do.BilRecords{
			TenantId:     tenantID,
			UserId:       userID,
			ApiKeyId:     apiKeyID,
			ChannelId:    channelID,
			ModelName:    modelName,
			RequestId:    requestID,
			RelayMode:    "task",
			InputTokens:  0,
			OutputTokens: totalTokens,
			InputPrice:   0,
			OutputPrice:  effectiveOutputPrice,
			TotalCost:    actualCost,
			Currency:     "USD",
			Status:       "settled",
			SettledAt:    gtime.NewFromTime(now),
			BillingMode:  billingMode,
			DiscountRatio: func() float64 {
				if discountRatio > 0 {
					return discountRatio
				}
				return 0
			}(),
			EffectiveInputPrice:     0,
			EffectiveOutputPrice:    effectiveOutputPrice,
			BillingInputMultiplier:  0,
			BillingOutputMultiplier: 0,
			CacheCreationTokens:     0,
			CacheReadTokens:         0,
			CacheCreationCost:       0,
			CacheReadCost:           0,
		}).Insert()
		if err != nil {
			return fmt.Errorf("settle task: create billing record: %w", err)
		}
		if billingResult != nil {
			taskBillingID, _ = billingResult.LastInsertId()
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 4. 清除缓存（事务提交后）
	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
	InvalidateWalletRedis(ctx, tenantID)
	CleanupPreDeduct(ctx, tenantID, requestID)
	CleanupPreDeduct(ctx, tenantID, requestID+"_adjust")

	// 5. 差额处理：实际 < 预扣时退还差额（实际 > 预扣时已在步骤 3 扣完，无需额外操作）
	if diff < -0.001 {
		if err := SettleFailed(ctx, tenantID, requestID+"_adjust", -diff); err != nil {
			g.Log().Warningf(ctx, "settle task adjust refund failed for %s: %v", requestID, err)
		}
	}

	// 6. 生成计费快照 + 摘要
	result := &common.SettlementResult{
		PreDeductAmount: preDeductAmount,
		ActualCost:      actualCost,
		BaseCost:        breakdown.BaseCost,
		TotalCost:       actualCost,
		OutputCost:      breakdown.OutputCost,
	}

	if diff > 0.001 {
		result.SupplementAmount = diff
	} else if diff < -0.001 {
		result.RefundAmount = -diff
	}

	if pricing != nil {
		internalSettlement := &SettlementResult{
			PreDeductAmount:  preDeductAmount,
			ActualCost:       actualCost,
			BaseCost:         breakdown.BaseCost,
			RefundAmount:     result.RefundAmount,
			SupplementAmount: result.SupplementAmount,
		}
		snapshot := GenerateBillingSnapshot(pricing, breakdown, nil, internalSettlement, nil)
		snapshot.RequestMeta.RequestedModel = modelName
		result.BillingSnapshot = SnapshotToJSON(snapshot)
		result.BillingSummary = GenerateBillingSummary(snapshot)
		result.BillingMode = pricing.BillingMode
		result.BillingSource = pricing.BillingSource
		result.RateMultiplier = pricing.DiscountRatio
	}

	// 7. 记录消费流水（事务外，best-effort）
	recordTransaction(ctx, wallet.ID, tenantID, "consume", -actualCost,
		fmt.Sprintf("consume: %s model=%s pre_deduct=%.6f actual=%.6f", requestID, modelName, preDeductAmount, actualCost),
		userID, requestID, modelName, taskBillingID, "billing_record", 0, apiKeyID, taskID, balanceAfter, frozenAfter)

	return result, nil
}

// buildTaskCostBreakdown 构建任务计费的 CostBreakdown
func buildTaskCostBreakdown(pricing *PricingResult, actualCost float64, totalTokens, _ int) *CostBreakdown {
	if pricing == nil {
		return &CostBreakdown{
			TotalCost: actualCost,
			BaseCost:  actualCost,
			Currency:  "USD",
		}
	}

	bd := &CostBreakdown{
		BillingMode:      pricing.BillingMode,
		PerRequestPrice:  pricing.PerRequestPrice,
		DiscountRatio:    pricing.DiscountRatio,
		TenantMultiplier: pricing.TenantMultiplier,
		Currency:         pricing.Currency,
	}

	if pricing.BillingMode == "per_request" {
		bd.BaseCost = pricing.PerRequestPrice
		bd.TotalCost = actualCost
		return bd
	}

	// token 模式
	bd.OutputTokens = totalTokens
	bd.OutputCost = actualCost
	bd.TotalCost = actualCost

	tenantMul := pricing.TenantMultiplier
	if tenantMul > 0 {
		bd.BaseCost = actualCost / tenantMul
	} else {
		bd.BaseCost = actualCost
	}

	return bd
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
		ok, err := PreDeduct(ctx, tenantID, diff, requestID+"_adjust", "")
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
// 公式：totalTokens / 1M × output_price × tenant_multiplier × 附加比率
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
	// 注意：跳过 duration/resolution，它们已体现在上游返回的 token 数中，不应再乘
	for k, ratio := range ratios {
		if k == "duration" || k == "resolution" {
			continue
		}
		cost *= ratio
	}

	// 最低消费
	if cost < 0.01 {
		cost = 0.01
	}

	return cost, nil
}
