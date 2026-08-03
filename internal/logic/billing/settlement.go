package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/internal/dao"
	rcommon "github.com/qianfree/team-api/relay/common"
)

// errAlreadySettled 结算幂等哨兵：当同一 request_id 的计费记录已存在（bil_records 唯一约束冲突）时，
// 从结算事务闭包返回该错误使整个事务回滚（钱包扣款一并撤销），调用方据此识别为幂等空操作，
// 不重复扣款、不重复写账单。必须原样返回（不可 gerror.Wrap），以保证 errors.Is 能识别。
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

// settlementTxParams 结算事务参数：三处结算共用的事务骨架的可变部分。
type settlementTxParams struct {
	tenantID        int64
	walletID        int64   // 钱包 ID
	preDeductAmount float64 // 预扣冻结金额（事务内从 frozen_balance 释放）
	actualCost      float64 // 实际扣款金额（事务内从 balance 扣除）
	logPrefix       string  // 错误信息前缀（settle / settle_with_usage / settle task）
	// createBillingRecord 在事务内创建计费记录并返回其 ID；
	// 唯一冲突（同一 request_id 已结算）时返回的 error 由骨架识别为 errAlreadySettled 并回滚。
	createBillingRecord func(ctx context.Context) (int64, error)
	// buildTransaction 根据事务内读到的准确余额与计费记录 ID 构造消费流水。
	buildTransaction func(billingID int64, balanceAfter, frozenAfter float64) do.BilTransactions
	// predeductRequestIDs 本次结算需置为 settled 的预扣追踪 request_id（task 结算含 "_adjust"）。
	predeductRequestIDs []string
}

// executeSettlementTx 结算事务公共骨架：claim 预扣追踪 → 钱包扣款 → 事务内读准确余额
// → 创建计费记录 → 记录消费流水，五步在同一事务内原子完成。
// Settle / SettleWithUsage / SettleTaskSuccess 三处共用，差异部分（计费记录构造、
// 流水构造、预扣追踪 request_id 集合）由 params 注入。
//
// claim 前置的两点意图：
//  1. 冻结释放额取「实际认领到的 track 金额之和」而非调用方传参——track 已被孤儿清理
//     置 expired（长请求超龄）或已被解冻置 released 时认领不到，对应冻结已释放过，
//     此时只扣 balance 不再动 frozen，杜绝 GREATEST 吞掉其他在途请求冻结额的双重释放；
//  2. 锁序统一为 track→wallet（与 UnfreezePreDeduct、孤儿清理一致），消除死锁隐患。
//
// 幂等：计费记录命中 request_id 唯一约束时整个事务回滚（钱包扣款与 claim 一并撤销），
// 返回 errAlreadySettled，由调用方按幂等空操作处理。
func executeSettlementTx(ctx context.Context, p settlementTxParams) (int64, error) {
	var billingID int64
	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now := time.Now()
		actualCostD := NewFromFloat(p.actualCost)

		// 语句顺序针对钱包热点行优化：bil_wallets 是同租户所有结算争抢的单行热点，
		// 行锁从钱包 UPDATE 起持有直到 COMMIT，锁内往返次数直接决定单租户结算吞吐。
		// 故：幂等闸门（计费记录唯一约束）与预扣 claim 前置到钱包锁之外，
		// 钱包 UPDATE 用 RETURNING 合并余额快照读，锁内只剩「UPDATE → 流水 INSERT → COMMIT」。

		// a. 创建计费记录（幂等闸门：同 request_id 的重复结算在触碰钱包行之前即被拒绝回滚）
		var err error
		billingID, err = p.createBillingRecord(ctx)
		if err != nil {
			if isDuplicateKeyErr(err) {
				// 同一 request_id 已结算：整个事务回滚，避免重复扣款/重复账单
				return errAlreadySettled
			}
			return gerror.Wrapf(err, "%s: create billing record", p.logPrefix)
		}

		// b. claim 预扣追踪（frozen→settled，每请求独立行，无热点争抢）
		claimedD, err := claimPredeductSettledTx(ctx, p.predeductRequestIDs)
		if err != nil {
			return gerror.Wrapf(err, "%s: claim prededuct settled", p.logPrefix)
		}
		if !claimedD.Equal(NewFromFloat(p.preDeductAmount)) {
			g.Log().Warningf(ctx, "%s: claimed frozen %.6f != pre-deduct %.6f (track expired/released before settle?) requests=%v",
				p.logPrefix, InexactFloat64(claimedD), p.preDeductAmount, p.predeductRequestIDs)
		}

		// c. 更新钱包并原子读回余额快照（RETURNING 合并原「UPDATE + SELECT」两次锁内往返；
		//    decimal 直传 NUMERIC，避免 float64 精度损失）
		var balanceAfter, frozenAfter float64
		row, err := g.DB().Ctx(ctx).GetOne(ctx,
			"UPDATE bil_wallets SET frozen_balance = GREATEST(frozen_balance - ?, 0), balance = balance - ?, updated_at = ? WHERE id = ? RETURNING balance, frozen_balance",
			claimedD, actualCostD, now, p.walletID)
		if err != nil {
			return gerror.Wrapf(err, "%s: update wallet", p.logPrefix)
		}
		// 余额快照 best-effort：解析失败保持 0/0（与原 readWalletBalanceTx 行为一致）
		if row != nil {
			if bd, e := decimal.NewFromString(row["balance"].String()); e == nil {
				balanceAfter = InexactFloat64(bd)
			}
			if fd, e := decimal.NewFromString(row["frozen_balance"].String()); e == nil {
				frozenAfter = InexactFloat64(fd)
			}
		}

		// d. 记录消费流水（事务内）
		if _, err = dao.BilTransactions.Ctx(ctx).Data(p.buildTransaction(billingID, balanceAfter, frozenAfter)).Insert(); err != nil {
			return gerror.Wrapf(err, "%s: record transaction", p.logPrefix)
		}

		return nil
	})
	if err != nil && !errors.Is(err, errAlreadySettled) {
		// 事务回滚后 DB 钱包仍保留原冻结。立即按预扣追踪记录中的原始金额释放，
		// 避免等待孤儿清理任务期间持续占用租户可用余额。
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		for _, requestID := range p.predeductRequestIDs {
			if releaseErr := UnfreezePreDeduct(releaseCtx, p.tenantID, requestID); releaseErr != nil {
				g.Log().Errorf(ctx, "%s: release prededuct after rollback request=%s: %v", p.logPrefix, requestID, releaseErr)
			}
		}
	}
	return billingID, err
}

// claimPredeductSettledTx 结算事务内 claim 预扣追踪：frozen→settled 条件更新并返回实际认领金额之和。
// 只有仍处于 frozen 的 track 才会被认领；已被孤儿清理置 expired / 已被解冻置 released 的 track
// 认领不到（返回和中不含其金额），对应冻结不再重复释放。task 结算会传入 requestID 与
// requestID+"_adjust" 两条（补扣调整产生），一并认领求和。
func claimPredeductSettledTx(ctx context.Context, requestIDs []string) (decimal.Decimal, error) {
	if len(requestIDs) == 0 {
		return decimal.Zero, nil
	}
	placeholders := make([]string, len(requestIDs))
	args := make([]any, len(requestIDs))
	for i, rid := range requestIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = rid
	}
	rows, err := g.DB().Ctx(ctx).GetAll(ctx,
		fmt.Sprintf("UPDATE bil_prededuct_tracks SET status = 'settled' WHERE request_id IN (%s) AND status = 'frozen' RETURNING amount", strings.Join(placeholders, ", ")),
		args...)
	if err != nil {
		return decimal.Zero, err
	}
	sum := decimal.Zero
	for _, row := range rows {
		// NUMERIC(20,10) 以字符串取出，经 decimal 精确解析，不走 float64
		amt, convErr := decimal.NewFromString(row["amount"].String())
		if convErr != nil {
			return decimal.Zero, gerror.Wrapf(convErr, "parse claimed prededuct amount %q", row["amount"].String())
		}
		sum = AddMoney(sum, amt)
	}
	return sum, nil
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
		// A4 修复：计价失败【不得】按零费用结算——那会把定价异常/模型未配价/短暂 DB 故障
		// 都变成免费请求。改为 fail-closed 兜底：按已冻结的预扣额计费（预扣是请求受理时的估价，
		// 当前可得的最佳估值），与 task 结算路径（async_polling / sync_image_worker 默认
		// actualCost = PreDeductAmount）保持一致。保留 token 数便于账单核对。
		g.Log().Errorf(ctx, "settle: calculate cost failed for %s (model=%s), fallback to pre-deduct estimate %.6f: %v",
			requestID, modelName, preDeductAmount, err)
		breakdown = &CostBreakdown{
			TotalCost:    preDeductAmount,
			BaseCost:     preDeductAmount,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			Currency:     "USD",
		}
	}
	actualCost := breakdown.TotalCost

	// 2. 计算差额（decimal 精确运算，float64 仅在结果出口转换）
	refundAmt, supplementAmt := calcSettlementDiff(preDeductAmount, actualCost)

	// 3. 获取钱包
	walletID, err := GetWalletID(ctx, tenantID)
	if err != nil {
		return nil, gerror.Wrapf(err, "settle: get wallet")
	}

	// 4. 获取定价信息（事务外只读）
	pricingResult, _ := GetModelPrice(ctx, tenantID, modelName)

	// 5. 事务内执行结算（钱包扣款 + 计费记录 + 流水 + tracks 状态）
	billingID, err := executeSettlementTx(ctx, settlementTxParams{
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

	// 6. 清除缓存（事务提交后）
	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
	InvalidateWalletRedis(ctx, tenantID)
	CleanupPreDeduct(ctx, tenantID, requestID)

	// 7. 异步检查余额预警
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

// SettleWithUsage 完整 Usage 结算（含 cache token 计费 + 计费快照）
func SettleWithUsage(ctx context.Context, tenantID, userID, apiKeyID, channelID int64,
	modelName, requestID, relayMode string,
	usage *rcommon.Usage, preDeductAmount float64, relayInfo *rcommon.RelayInfo) (*SettlementResult, error) {

	// 1. 使用完整 Usage 计算实际费用（含 cache token）
	breakdown, err := CalculateCostWithUsage(ctx, tenantID, modelName, usage)
	if err != nil {
		// A4 修复：计价失败 fail-closed 兜底按预扣额计费，而非零费用（免费请求）。见 Settle 同段说明。
		g.Log().Errorf(ctx, "settle_with_usage: calculate cost failed for %s (model=%s), fallback to pre-deduct estimate %.6f: %v",
			requestID, modelName, preDeductAmount, err)
		fb := &CostBreakdown{
			TotalCost: preDeductAmount,
			BaseCost:  preDeductAmount,
			Currency:  "USD",
		}
		if usage != nil {
			fb.InputTokens = usage.PromptTokens
			fb.OutputTokens = usage.CompletionTokens
		}
		breakdown = fb
	}
	actualCost := breakdown.TotalCost

	// 2. 计算差额（decimal 精确运算，float64 仅在结果出口转换）
	refundAmt, supplementAmt := calcSettlementDiff(preDeductAmount, actualCost)

	// 3. 获取钱包
	walletID, err := GetWalletID(ctx, tenantID)
	if err != nil {
		return nil, gerror.Wrapf(err, "settle_with_usage: get wallet")
	}

	// 4. 获取定价信息（事务外只读）
	pricingResult, _ := GetModelPrice(ctx, tenantID, modelName)

	// 5. 事务内执行结算（钱包扣款 + 计费记录 + 流水 + tracks 状态）
	billingID, err := executeSettlementTx(ctx, settlementTxParams{
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

	// 6. 清除缓存（事务提交后）
	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
	InvalidateWalletRedis(ctx, tenantID)
	CleanupPreDeduct(ctx, tenantID, requestID)

	// 7. 异步检查余额预警
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

	// 解冻金额由预扣追踪记录决定，不信任调用方传入的估算值。
	if err := UnfreezePreDeduct(ctx, tenantID, requestID); err != nil {
		return err
	}

	// 无需额外操作，预扣金额原路退回
	return nil
}

// SettleStreamInterrupted 流式中断结算：按已确认 usage 走完整 Usage 结算（含 cache token 计费 + 快照）。
// 此前按 (input, output) 两数走 Settle，会丢弃 usage 中的 cache_read / cache_creation token——
// Claude 场景 prompt_tokens 不含 cache token，大缓存请求中断时缓存部分完全漏计费。
func SettleStreamInterrupted(ctx context.Context, tenantID, userID, apiKeyID, channelID int64,
	modelName, requestID, relayMode string,
	usage *rcommon.Usage, preDeductAmount float64, projectID int64) (*SettlementResult, error) {

	// usage 为 nil 必须兜换空 Usage：SettleWithUsage 对 nil usage 会 fail-closed 按预扣全额计费，
	// 而中断且无任何 usage 的正确语义是 0 token 结算（成本 0，全额退差）
	if usage == nil {
		usage = &rcommon.Usage{}
	}
	relayInfo := &rcommon.RelayInfo{
		ProjectID:       projectID,
		OriginModelName: modelName,
		IsStream:        true,
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
