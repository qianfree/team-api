package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/gconv"

	"github.com/qianfree/team-api/internal/dao"
)

// DailyReconciliationResult 日对账结果
type DailyReconciliationResult struct {
	Date                   string
	TotalSettled           float64
	TotalWalletDeduct      float64
	Difference             float64
	DifferencePct          float64
	RecordCount            int64
	MissingSettlementCount int64   // 交叉对账：成功用量中无计费记录的请求数（漏结算=免单）
	MissingSettlementCost  float64 // 交叉对账：漏结算请求的用量记录费用合计（结算失败时通常为 0，条数才是信号）
	OrphanRecordCount      int64   // 崩溃窗口探测：有计费记录但无消费流水（结算第 2/3 步失败且补偿未覆盖的残留）
}

// RunDailyReconciliation 执行日对账
// 比较 billing_records 中已结算总额 与 钱包扣减总额，差异 > 0.1% 时告警；
// 并交叉核对 bil_usage_logs ↔ bil_records，发现"请求成功但从未结算"的免单请求。
func RunDailyReconciliation(ctx context.Context) (*DailyReconciliationResult, error) {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")

	result := &DailyReconciliationResult{Date: yesterday}

	// 1. 统计 billing_records 中昨日已结算总额（上界取次日 00:00 开区间，不丢最后一秒）
	type settledRow struct {
		TotalCost float64 `json:"total_cost"`
		Count     int64   `json:"count"`
	}
	var settled settledRow
	err := dao.BilRecords.Ctx(ctx).
		Where("status", "settled").
		Where("settled_at >= ?", yesterday+" 00:00:00").
		Where("settled_at < ?", today+" 00:00:00").
		Fields("COALESCE(SUM(total_cost), 0) as total_cost, COUNT(*) as count").
		Scan(&settled)
	if err != nil {
		return nil, gerror.Wrapf(err, "query settled records")
	}

	result.TotalSettled = settled.TotalCost
	result.RecordCount = settled.Count

	// 2. 统计 bil_transactions 中昨日消费流水总额（负数，取绝对值）
	type txnRow struct {
		TotalDeduct float64 `json:"total_deduct"`
	}
	var txn txnRow
	err = dao.BilTransactions.Ctx(ctx).
		Where("type", "consume").
		Where("created_at >= ?", yesterday+" 00:00:00").
		Where("created_at < ?", today+" 00:00:00").
		Fields("COALESCE(SUM(ABS(amount)), 0) as total_deduct").
		Scan(&txn)
	if err != nil {
		return nil, gerror.Wrapf(err, "query transactions")
	}

	result.TotalWalletDeduct = txn.TotalDeduct

	// 3. 计算差异
	result.Difference = result.TotalSettled - result.TotalWalletDeduct

	if result.TotalWalletDeduct > 0 {
		result.DifferencePct = (result.Difference / result.TotalWalletDeduct) * 100
		if result.DifferencePct < 0 {
			result.DifferencePct = -result.DifferencePct
		}
	}

	// 4. 差异 > 0.1% 时告警
	if result.DifferencePct > 0.1 {
		g.Log().Warningf(ctx,
			"[RECONCILIATION WARNING] date=%s settled=%.6f deduct=%.6f diff=%.6f (%.2f%%) records=%d",
			yesterday, result.TotalSettled, result.TotalWalletDeduct,
			result.Difference, result.DifferencePct, result.RecordCount)
	} else {
		g.Log().Infof(ctx,
			"[RECONCILIATION OK] date=%s settled=%.6f deduct=%.6f diff=%.6f records=%d",
			yesterday, result.TotalSettled, result.TotalWalletDeduct,
			result.Difference, result.RecordCount)
	}

	// 4.5 交叉对账：昨日成功用量中 request_id 无对应计费记录的请求 = 漏结算（免单）。
	// bil_records 与 bil_transactions 在同一事务写入、恒相等，上面的聚合对账发现不了漏结算，
	// 必须以 bil_usage_logs（请求侧真相）为基准反连接核对。join 不限制 bil_records 日期，
	// 避免跨午夜结算（23:59 成功、00:00 落账）被误报。
	type missRow struct {
		Cnt  int64   `json:"cnt"`
		Cost float64 `json:"cost"`
	}
	var miss missRow
	err = dao.BilUsageLogs.Ctx(ctx).
		As("u").
		LeftJoin("bil_records r", "r.request_id = u.request_id").
		Where("u.status", "success").
		Where("u.created_at >= ?", yesterday+" 00:00:00").
		Where("u.created_at < ?", today+" 00:00:00").
		Where("r.request_id IS NULL").
		Fields("COUNT(*) as cnt, COALESCE(SUM(u.actual_cost), 0) as cost").
		Scan(&miss)
	if err != nil {
		g.Log().Errorf(ctx, "[RECONCILIATION] cross-check usage_logs vs bil_records failed: %v", err)
	} else if miss.Cnt > 0 {
		result.MissingSettlementCount = miss.Cnt
		result.MissingSettlementCost = miss.Cost
		g.Log().Warningf(ctx,
			"[RECONCILIATION WARNING] date=%s missing settlements: count=%d logged_cost=%.6f (usage success but no bil_record — free rides, needs manual recovery)",
			yesterday, miss.Cnt, miss.Cost)
	}

	// 4.6 崩溃窗口探测：昨日有计费记录但无对应 consume 流水。
	// 结算骨架是「闸门（records）→ Redis 扣款 → 流水」三步，第 2/3 步失败会触发补偿删除记录，
	// 补偿自身失败时留下此类孤儿记录（有账单无账本），需要人工核查资金状态。
	type orphanRow struct {
		Cnt int64 `json:"cnt"`
	}
	var orphan orphanRow
	err = dao.BilRecords.Ctx(ctx).
		As("r").
		LeftJoin("bil_transactions t", "t.related_id = r.id AND t.related_type = 'billing_record' AND t.type = 'consume'").
		Where("r.settled_at >= ?", yesterday+" 00:00:00").
		Where("r.settled_at < ?", today+" 00:00:00").
		Where("t.id IS NULL").
		Fields("COUNT(*) as cnt").
		Scan(&orphan)
	if err != nil {
		g.Log().Errorf(ctx, "[RECONCILIATION] orphan-record check failed: %v", err)
	} else if orphan.Cnt > 0 {
		result.OrphanRecordCount = orphan.Cnt
		g.Log().Warningf(ctx,
			"[RECONCILIATION WARNING] date=%s orphan billing records (no consume transaction): count=%d — settlement crash-window residue, needs manual fund check",
			yesterday, orphan.Cnt)
	}

	// 5. 冻结余额一致性校验（Redis 权威值 vs DB 物化值）
	reconcileFrozenBalance(ctx)

	return result, nil
}

// reconcileFrozenBalance 校验 Redis 权威冻结值与 DB 物化冻结值是否一致。
// 物化窗口（数秒）内的漂移属正常滞后；漂移持续超过 2 倍物化间隔才告警——
// 意味着物化器停滞或某条资金路径没有正确标记脏租户。
func reconcileFrozenBalance(ctx context.Context) {
	materializeLag := walletMaterializeInterval(ctx) * 2

	cursor := 0
	for {
		res, err := g.Redis().Do(ctx, "SCAN", cursor, "MATCH", "wallet:v2:*", "COUNT", 200)
		if err != nil {
			g.Log().Warningf(ctx, "[RECONCILIATION] scan wallet hashes failed: %v", err)
			return
		}
		arr := res.Array()
		if len(arr) != 2 {
			return
		}
		cursor = gconv.Int(arr[0])
		for _, key := range gconv.Strings(arr[1]) {
			var tenantID int64
			if _, err := fmt.Sscanf(key, "wallet:v2:%d", &tenantID); err != nil || tenantID <= 0 {
				continue
			}
			checkOneWalletFrozenDrift(ctx, tenantID, materializeLag)
		}
		if cursor == 0 {
			return
		}
	}
}

// checkOneWalletFrozenDrift 比对单个租户的 Redis/DB 冻结值，超窗漂移告警
func checkOneWalletFrozenDrift(ctx context.Context, tenantID int64, materializeLag time.Duration) {
	_, frozenMicro, exists, err := readWalletHash(ctx, tenantID)
	if err != nil || !exists {
		return
	}

	var w *struct {
		FrozenBalance float64     `json:"frozen_balance"`
		UpdatedAt     *gtime.Time `json:"updated_at"`
	}
	err = dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("frozen_balance, updated_at").
		Scan(&w)
	if err != nil || w == nil {
		return
	}

	diff := NewFromFloat(w.FrozenBalance).Sub(FromMicro(frozenMicro)).Abs()
	if !diff.GreaterThan(NewFromFloat(0.000001)) {
		return
	}
	// DB 在物化窗口内刚刷新过：漂移是正常滞后，下轮物化会收敛
	if w.UpdatedAt != nil && time.Since(w.UpdatedAt.Time) <= materializeLag {
		return
	}
	g.Log().Warningf(ctx,
		"[RECONCILIATION WARNING] tenant=%d frozen drift persists: db=%.6f redis=%.6f db_updated_at=%v (materializer stuck or dirty-mark missing?)",
		tenantID, w.FrozenBalance, InexactFloat64(FromMicro(frozenMicro)), w.UpdatedAt)
}
