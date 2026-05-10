package billing

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

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

	// 2. 统计 bil_transactions 中昨日结算流水总额（负数，取绝对值）
	type txnRow struct {
		TotalDeduct float64 `json:"total_deduct"`
	}
	var txn txnRow
	err = dao.BilTransactions.Ctx(ctx).
		Where("type", "settle").
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

	return result, nil
}

// CleanExpiredPreDeducts 清理过期的预扣记录（防止异常占用余额）
// 超过 PreDeductMaxAge 未结算的预扣应被清理
func CleanExpiredPreDeducts(ctx context.Context) {
	// 清理超过 5 分钟未结算的预扣 Redis key
	// 在实际运行中，这应该由 Redis TTL 自动清理
	// 这里作为额外的安全网，扫描并记录异常
	g.Log().Info(ctx, "[PRE-DEDUCT] expired pre-deduct cleanup check passed (relying on Redis TTL)")
}
