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
	Date              string
	TotalSettled      float64
	TotalWalletDeduct float64
	Difference        float64
	DifferencePct     float64
	RecordCount       int64
}

// RunDailyReconciliation 执行日对账
// 比较 billing_records 中已结算总额 与 钱包扣减总额，差异 > 0.1% 时告警
func RunDailyReconciliation(ctx context.Context) (*DailyReconciliationResult, error) {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	result := &DailyReconciliationResult{Date: yesterday}

	// 1. 统计 billing_records 中昨日已结算总额
	type settledRow struct {
		TotalCost float64 `json:"total_cost"`
		Count     int64   `json:"count"`
	}
	var settled settledRow
	err := dao.BilRecords.Ctx(ctx).
		Where("status", "settled").
		Where("settled_at >= ?", yesterday+" 00:00:00").
		Where("settled_at < ?", yesterday+" 23:59:59").
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
		Where("created_at < ?", yesterday+" 23:59:59").
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
		RequestID string  `json:"request_id"`
		TenantID  int64   `json:"tenant_id"`
		Amount    float64 `json:"amount"`
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

	// 2. 按 tenant_id 分组聚合（A8：金额用 decimal 累加，避免 float64 逐笔求和漂移；
	//    该总额会喂给 frozen_balance 释放，属金额变更路径，须精确）
	tenantAmounts := make(map[int64]decimal.Decimal)
	tenantRequests := make(map[int64][]string)
	for _, t := range tracks {
		tenantAmounts[t.TenantID] = tenantAmounts[t.TenantID].Add(dec(t.Amount))
		tenantRequests[t.TenantID] = append(tenantRequests[t.TenantID], t.RequestID)
	}

	// 3. 逐租户释放冻结金额
	for tenantID, totalAmountD := range tenantAmounts {
		requestIDs := tenantRequests[tenantID]
		totalAmount := roundMoney(totalAmountD) // decimal 累加结果收敛到 10 位再落库

		// 释放 DB frozen_balance
		_, err := g.DB().Ctx(ctx).Exec(ctx,
			"UPDATE bil_wallets SET frozen_balance = GREATEST(frozen_balance - $1, 0), updated_at = $2 WHERE tenant_id = $3",
			totalAmount, time.Now(), tenantID)
		if err != nil {
			g.Log().Errorf(ctx, "[PRE-DEDUCT] clean expired: unfreeze failed: tenant=%d err=%v", tenantID, err)
			continue
		}

		// 标记 tracks 为 expired
		for _, reqID := range requestIDs {
			g.DB().Ctx(ctx).Exec(ctx,
				"UPDATE bil_prededuct_tracks SET status = 'expired', expired_at = $1 WHERE tenant_id = $2 AND request_id = $3 AND status = 'frozen'",
				time.Now(), tenantID, reqID)
		}

		// 清除缓存
		walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
		InvalidateWalletRedis(ctx, tenantID)

		g.Log().Infof(ctx,
			"[PRE-DEDUCT] cleaned orphaned: tenant=%d amount=%.6f count=%d",
			tenantID, totalAmount, len(requestIDs))
	}
}
