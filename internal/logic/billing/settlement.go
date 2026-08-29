package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	rcommon "github.com/qianfree/team-api/relay/common"
)

// errAlreadySettled 结算幂等哨兵：当同一 request_id 的计费记录已存在（bil_records 唯一约束冲突）时，
// 骨架在任何资金变动之前返回该错误，调用方据此识别为幂等空操作，不重复扣款、不重复写账单。
// 必须原样返回（不可 gerror.Wrap），以保证 errors.Is 能识别。
var errAlreadySettled = errors.New("billing: request already settled (idempotent skip)")

// isDuplicateKeyErr 判断 error 是否为 PostgreSQL 唯一约束冲突（SQLSTATE 23505）。
// 结算时 bil_records.request_id 唯一索引会拒绝同一请求的第二次插入，据此把重复结算识别为
// 幂等冲突。跨驱动（lib/pq、pgx）统一走错误文案匹配，避免耦合具体驱动的错误类型。
func isDuplicateKeyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key value") ||
		strings.Contains(msg, "23505") ||
		strings.Contains(msg, "uk_bil_records_request")
}

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
	DuplicateSkip    bool           // 幂等跳过：本次为重复结算（未扣款未写账单），调用方不得再累加额度
}

// calcSettlementDiff 计算预扣与实际费用的差额（decimal 精确运算）。
// 返回 (退款金额, 补扣金额)，两者互斥且均为非负。
func calcSettlementDiff(preDeductAmount, actualCost float64) (refundAmt, supplementAmt float64) {
	diffD := SubtractMoney(NewFromFloat(preDeductAmount), NewFromFloat(actualCost))
	if diffD.GreaterThan(Zero) {
		refundAmt = InexactFloat64(diffD)
	} else if diffD.LessThan(Zero) {
		supplementAmt = InexactFloat64(diffD.Neg())
	}
	return
}

// settlementTxParams 结算参数：三处结算共用的骨架的可变部分。
type settlementTxParams struct {
	tenantID        int64
	walletID        int64   // 钱包 ID（仅用于流水记录，骨架不再更新钱包行）
	preDeductAmount float64 // 预扣金额（用于与 Redis 实际认领金额比对告警）
	actualCost      float64 // 实际扣款金额（SettleClaim 从 Redis 余额扣除）
	logPrefix       string  // 错误信息前缀（settle / settle_with_usage / settle task）
	// createBillingRecord 创建计费记录并返回其 ID（幂等闸门）；
	// 唯一冲突（同一 request_id 已结算）时返回的 error 由骨架识别为 errAlreadySettled。
	createBillingRecord func(ctx context.Context) (int64, error)
	// buildTransaction 根据计费记录 ID 与 Redis 返回的余额快照构造消费流水。
	buildTransaction func(billingID int64, balanceAfter, frozenAfter float64) do.BilTransactions
	// predeductRequestIDs 本次结算需认领的预扣 request_id（task 结算含 "_adjust"）。
	predeductRequestIDs []string
}

// executeSettlement 结算公共骨架：幂等闸门 → Redis 认领扣款 → 记流水，三步顺序执行。
// Settle / SettleWithUsage / SettleTaskSuccess 三处共用，差异部分（计费记录构造、
// 流水构造、预扣认领 request_id 集合）由 params 注入。
//
// Redis 权威化架构下结算不再触碰 bil_wallets 行（消除钱包热点行锁）：
//  1. 幂等闸门先行：bil_records.request_id 唯一约束在任何资金变动之前拒绝重复结算；
//  2. SettleClaim 是资金提交点：Lua 原子完成「认领预扣 hash → 释放冻结 → 扣减余额」，
//     认领金额取实际删掉的预扣 hash 之和——预扣已被解冻/过期/丢失时认领不到（0），
//     对应冻结早已释放，只扣 balance 不再动 frozen，杜绝重复释放；
//  3. 流水 balance_after/frozen_after 取 Lua 返回值（Redis 权威快照），账本链连续。
//
// 崩溃窗口补偿：第 2/3 步失败时按「无扣款即无账单」回滚——删除已创建的计费记录；
// 第 3 步失败另需逆转 Redis 扣款（余额退回、冻结不恢复：请求已结束，语义为本单免费）。
// 补偿自身失败时由日对账「bil_records 无 consume 流水」探测项兜底发现。
func executeSettlement(ctx context.Context, p settlementTxParams) (int64, error) {
	actualCostD := NewFromFloat(p.actualCost)

	// a. 创建计费记录（幂等闸门：同 request_id 的重复结算在触碰资金之前即被拒绝）
	billingID, err := p.createBillingRecord(ctx)
	if err != nil {
		if isDuplicateKeyErr(err) {
			return 0, errAlreadySettled
		}
		return 0, gerror.Wrapf(err, "%s: create billing record", p.logPrefix)
	}

	// b. Redis 认领预扣 + 扣款（资金提交点）
	claimedD, balanceAfterD, frozenAfterD, err := SettleClaim(ctx, p.tenantID, actualCostD, p.predeductRequestIDs)
	if err != nil {
		if delErr := deleteBillingRecord(ctx, billingID); delErr != nil {
			g.Log().Errorf(ctx, "%s: compensate delete billing record %d failed: %v", p.logPrefix, billingID, delErr)
		}
		return 0, gerror.Wrapf(err, "%s: settle claim", p.logPrefix)
	}
	if !claimedD.Equal(NewFromFloat(p.preDeductAmount)) {
		g.Log().Warningf(ctx, "%s: claimed frozen %.6f != pre-deduct %.6f (prededuct released/expired before settle?) requests=%v",
			p.logPrefix, InexactFloat64(claimedD), p.preDeductAmount, p.predeductRequestIDs)
	}

	// c. 记录消费流水
	_, err = dao.BilTransactions.Ctx(ctx).Data(p.buildTransaction(billingID,
		InexactFloat64(balanceAfterD), InexactFloat64(frozenAfterD))).Insert()
	if err != nil {
		// 逆转扣款 + 删除计费记录，回到「未结算」状态（预扣已随认领释放，本单免费）
		if _, _, creditErr := CreditWalletRedis(ctx, p.tenantID, actualCostD); creditErr != nil {
			g.Log().Errorf(ctx, "%s: compensate reverse charge failed: tenant=%d request=%v cost=%.6f: %v",
				p.logPrefix, p.tenantID, p.predeductRequestIDs, p.actualCost, creditErr)
		}
		if delErr := deleteBillingRecord(ctx, billingID); delErr != nil {
			g.Log().Errorf(ctx, "%s: compensate delete billing record %d failed: %v", p.logPrefix, billingID, delErr)
		}
		return 0, gerror.Wrapf(err, "%s: record transaction", p.logPrefix)
	}

	return billingID, nil
}

// deleteBillingRecord 删除计费记录（结算补偿用：记录创建于数秒前、无任何引用，物理删除保持账表干净）
func deleteBillingRecord(ctx context.Context, billingID int64) error {
	_, err := dao.BilRecords.Ctx(ctx).Where("id", billingID).Delete()
	return err
}

// Settle 结算请求费用
// 预扣→调用→结算→退差额/补扣
// billAt 为请求受理时刻（时段定价按该时刻评估；零值按当前时刻兜底）
func Settle(ctx context.Context, tenantID, userID, apiKeyID, channelID int64,
	modelName, requestID, relayMode string,
	inputTokens, outputTokens int,
	preDeductAmount float64, projectID int64, billAt time.Time) (*SettlementResult, error) {

	// 1. 定价与实际费用：按受理时刻一次性取价（时段定价预扣/结算口径一致）
	pricingResult, pricingErr := GetModelPriceAt(ctx, tenantID, modelName, billAt)
	var breakdown *CostBreakdown
	if pricingErr != nil {
		// A4 修复：计价失败【不得】按零费用结算——那会把定价异常/模型未配价/短暂 DB 故障
		// 都变成免费请求。改为 fail-closed 兜底：按已冻结的预扣额计费（预扣是请求受理时的估价，
		// 当前可得的最佳估值），与 task 结算路径（async_polling / sync_image_worker 默认
		// actualCost = PreDeductAmount）保持一致。保留 token 数便于账单核对。
		g.Log().Errorf(ctx, "settle: calculate cost failed for %s (model=%s), fallback to pre-deduct estimate %.6f: %v",
			requestID, modelName, preDeductAmount, pricingErr)
		breakdown = &CostBreakdown{
			TotalCost:    preDeductAmount,
			BaseCost:     preDeductAmount,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			Currency:     Currency(ctx),
		}
	} else {
		breakdown = computeCost(pricingResult, inputTokens, outputTokens, nil)
	}
	actualCost := breakdown.TotalCost

	// 2. 计算差额（decimal 精确运算，float64 仅在结果出口转换）
	refundAmt, supplementAmt := calcSettlementDiff(preDeductAmount, actualCost)

	// 3. 获取钱包
	walletID, err := GetWalletID(ctx, tenantID)
	if err != nil {
		return nil, gerror.Wrapf(err, "settle: get wallet")
	}

	// 4. 定价信息已在第 1 步随费用计算一并获取（事务外只读）

	// 5. 执行结算（幂等闸门 → Redis 认领扣款 → 流水）
	billingID, err := executeSettlement(ctx, settlementTxParams{
		tenantID:        tenantID,
		walletID:        walletID,
		preDeductAmount: preDeductAmount,
		actualCost:      actualCost,
		logPrefix:       "settle",
		createBillingRecord: func(ctx context.Context) (int64, error) {
			var inputSnapPrice, outputSnapPrice float64
			var billingMode string
			var discountRatio, billingInputMult, billingOutputMult float64
			if pricingResult != nil {
				inputSnapPrice = pricingResult.InputPrice * pricingResult.TenantMultiplier
				outputSnapPrice = pricingResult.OutputPrice * pricingResult.TenantMultiplier
				billingMode = pricingResult.BillingMode
				discountRatio = pricingResult.DiscountRatio
				billingInputMult = breakdown.InputMultiplier
				billingOutputMult = breakdown.OutputMultiplier
			}
			return createBillingRecord(ctx, tenantID, userID, apiKeyID, channelID,
				modelName, requestID, relayMode, inputTokens, outputTokens,
				inputSnapPrice, outputSnapPrice, actualCost,
				billingMode, discountRatio, billingInputMult, billingOutputMult)
		},
		buildTransaction: func(billingID int64, balanceAfter, frozenAfter float64) do.BilTransactions {
			return do.BilTransactions{
				TenantId:     tenantID,
				WalletId:     walletID,
				Type:         "consume",
				Amount:       -actualCost,
				BalanceAfter: balanceAfter,
				FrozenAfter:  frozenAfter,
				RelatedId:    billingID,
				RelatedType:  "billing_record",
				Description:  fmt.Sprintf("consume: %s model=%s input=%d output=%d pre_deduct=%.6f actual=%.6f", requestID, modelName, inputTokens, outputTokens, preDeductAmount, actualCost),
				UserId:       userID,
				RequestId:    requestID,
				ModelName:    modelName,
				ProjectId:    projectID,
				ApiKeyId:     apiKeyID,
			}
		},
		predeductRequestIDs: []string{requestID},
	})
	if err != nil {
		if errors.Is(err, errAlreadySettled) {
			// 幂等跳过：该请求此前已结算完成，本次为重复调用，不再扣款/写账单
			g.Log().Warningf(ctx, "settle: duplicate settlement skipped for request=%s (idempotent)", requestID)
			return &SettlementResult{
				PreDeductAmount:  preDeductAmount,
				ActualCost:       actualCost,
				BaseCost:         breakdown.BaseCost,
				RefundAmount:     refundAmt,
				SupplementAmount: supplementAmt,
				CostBreakdown:    breakdown,
				DuplicateSkip:    true,
			}, nil
		}
		return nil, err
	}

	// 6. 异步检查余额预警（GetWallet 读 Redis 权威值，实时准确）
	go CheckBalanceWarning(context.Background(), tenantID)

	return &SettlementResult{
		PreDeductAmount:  preDeductAmount,
		BaseCost:         breakdown.BaseCost,
		ActualCost:       actualCost,
		RefundAmount:     refundAmt,
		SupplementAmount: supplementAmt,
		BillingRecordID:  billingID,
	}, nil
}

// SettleWithUsage 完整 Usage 结算（含 cache token 计费 + 计费快照）。
// 时段定价按 relayInfo.StartTime（请求受理时刻）评估，跨时段边界的请求不因结算延迟变价。
func SettleWithUsage(ctx context.Context, tenantID, userID, apiKeyID, channelID int64,
	modelName, requestID, relayMode string,
	usage *rcommon.Usage, preDeductAmount float64, relayInfo *rcommon.RelayInfo) (*SettlementResult, error) {

	// 1. 定价与实际费用：按受理时刻一次性取价后用完整 Usage 计算（含 cache token）。
	// 此前 CalculateCostWithUsage 与第 4 步各取一次定价，两次结果可能不一致（缓存刷新窗口），已合并。
	var billAt time.Time
	if relayInfo != nil {
		billAt = relayInfo.StartTime
	}
	pricingResult, pricingErr := GetModelPriceAt(ctx, tenantID, modelName, billAt)
	var breakdown *CostBreakdown
	if pricingErr != nil || usage == nil {
		// A4 修复：计价失败 fail-closed 兜底按预扣额计费，而非零费用（免费请求）。见 Settle 同段说明。
		if pricingErr != nil {
			g.Log().Errorf(ctx, "settle_with_usage: calculate cost failed for %s (model=%s), fallback to pre-deduct estimate %.6f: %v",
				requestID, modelName, preDeductAmount, pricingErr)
		} else {
			g.Log().Errorf(ctx, "settle_with_usage: usage is nil for %s (model=%s), fallback to pre-deduct estimate %.6f",
				requestID, modelName, preDeductAmount)
		}
		fb := &CostBreakdown{
			TotalCost: preDeductAmount,
			BaseCost:  preDeductAmount,
			Currency:  Currency(ctx),
		}
		if usage != nil {
			fb.InputTokens = usage.PromptTokens
			fb.OutputTokens = usage.CompletionTokens
		}
		breakdown = fb
	} else {
		breakdown = computeCost(pricingResult, usage.PromptTokens, usage.CompletionTokens, usage)
	}
	actualCost := breakdown.TotalCost

	// 2. 计算差额（decimal 精确运算，float64 仅在结果出口转换）
	refundAmt, supplementAmt := calcSettlementDiff(preDeductAmount, actualCost)

	// 3. 获取钱包
	walletID, err := GetWalletID(ctx, tenantID)
	if err != nil {
		return nil, gerror.Wrapf(err, "settle_with_usage: get wallet")
	}

	// 4. 定价信息已在第 1 步随费用计算一并获取（事务外只读）

	// 5. 执行结算（幂等闸门 → Redis 认领扣款 → 流水）
	billingID, err := executeSettlement(ctx, settlementTxParams{
		tenantID:        tenantID,
		walletID:        walletID,
		preDeductAmount: preDeductAmount,
		actualCost:      actualCost,
		logPrefix:       "settle_with_usage",
		createBillingRecord: func(ctx context.Context) (int64, error) {
			var inputSnapPrice, outputSnapPrice float64
			var billingMode string
			var discountRatio, billingInputMult, billingOutputMult float64
			if pricingResult != nil {
				inputSnapPrice = pricingResult.InputPrice * pricingResult.TenantMultiplier
				outputSnapPrice = pricingResult.OutputPrice * pricingResult.TenantMultiplier
				billingMode = pricingResult.BillingMode
				discountRatio = pricingResult.DiscountRatio
				billingInputMult = breakdown.InputMultiplier
				billingOutputMult = breakdown.OutputMultiplier
			}
			return createBillingRecordWithSnapshot(ctx, tenantID, userID, apiKeyID, channelID,
				modelName, requestID, relayMode, breakdown,
				inputSnapPrice, outputSnapPrice, actualCost,
				billingMode, discountRatio, billingInputMult, billingOutputMult, pricingResult)
		},
		buildTransaction: func(billingID int64, balanceAfter, frozenAfter float64) do.BilTransactions {
			var txProjectID int64
			if relayInfo != nil {
				txProjectID = relayInfo.ProjectID
			}
			return do.BilTransactions{
				TenantId:     tenantID,
				WalletId:     walletID,
				Type:         "consume",
				Amount:       -actualCost,
				BalanceAfter: balanceAfter,
				FrozenAfter:  frozenAfter,
				RelatedId:    billingID,
				RelatedType:  "billing_record",
				Description:  fmt.Sprintf("consume: %s model=%s input=%d output=%d pre_deduct=%.6f actual=%.6f", requestID, modelName, breakdown.InputTokens, breakdown.OutputTokens, preDeductAmount, actualCost),
				UserId:       userID,
				RequestId:    requestID,
				ModelName:    modelName,
				ProjectId:    txProjectID,
				ApiKeyId:     apiKeyID,
			}
		},
		predeductRequestIDs: []string{requestID},
	})
	if err != nil {
		if errors.Is(err, errAlreadySettled) {
			// 幂等跳过：该请求此前已结算完成，本次为重复调用，不再扣款/写账单
			g.Log().Warningf(ctx, "settle_with_usage: duplicate settlement skipped for request=%s (idempotent)", requestID)
			return &SettlementResult{
				PreDeductAmount:  preDeductAmount,
				ActualCost:       actualCost,
				BaseCost:         breakdown.BaseCost,
				RefundAmount:     refundAmt,
				SupplementAmount: supplementAmt,
				CostBreakdown:    breakdown,
				DuplicateSkip:    true,
			}, nil
		}
		return nil, err
	}

	// 6. 异步检查余额预警（GetWallet 读 Redis 权威值，实时准确）
	go CheckBalanceWarning(context.Background(), tenantID)

	settlementResult := &SettlementResult{
		PreDeductAmount:  preDeductAmount,
		ActualCost:       actualCost,
		BaseCost:         breakdown.BaseCost,
		RefundAmount:     refundAmt,
		SupplementAmount: supplementAmt,
		BillingRecordID:  billingID,
		CostBreakdown:    breakdown,
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

	return settlementResult, nil
}

// SettleFailed 失败请求结算：退还预扣金额
func SettleFailed(ctx context.Context, tenantID int64, requestID string, preDeductAmount float64) error {
	if preDeductAmount <= 0 {
		return nil
	}

	// 解冻金额由 Redis 预扣 hash 决定（认领即删，恰好一次），不信任调用方传入的估算值
	_, err := UnfreezePreDeduct(ctx, tenantID, requestID)
	return err
}

// SettleStreamInterrupted 流式中断结算：按已确认 usage 走完整 Usage 结算（含 cache token 计费 + 快照）。
// 此前按 (input, output) 两数走 Settle，会丢弃 usage 中的 cache_read / cache_creation token——
// Claude 场景 prompt_tokens 不含 cache token，大缓存请求中断时缓存部分完全漏计费。
// billAt 为请求受理时刻（写入合成 RelayInfo 供时段定价评估；零值按当前时刻兜底）。
func SettleStreamInterrupted(ctx context.Context, tenantID, userID, apiKeyID, channelID int64,
	modelName, requestID, relayMode string,
	usage *rcommon.Usage, preDeductAmount float64, projectID int64, billAt time.Time) (*SettlementResult, error) {

	// usage 为 nil 必须兜换空 Usage：SettleWithUsage 对 nil usage 会 fail-closed 按预扣全额计费，
	// 而中断且无任何 usage 的正确语义是 0 token 结算（成本 0，全额退差）
	if usage == nil {
		usage = &rcommon.Usage{}
	}
	relayInfo := &rcommon.RelayInfo{
		ProjectID:       projectID,
		OriginModelName: modelName,
		IsStream:        true,
		StartTime:       billAt,
	}
	return SettleWithUsage(ctx, tenantID, userID, apiKeyID, channelID,
		modelName, requestID, relayMode, usage, preDeductAmount, relayInfo)
}

// createBillingRecord 创建计费记录（含快照字段）。依赖调用方传入携带事务的 ctx，内部用 dao.Xxx.Ctx(ctx) 传播事务
func createBillingRecord(ctx context.Context, tenantID, userID, apiKeyID, channelID int64,
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
		Currency:     Currency(ctx),
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

	result, err := dao.BilRecords.Ctx(ctx).Data(data).Insert()
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

// createBillingRecordWithSnapshot 创建计费记录（含 cache token 和完整快照）。依赖调用方传入携带事务的 ctx
func createBillingRecordWithSnapshot(ctx context.Context, tenantID, userID, apiKeyID, channelID int64,
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
		Currency:     Currency(ctx),
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

	result, err := dao.BilRecords.Ctx(ctx).Data(data).Insert()
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}
