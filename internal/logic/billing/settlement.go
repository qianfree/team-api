package billing

import (
	"context"
	"fmt"
	do "github.com/qianfree/team-api/internal/model/do"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	rcommon "github.com/qianfree/team-api/relay/common"
)

// SettlementResult 结算结果
type SettlementResult struct {
	PreDeductAmount  float64        // 预扣金额
	BaseCost         float64        // 基础费用（应用租户折扣前）
	ActualCost       float64        // 实际费用（应用折扣后）
	RefundAmount     float64        // 退款金额（预扣 - 实际，正数）
	SupplementAmount float64        // 补扣金额（实际 - 预扣，正数）
	BillingRecordID  int64          // 计费记录 ID
	BillingSnapshot  string         // 计费快照 JSON
	BillingSummary   string         // 计费摘要文本
	CostBreakdown    *CostBreakdown // 费用明细
	BillingMode      string         // 计费模式
	BillingSource    string         // 定价来源
	RateMultiplier   float64        // 费率倍率
	PlanID           int64          // 扣费套餐 ID
	PlanDeduction    float64        // 套餐扣费金额
	WalletDeduction  float64        // 钱套餐扣费金额
}

// Settle 结算请求费用
// 预扣→调用→结算→退差额/补扣
func Settle(ctx context.Context, tenantID, userID, apiKeyID, channelID int64,
	modelName, requestID, relayMode string,
	inputTokens, outputTokens int,
	preDeductAmount float64, projectID int64) (*SettlementResult, error) {

	// 1. 计算实际费用
	breakdown, err := CalculateCost(ctx, tenantID, modelName, inputTokens, outputTokens)
	if err != nil {
		g.Log().Errorf(ctx, "settle: calculate cost failed for %s: %v", requestID, err)
		breakdown = &CostBreakdown{TotalCost: 0, Currency: "USD"}
	}
	actualCost := breakdown.TotalCost

	// 2. 读取预扣拆分（包/钱包）
	pkgPreDeducted, walletPreDeducted, packageID := GetPreDeductSplit(ctx, requestID)
	// 兼容旧流程（无包扣费时，全走钱包）
	if pkgPreDeducted == 0 && walletPreDeducted == 0 {
		walletPreDeducted = preDeductAmount
	}

	// 3. 计算包/钱包各自的实际扣费
	var pkgActual, walletActual float64
	if pkgPreDeducted > 0 {
		pkgActual = actualCost
		if pkgActual > pkgPreDeducted {
			pkgActual = pkgPreDeducted
		}
		walletActual = actualCost - pkgActual
	} else {
		walletActual = actualCost
	}

	// 4. 计算差额
	var refundAmt, supplementAmt float64
	if preDeductAmount > actualCost {
		refundAmt = preDeductAmount - actualCost
	} else if actualCost > preDeductAmount {
		supplementAmt = actualCost - preDeductAmount
	}

	// 5. 获取钱包
	wallet, err := GetWallet(ctx, tenantID)
	if err != nil {
		return nil, gerror.Wrapf(err, "settle: get wallet")
	}

	// 6. 获取定价信息（事务外只读）
	pricingResult, _ := GetModelPrice(ctx, tenantID, modelName)

	// 7. 事务内执行结算
	var balanceAfter, frozenAfter float64
	var billingID int64
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 7a. 结算包扣费（退还差额到 remaining_credits）
		if pkgPreDeducted > 0 && packageID > 0 {
			SettlePlanDeduction(ctx, tx, packageID, pkgPreDeducted, pkgActual)
		}

		// 7b. 更新钱包（仅 wallet 部分）
		if walletPreDeducted > 0 {
			now := time.Now()
			_, err := tx.Ctx(ctx).Exec(
				"UPDATE bil_wallets SET frozen_balance = GREATEST(frozen_balance - ?, 0), balance = balance - ?, updated_at = ? WHERE id = ?",
				walletPreDeducted, walletActual, now, wallet.ID)
			if err != nil {
				return gerror.Wrapf(err, "settle: update wallet")
			}
		}

		// 7c. 事务内读取准确余额
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

		// 7d. 创建计费记录
		var inputSnapPrice, outputSnapPrice float64
		var billingMode string
		var discountRatio, billingInputMult, billingOutputMult float64
		if pricingResult != nil {
			inputSnapPrice = pricingResult.InputPrice * pricingResult.ModelMultiplier * pricingResult.TenantMultiplier
			outputSnapPrice = pricingResult.OutputPrice * pricingResult.ModelMultiplier * pricingResult.TenantMultiplier
			billingMode = pricingResult.BillingMode
			discountRatio = pricingResult.DiscountRatio
			billingInputMult = breakdown.InputMultiplier
			billingOutputMult = breakdown.OutputMultiplier
		}

		billingID, err = createBillingRecord(ctx, tx, tenantID, userID, apiKeyID, channelID,
			modelName, requestID, relayMode, inputTokens, outputTokens,
			inputSnapPrice, outputSnapPrice, actualCost,
			billingMode, discountRatio, billingInputMult, billingOutputMult)
		if err != nil {
			return gerror.Wrapf(err, "settle: create billing record")
		}

		// 更新计费记录的包扣费字段
		if packageID > 0 || pkgActual > 0 || walletActual > 0 {
			_, err = tx.Ctx(ctx).Exec(
				"UPDATE bil_records SET tenant_plan_id = ?, plan_deduction = ?, wallet_deduction = ? WHERE id = ?",
				packageID, pkgActual, walletActual, billingID)
			if err != nil {
				g.Log().Warningf(ctx, "settle: update billing record plan fields failed: %v", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 8. 清除缓存（事务提交后）
	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
	InvalidateWalletRedis(ctx, tenantID)
	CleanupPreDeduct(ctx, tenantID, requestID)
	markPredeductSettled(ctx, requestID)
	if packageID > 0 {
		InvalidatePlanCache(ctx, tenantID)
	}

	// 9. 记录消费流水（事务外，best-effort）
	recordTransaction(ctx, wallet.ID, tenantID, "consume", -actualCost,
		fmt.Sprintf("consume: %s model=%s input=%d output=%d pre_deduct=%.6f actual=%.6f", requestID, modelName, inputTokens, outputTokens, preDeductAmount, actualCost),
		userID, requestID, modelName, billingID, "billing_record", projectID, apiKeyID, "", balanceAfter, frozenAfter)

	return &SettlementResult{
		PreDeductAmount:  preDeductAmount,
		BaseCost:         breakdown.BaseCost,
		ActualCost:       actualCost,
		RefundAmount:     refundAmt,
		SupplementAmount: supplementAmt,
		BillingRecordID:  billingID,
		PlanID:           packageID,
		PlanDeduction:    pkgActual,
		WalletDeduction:  walletActual,
	}, nil
}

// SettleWithUsage 完整 Usage 结算（含 cache token 计费 + 计费快照）
func SettleWithUsage(ctx context.Context, tenantID, userID, apiKeyID, channelID int64,
	modelName, requestID, relayMode string,
	usage *rcommon.Usage, preDeductAmount float64, relayInfo *rcommon.RelayInfo) (*SettlementResult, error) {

	// 1. 使用完整 Usage 计算实际费用（含 cache token）
	breakdown, err := CalculateCostWithUsage(ctx, tenantID, modelName, usage)
	if err != nil {
		g.Log().Errorf(ctx, "settle_with_usage: calculate cost failed for %s: %v", requestID, err)
		breakdown = &CostBreakdown{TotalCost: 0, Currency: "USD"}
	}
	actualCost := breakdown.TotalCost

	// 2. 读取预扣拆分（包/钱包）
	pkgPreDeducted, walletPreDeducted, packageID := GetPreDeductSplit(ctx, requestID)
	if pkgPreDeducted == 0 && walletPreDeducted == 0 {
		walletPreDeducted = preDeductAmount
	}

	// 3. 计算包/钱包各自的实际扣费
	var pkgActual, walletActual float64
	if pkgPreDeducted > 0 {
		pkgActual = actualCost
		if pkgActual > pkgPreDeducted {
			pkgActual = pkgPreDeducted
		}
		walletActual = actualCost - pkgActual
	} else {
		walletActual = actualCost
	}

	// 4. 计算差额
	var refundAmt, supplementAmt float64
	if preDeductAmount > actualCost {
		refundAmt = preDeductAmount - actualCost
	} else if actualCost > preDeductAmount {
		supplementAmt = actualCost - preDeductAmount
	}

	// 5. 获取钱包
	wallet, err := GetWallet(ctx, tenantID)
	if err != nil {
		return nil, gerror.Wrapf(err, "settle_with_usage: get wallet")
	}

	// 6. 获取定价信息（事务外只读）
	pricingResult, _ := GetModelPrice(ctx, tenantID, modelName)

	// 7. 事务内执行结算
	var balanceAfter, frozenAfter float64
	var billingID int64
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 7a. 结算包扣费
		if pkgPreDeducted > 0 && packageID > 0 {
			SettlePlanDeduction(ctx, tx, packageID, pkgPreDeducted, pkgActual)
		}

		// 7b. 更新钱包（仅 wallet 部分）
		if walletPreDeducted > 0 {
			now := time.Now()
			_, err := tx.Ctx(ctx).Exec(
				"UPDATE bil_wallets SET frozen_balance = GREATEST(frozen_balance - ?, 0), balance = balance - ?, updated_at = ? WHERE id = ?",
				walletPreDeducted, walletActual, now, wallet.ID)
			if err != nil {
				return gerror.Wrapf(err, "settle_with_usage: update wallet")
			}
		}

		// 7c. 事务内读取准确余额
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

		// 7d. 创建计费记录（含快照）
		var inputSnapPrice, outputSnapPrice float64
		var billingMode string
		var discountRatio, billingInputMult, billingOutputMult float64
		if pricingResult != nil {
			inputSnapPrice = pricingResult.InputPrice * pricingResult.ModelMultiplier * pricingResult.TenantMultiplier
			outputSnapPrice = pricingResult.OutputPrice * pricingResult.ModelMultiplier * pricingResult.TenantMultiplier
			billingMode = pricingResult.BillingMode
			discountRatio = pricingResult.DiscountRatio
			billingInputMult = breakdown.InputMultiplier
			billingOutputMult = breakdown.OutputMultiplier
		}

		billingID, err = createBillingRecordWithSnapshot(ctx, tx, tenantID, userID, apiKeyID, channelID,
			modelName, requestID, relayMode, breakdown,
			inputSnapPrice, outputSnapPrice, actualCost,
			billingMode, discountRatio, billingInputMult, billingOutputMult, pricingResult)
		if err != nil {
			return gerror.Wrapf(err, "settle_with_usage: create billing record")
		}

		// 更新计费记录的包扣费字段
		if packageID > 0 || pkgActual > 0 || walletActual > 0 {
			_, err = tx.Ctx(ctx).Exec(
				"UPDATE bil_records SET tenant_plan_id = ?, plan_deduction = ?, wallet_deduction = ? WHERE id = ?",
				packageID, pkgActual, walletActual, billingID)
			if err != nil {
				g.Log().Warningf(ctx, "settle_with_usage: update billing record plan fields failed: %v", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 8. 清除缓存（事务提交后）
	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
	InvalidateWalletRedis(ctx, tenantID)
	CleanupPreDeduct(ctx, tenantID, requestID)
	markPredeductSettled(ctx, requestID)
	if packageID > 0 {
		InvalidatePlanCache(ctx, tenantID)
	}

	settlementResult := &SettlementResult{
		PreDeductAmount:  preDeductAmount,
		ActualCost:       actualCost,
		BaseCost:         breakdown.BaseCost,
		RefundAmount:     refundAmt,
		SupplementAmount: supplementAmt,
		BillingRecordID:  billingID,
		CostBreakdown:    breakdown,
		PlanID:           packageID,
		PlanDeduction:    pkgActual,
		WalletDeduction:  walletActual,
	}

	var snapshotJSON, summaryText string
	if pricingResult != nil {
		snapshot := GenerateBillingSnapshot(pricingResult, breakdown, usage, settlementResult, relayInfo)
		snapshotJSON = SnapshotToJSON(snapshot)
		summaryText = GenerateBillingSummary(snapshot)
		settlementResult.BillingMode = pricingResult.BillingMode
		settlementResult.BillingSource = pricingResult.BillingSource
		settlementResult.RateMultiplier = pricingResult.DiscountRatio
	}
	if snapshotJSON == "" {
		snapshotJSON = "null"
	}
	settlementResult.BillingSnapshot = snapshotJSON
	settlementResult.BillingSummary = summaryText

	// 9. 从 relayInfo 提取 projectID
	var txProjectID int64
	if relayInfo != nil {
		txProjectID = relayInfo.ProjectID
	}

	// 10. 记录消费流水（事务外，best-effort）
	recordTransaction(ctx, wallet.ID, tenantID, "consume", -actualCost,
		fmt.Sprintf("consume: %s model=%s input=%d output=%d pre_deduct=%.6f actual=%.6f", requestID, modelName, breakdown.InputTokens, breakdown.OutputTokens, preDeductAmount, actualCost),
		userID, requestID, modelName, billingID, "billing_record", txProjectID, apiKeyID, "", balanceAfter, frozenAfter)

	return settlementResult, nil
}

// SettleFailed 失败请求结算：退还预扣金额
func SettleFailed(ctx context.Context, tenantID int64, requestID string, preDeductAmount float64) error {
	// 读取预扣拆分（包/钱包）
	pkgPreDeducted, walletPreDeducted, packageID := GetPreDeductSplit(ctx, requestID)
	if pkgPreDeducted == 0 && walletPreDeducted == 0 {
		walletPreDeducted = preDeductAmount
	}

	// 回滚包扣费
	if pkgPreDeducted > 0 && packageID > 0 {
		RollbackPlanPreDeduct(ctx, tenantID, packageID, pkgPreDeducted)
		InvalidatePlanCache(ctx, tenantID)
	}

	// 解冻钱包预扣金额
	if walletPreDeducted > 0 {
		UnfreezePreDeduct(ctx, tenantID, requestID, walletPreDeducted)
	} else {
		// 兼容旧流程：无拆分记录时解冻全额
		UnfreezePreDeduct(ctx, tenantID, requestID, preDeductAmount)
	}
	markPredeductReleased(ctx, requestID)

	return nil
}

// SettleStreamInterrupted 流式中断结算：按已确认 usage 结算
func SettleStreamInterrupted(ctx context.Context, tenantID, userID, apiKeyID, channelID int64,
	modelName, requestID, relayMode string,
	confirmedInput, confirmedOutput int,
	preDeductAmount float64, projectID int64) (*SettlementResult, error) {

	// 流式中断：按已确认的 token 计费
	return Settle(ctx, tenantID, userID, apiKeyID, channelID,
		modelName, requestID, relayMode,
		confirmedInput, confirmedOutput,
		preDeductAmount, projectID)
}

// createBillingRecord 创建计费记录（含快照字段）
func createBillingRecord(ctx context.Context, tx gdb.TX, tenantID, userID, apiKeyID, channelID int64,
	modelName, requestID, relayMode string,
	inputTokens, outputTokens int,
	inputPrice, outputPrice, totalCost float64,
	billingMode string, discountRatio float64,
	billingInputMult, billingOutputMult float64) (int64, error) {

	now := time.Now()
	data := do.BilRecords{
		TenantId:     tenantID,
		UserId:       userID,
		ApiKeyId:     apiKeyID,
		ChannelId:    channelID,
		ModelName:    modelName,
		RequestId:    requestID,
		RelayMode:    relayMode,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		InputPrice:   inputPrice,
		OutputPrice:  outputPrice,
		TotalCost:    totalCost,
		Currency:     "USD",
		Status:       "settled",
		SettledAt:    gtime.NewFromTime(now),
	}

	// 快照字段
	if billingMode != "" {
		data.BillingMode = billingMode
	}
	if discountRatio > 0 {
		data.DiscountRatio = discountRatio
	}
	data.EffectiveInputPrice = inputPrice
	data.EffectiveOutputPrice = outputPrice
	data.BillingInputMultiplier = billingInputMult
	data.BillingOutputMultiplier = billingOutputMult

	result, err := tx.Model("bil_records").Ctx(ctx).Data(data).Insert()
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// UpdateUsageLogCost 更新用量日志的实际费用
func UpdateUsageLogCost(ctx context.Context, requestID string, totalCost float64, inputTokens, outputTokens int) {
	dao.BilUsageLogs.Ctx(ctx).
		Where("request_id", requestID).
		Data(do.BilUsageLogs{
			TotalCost:    totalCost,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		}).Update()
}

// UpdateUsageLogCostWithSnapshot 更新用量日志的费用、token 明细和计费快照
func UpdateUsageLogCostWithSnapshot(ctx context.Context, requestID string, breakdown *CostBreakdown, totalCost float64, snapshotJSON, summaryText string) {
	data := do.BilUsageLogs{
		TotalCost:           breakdown.BaseCost,
		InputTokens:         breakdown.InputTokens,
		OutputTokens:        breakdown.OutputTokens,
		InputCost:           breakdown.InputCost,
		OutputCost:          breakdown.OutputCost,
		CacheCreationTokens: breakdown.CacheCreationTokens,
		CacheReadTokens:     breakdown.CacheReadTokens,
		CacheCreationCost:   breakdown.CacheCreationCost,
		CacheReadCost:       breakdown.CacheReadCost,
		ActualCost:          totalCost,
		BillingSummary:      summaryText,
	}
	if snapshotJSON != "" {
		data.BillingSnapshot = snapshotJSON
	}
	dao.BilUsageLogs.Ctx(ctx).
		Where("request_id", requestID).
		Data(data).Update()
}

// createBillingRecordWithSnapshot 创建计费记录（含 cache token 和完整快照）
func createBillingRecordWithSnapshot(ctx context.Context, tx gdb.TX, tenantID, userID, apiKeyID, channelID int64,
	modelName, requestID, relayMode string,
	breakdown *CostBreakdown,
	inputPrice, outputPrice, totalCost float64,
	billingMode string, discountRatio float64,
	billingInputMult, billingOutputMult float64,
	pricing *PricingResult) (int64, error) {

	now := time.Now()
	data := do.BilRecords{
		TenantId:     tenantID,
		UserId:       userID,
		ApiKeyId:     apiKeyID,
		ChannelId:    channelID,
		ModelName:    modelName,
		RequestId:    requestID,
		RelayMode:    relayMode,
		InputTokens:  breakdown.InputTokens,
		OutputTokens: breakdown.OutputTokens,
		InputPrice:   inputPrice,
		OutputPrice:  outputPrice,
		TotalCost:    totalCost,
		Currency:     "USD",
		Status:       "settled",
		SettledAt:    gtime.NewFromTime(now),
	}

	if billingMode != "" {
		data.BillingMode = billingMode
	}
	if discountRatio > 0 {
		data.DiscountRatio = discountRatio
	}
	data.EffectiveInputPrice = inputPrice
	data.EffectiveOutputPrice = outputPrice
	data.BillingInputMultiplier = billingInputMult
	data.BillingOutputMultiplier = billingOutputMult

	// Cache token 快照
	data.CacheCreationTokens = breakdown.CacheCreationTokens
	data.CacheReadTokens = breakdown.CacheReadTokens
	data.CacheCreationCost = breakdown.CacheCreationCost
	data.CacheReadCost = breakdown.CacheReadCost

	// 完整倍率快照
	if pricing != nil {
		data.ModelMultiplier = pricing.ModelMultiplier
		data.TenantMultiplier = pricing.TenantMultiplier
		data.BaseInputPrice = pricing.BaseInputPrice
		data.BaseOutputPrice = pricing.BaseOutputPrice
	}

	result, err := tx.Model("bil_records").Ctx(ctx).Data(data).Insert()
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// markPredeductSettled 标记预扣追踪记录为已结算
func markPredeductSettled(ctx context.Context, requestID string) {
	_, err := g.DB().Ctx(ctx).Exec(ctx,
		"UPDATE bil_prededuct_tracks SET status = 'settled' WHERE request_id = $1 AND status = 'frozen'",
		requestID)
	if err != nil {
		g.Log().Warningf(ctx, "[PRE-DEDUCT] mark settled failed: request=%s err=%v", requestID, err)
	}
}

// markPredeductReleased 标记预扣追踪记录为已释放
func markPredeductReleased(ctx context.Context, requestID string) {
	_, err := g.DB().Ctx(ctx).Exec(ctx,
		"UPDATE bil_prededuct_tracks SET status = 'released' WHERE request_id = $1 AND status = 'frozen'",
		requestID)
	if err != nil {
		g.Log().Warningf(ctx, "[PRE-DEDUCT] mark released failed: request=%s err=%v", requestID, err)
	}
}
