package billing

import (
	"context"
	"database/sql"
	"errors"
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
	return estimateTaskCost(pricing, ratios), nil
}

// imagePlaceholderPreDeduct 图片等按次任务**未配置按次单价**时的占位预扣（USD）。
// 图片提交时拿不到 token 用量、无法精确估价，故先按此小额占位冻结，结算阶段再按上游
// 返回的真实 token 用量多退少补。仅用于「无有效定价」的兜底，不覆盖已配置的真实按次单价。
const imagePlaceholderPreDeduct = 0.1

// estimateTaskCost 纯函数：根据定价与计费比率估算任务预扣费用，不依赖数据库/缓存，便于单测。
//
// 计费口径按任务类型分流：
//   - per_request（按次计费，图片/音乐等）：直接取按次单价；
//   - 时长类任务（视频生成，ratios 携带 duration/resolution 信号）：按
//     10000 tokens/s × duration × resolution 预估 token 再乘输出单价；
//   - 其余无时长信号的任务（如未显式配成 per_request 的图片模型）：退回按次单价，
//     **不再**套用视频 token 估算——图片没有时长/分辨率，套 10000×5×2.25 会凭空估出
//     11.25 万 token 的天价预扣（$30/1M 输出价即得 $3.375），且与结算的「0 token」自相矛盾。
func estimateTaskCost(pricing *PricingResult, ratios map[string]float64) float64 {
	if pricing == nil {
		return 0.01
	}

	var cost float64
	switch {
	case pricing.BillingMode == "per_request":
		// 按次计费：直接用单价
		cost = pricing.PerRequestPrice
	case pricing.OutputPrice > 0 && hasDurationSignal(ratios):
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
	default:
		// 无时长信号（图片等扁平计费任务）：优先按次单价；未配按次价时用占位预扣，
		// 绝不走视频 token 估算。结算阶段再按上游真实 token 用量多退少补
		// （见 sync_image_worker.settleSyncImageSuccess）。
		if pricing.PerRequestPrice > 0 {
			cost = pricing.PerRequestPrice
		} else {
			cost = imagePlaceholderPreDeduct
		}
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
	return cost
}

// hasDurationSignal 判断计费比率里是否携带时长类任务（视频）的 duration/resolution 信号。
// 视频提交管线（VolcengineVideoAdaptor.EstimateBilling）必然写入这两个键；图片等同步/异步
// 任务传 nil ratios，据此区分「该走视频 token 估算」还是「按次计费」。
func hasDurationSignal(ratios map[string]float64) bool {
	if ratios == nil {
		return false
	}
	if _, ok := ratios["duration"]; ok {
		return true
	}
	_, ok := ratios["resolution"]
	return ok
}

// PreDeductTask 预扣任务费用
func (b *TaskBillingProviderImpl) PreDeductTask(ctx context.Context, tenantID int64, requestID string, estimatedCost float64, modelName string) (float64, error) {
	ok, err := PreDeduct(ctx, tenantID, estimatedCost, requestID, modelName)
	if !ok {
		if err == nil {
			return 0, fmt.Errorf("pre-deduct task failed: insufficient balance")
		}
		return 0, fmt.Errorf("pre-deduct task failed: %w", err)
	}
	return estimatedCost, nil
}

func (b *TaskBillingProviderImpl) CheckRateLimit(ctx context.Context, tenantID, userID, apiKeyID int64, keyQPS int) (bool, string, int, int, int64) {
	result := CheckRateLimitWithKeyLimit(ctx, LoadRateLimitConfig(ctx), tenantID, userID, apiKeyID, keyQPS)
	return result.Allowed, result.LimitLevel, result.Limit, result.Remaining, result.ResetAt
}

func (b *TaskBillingProviderImpl) AcquireApiKeyConcurrent(ctx context.Context, apiKeyID int64, limit int) bool {
	return AcquireApiKeyConcurrent(ctx, apiKeyID, limit)
}

func (b *TaskBillingProviderImpl) ReleaseApiKeyConcurrent(ctx context.Context, apiKeyID int64) {
	ReleaseApiKeyConcurrent(ctx, apiKeyID)
}

func (b *TaskBillingProviderImpl) CheckApiKeyQuota(ctx context.Context, apiKeyID int64, preDeductAmount float64) error {
	return CheckApiKeyQuota(ctx, apiKeyID, preDeductAmount)
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

	// 3. 事务内执行结算（钱包扣款 + 计费记录 + 流水 + tracks 状态）
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
			SettledAt:    gtime.NewFromTime(time.Now()),
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
			if isDuplicateKeyErr(err) {
				// 同一 request_id 已结算：整个事务回滚（3a 钱包扣款一并撤销），避免重复扣款/重复账单
				return errAlreadySettled
			}
			return fmt.Errorf("settle task: create billing record: %w", err)
		}
		if billingResult != nil {
			taskBillingID, _ = billingResult.LastInsertId()
		}

		// 3d. 记录消费流水（事务内）
		_, err = tx.Model("bil_transactions").Ctx(ctx).Data(do.BilTransactions{
			TenantId:     tenantID,
			WalletId:     wallet.ID,
			Type:         "consume",
			Amount:       -actualCost,
			BalanceAfter: balanceAfter,
			FrozenAfter:  frozenAfter,
			RelatedId:    taskBillingID,
			RelatedType:  "billing_record",
			Description:  fmt.Sprintf("consume: %s model=%s pre_deduct=%.6f actual=%.6f", requestID, modelName, preDeductAmount, actualCost),
			UserId:       userID,
			RequestId:    requestID,
			ModelName:    modelName,
			ProjectId:    0,
			ApiKeyId:     apiKeyID,
			TaskId:       taskID,
		}).Insert()
		if err != nil {
			return fmt.Errorf("settle task: record transaction: %w", err)
		}

		// 3e. 标记预扣追踪记录为已结算（事务内）
		// 同时覆盖 requestID 与 requestID+"_adjust"：步骤 3a 已按总预扣额（含 AdjustTaskBilling
		// 补扣产生的 _adjust 冻结）一次性释放，两条追踪记录都应随之置为 settled；
		// 否则残留的 _adjust frozen 追踪会被日对账判为不一致，并被孤儿清理二次释放。
		_, err = tx.Ctx(ctx).Exec(
			"UPDATE bil_prededuct_tracks SET status = 'settled' WHERE request_id IN ($1, $2) AND status = 'frozen'",
			requestID, requestID+"_adjust")
		if err != nil {
			return fmt.Errorf("settle task: mark prededuct settled: %w", err)
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, errAlreadySettled) {
			// 幂等跳过：该任务此前已结算完成，本次为重复调用（轮询/重放），不再扣款/写账单
			g.Log().Warningf(ctx, "settle task: duplicate settlement skipped for request=%s (idempotent)", requestID)
			return &common.SettlementResult{
				PreDeductAmount: preDeductAmount,
				ActualCost:      actualCost,
				BaseCost:        breakdown.BaseCost,
				TotalCost:       actualCost,
				OutputCost:      breakdown.OutputCost,
			}, nil
		}
		return nil, err
	}

	// 4. 清除缓存（事务提交后）
	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
	InvalidateWalletRedis(ctx, tenantID)
	CleanupPreDeduct(ctx, tenantID, requestID)
	CleanupPreDeduct(ctx, tenantID, requestID+"_adjust")

	// 5. 异步检查余额预警
	go CheckBalanceWarning(context.Background(), tenantID)

	// 6. 差额已在步骤 3a 一次性结清，此处不得再做任何解冻/退款：
	//    步骤 3a 已 frozen_balance -= preDeductAmount（释放全部预扣冻结）、balance -= actualCost
	//    （只扣真实成本），无论 actualCost 大于还是小于预扣，可用余额都已精确调整到位
	//    （available 变化量恰为 preDeductAmount - actualCost）。
	//    切勿再对 requestID+"_adjust" 调 UnfreezePreDeduct/SettleFailed——那会在步骤 3a 之外
	//    二次释放从未单独冻结过的金额，导致 frozen_balance 被过度释放（Redis 侧无下限时甚至为负）。

	// 7. 生成计费快照 + 摘要
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

	// token 模式但没有真实 token 用量（图片等扁平计费任务）：把费用整体记为 BaseCost，
	// 不摊进 OutputCost。否则快照会生成「0 token 却有 output 费用」的自相矛盾行
	// （如 0 tokens × $30/1M = $3.375）。真实费用仍由结算的 actual_cost 体现。
	if totalTokens <= 0 {
		bd.BaseCost = actualCost
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

func (b *TaskBillingProviderImpl) IncrApiKeyQuotaUsed(ctx context.Context, apiKeyID int64, amount float64) {
	IncrApiKeyQuotaUsed(ctx, apiKeyID, amount)
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
