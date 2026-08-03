package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"

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

	// 5. 冻结余额一致性校验
	reconcileFrozenBalance(ctx)

	return result, nil
}

// reconcileFrozenBalance 校验所有租户的 frozen_balance 与追踪记录是否一致
func reconcileFrozenBalance(ctx context.Context) {
	type frozenRow struct {
		TenantID      int64   `json:"tenant_id"`
		FrozenBalance float64 `json:"frozen_balance"`
	}
	var wallets []frozenRow
	dao.BilWallets.Ctx(ctx).
		Where("frozen_balance > 0").
		Fields("tenant_id, frozen_balance").
		Scan(&wallets)

	for _, w := range wallets {
		type sumRow struct {
			Total float64 `json:"total"`
		}
		var tracked sumRow
		dao.BilPredeductTracks.Ctx(ctx).
			Where("tenant_id", w.TenantID).
			Where("status", "frozen").
			Fields("COALESCE(SUM(amount), 0) as total").
			Scan(&tracked)

		diff := w.FrozenBalance - tracked.Total
		if diff > 0.000001 || diff < -0.000001 {
			g.Log().Warningf(ctx,
				"[RECONCILIATION WARNING] tenant=%d frozen_balance=%.6f tracked=%.6f diff=%.6f",
				w.TenantID, w.FrozenBalance, tracked.Total, diff)
		}
	}
}

// CleanSettledPreDeductTracks 清理已终态的预扣追踪记录
// 删除 2 天前状态为 settled / released / expired 的记录，防止表无限增长
func CleanSettledPreDeductTracks(ctx context.Context) {
	const (
		retentionDays = 2
		batchSize     = 5000
	)

	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	var totalDeleted int64
	for {
		result, err := g.DB().Ctx(ctx).Exec(ctx,
			`DELETE FROM bil_prededuct_tracks WHERE id IN (
				SELECT id FROM bil_prededuct_tracks
				WHERE status IN ('settled', 'released', 'expired')
				  AND created_at < ?
				LIMIT ?
			)`, cutoff, batchSize)
		if err != nil {
			g.Log().Errorf(ctx, "[PRE-DEDUCT] clean settled tracks: delete failed: %v", err)
			return
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			break
		}
		totalDeleted += rows
	}

	if totalDeleted > 0 {
		g.Log().Infof(ctx, "[PRE-DEDUCT] cleaned %d settled/released/expired tracks older than %d days",
			totalDeleted, retentionDays)
	}
}

// CleanExpiredPreDeducts 清理过期的预扣记录（防止异常占用余额）
// 超过 PreDeductMaxAge 未结算的预扣应被清理
func CleanExpiredPreDeducts(ctx context.Context) {
	// 1. 查询所有超过 PreDeductMaxAge 仍未结算的冻结记录
	type trackRow struct {
		RequestID string          `json:"request_id"`
		TenantID  int64           `json:"tenant_id"`
		Amount    decimal.Decimal `json:"amount"`
	}
	var tracks []trackRow

	cutoff := time.Now().Add(-time.Duration(PreDeductMaxAge) * time.Second)
	err := dao.BilPredeductTracks.Ctx(ctx).
		Where("status", "frozen").
		Where("created_at < ?", cutoff).
		Fields("request_id, tenant_id, amount").
		Scan(&tracks)
	if err != nil {
		g.Log().Errorf(ctx, "[PRE-DEDUCT] clean expired: query failed: %v", err)
		return
	}

	if len(tracks) == 0 {
		return
	}

	g.Log().Warningf(ctx, "[PRE-DEDUCT] found %d orphaned pre-deducts to clean", len(tracks))

	// 2. 按 tenant_id 分组：记录每个租户名下待清理的 (request_id, amount) 明细。
	//    注意：金额累加会推迟到第 3 步「实际认领成功」之后，只对本实例真正认领到的 tracks 求和。
	tenantTracks := make(map[int64][]trackRow)
	for _, t := range tracks {
		tenantTracks[t.TenantID] = append(tenantTracks[t.TenantID], t)
	}

	// 3. 逐租户释放冻结金额
	// 多实例部署时，N 个实例会同时扫到同一批过期 tracks。为避免重复/超额释放 frozen_balance，
	// 采用「先 claim 再 release」：先用带 status='frozen' 谓词的条件更新把 track 标记为 expired，
	// 仅对本实例真正从 frozen 翻成 expired（RowsAffected > 0）的 track 累加金额并释放。
	// status='frozen' 的条件更新是跨实例的原子闸门——同一 request_id 只有一个实例能命中，
	// 因此各实例释放的金额之和恒等于实际冻结总额，不会重复或超额。
	for tenantID, tenantRows := range tenantTracks {
		// 3a. 逐条 claim，仅累加本实例成功认领的金额
		claimedAmount := decimal.Zero
		claimed := 0
		for _, tr := range tenantRows {
			res, err := g.DB().Ctx(ctx).Exec(ctx,
				"UPDATE bil_prededuct_tracks SET status = 'expired', expired_at = $1 WHERE tenant_id = $2 AND request_id = $3 AND status = 'frozen'",
				time.Now(), tenantID, tr.RequestID)
			if err != nil {
				g.Log().Warningf(ctx, "[PRE-DEDUCT] clean expired: mark track expired failed: tenant=%d request=%s err=%v", tenantID, tr.RequestID, err)
				continue
			}
			if rows, _ := res.RowsAffected(); rows > 0 {
				claimedAmount = claimedAmount.Add(tr.Amount)
				claimed++
			}
		}

		// 3b. 本实例未认领到任何 track（全部已被其它实例处理）→ 跳过释放，绝不重复入账
		if claimed == 0 {
			g.Log().Infof(ctx, "[PRE-DEDUCT] clean expired: tenant=%d all %d tracks already claimed by other instances, skip release", tenantID, len(tenantRows))
			continue
		}

		// 3c. 释放 DB frozen_balance（金额仅来自本实例实际认领的 tracks，精确无重复）
		_, err := g.DB().Ctx(ctx).Exec(ctx,
			"UPDATE bil_wallets SET frozen_balance = GREATEST(frozen_balance - $1, 0), updated_at = $2 WHERE tenant_id = $3",
			claimedAmount, time.Now(), tenantID)
		if err != nil {
			g.Log().Errorf(ctx, "[PRE-DEDUCT] clean expired: unfreeze failed: tenant=%d err=%v", tenantID, err)
			continue
		}

		// 清除缓存
		walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
		InvalidateWalletRedis(ctx, tenantID)

		g.Log().Infof(ctx,
			"[PRE-DEDUCT] cleaned orphaned: tenant=%d amount=%.6f count=%d",
			tenantID, claimedAmount.InexactFloat64(), claimed)
	}
}
