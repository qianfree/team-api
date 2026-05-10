package task

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// DailyUsageSummary 每日用量汇总
func DailyUsageSummary(ctx context.Context) {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	_, err := g.DB().Exec(ctx, `
		INSERT INTO bil_daily_usage_summary (tenant_id, date, total_requests, total_tokens, total_cost)
		SELECT
			tenant_id,
			$1::date,
			COUNT(*),
			COALESCE(SUM(input_tokens + output_tokens), 0),
			COALESCE(SUM(total_cost), 0)
		FROM bil_usage_logs
		WHERE created_at::date = $1::date
		GROUP BY tenant_id
		ON CONFLICT (tenant_id, date)
		DO UPDATE SET
			total_requests = EXCLUDED.total_requests,
			total_tokens = EXCLUDED.total_tokens,
			total_cost = EXCLUDED.total_cost,
			updated_at = NOW()
	`, yesterday)
	if err != nil {
		g.Log().Errorf(ctx, "[Summary] DailyUsageSummary: %v", err)
	}
}

// DailyRevenueSummary 每日收入汇总
func DailyRevenueSummary(ctx context.Context) {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	var rechargeTotal struct {
		Total float64 `json:"total"`
	}
	g.DB().Raw(`
		SELECT COALESCE(SUM(amount), 0) as total
		FROM bil_transactions
		WHERE type = 'recharge' AND created_at::date = $1::date
	`, yesterday).Scan(ctx, &rechargeTotal)

	var consumptionTotal struct {
		Total float64 `json:"total"`
	}
	g.DB().Raw(`
		SELECT COALESCE(SUM(total_cost), 0) as total
		FROM bil_usage_logs
		WHERE created_at::date = $1::date
	`, yesterday).Scan(ctx, &consumptionTotal)

	var newOrders struct {
		Count int `json:"count"`
	}
	g.DB().Raw(`
		SELECT COUNT(*) as count
		FROM ord_orders
		WHERE created_at::date = $1::date
	`, yesterday).Scan(ctx, &newOrders)

	var paidOrders struct {
		Count int `json:"count"`
	}
	g.DB().Raw(`
		SELECT COUNT(*) as count
		FROM ord_orders
		WHERE status IN ('paid', 'fulfilled') AND paid_at::date = $1::date
	`, yesterday).Scan(ctx, &paidOrders)

	netRevenue := rechargeTotal.Total - consumptionTotal.Total

	_, err := g.DB().Exec(ctx, `
		INSERT INTO bil_daily_revenue_summary (date, total_recharge, total_consumption, net_revenue, new_orders, paid_orders)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (date)
		DO UPDATE SET
			total_recharge = EXCLUDED.total_recharge,
			total_consumption = EXCLUDED.total_consumption,
			net_revenue = EXCLUDED.net_revenue,
			new_orders = EXCLUDED.new_orders,
			paid_orders = EXCLUDED.paid_orders,
			updated_at = NOW()
	`, yesterday, rechargeTotal.Total, consumptionTotal.Total, netRevenue, newOrders.Count, paidOrders.Count)
	if err != nil {
		g.Log().Errorf(ctx, "[Summary] DailyRevenueSummary: %v", err)
	}
}

// MonthlyUsageSummary 每月用量汇总
func MonthlyUsageSummary(ctx context.Context) {
	lastMonth := time.Now().AddDate(0, -1, 0).Format("2006-01")
	lastMonthStart := lastMonth + "-01"
	lastMonthEnd := time.Now().Format("2006-01") + "-01"

	_, err := g.DB().Exec(ctx, `
		INSERT INTO bil_monthly_usage_summary (tenant_id, month, total_requests, total_tokens, total_cost)
		SELECT
			tenant_id,
			$1::date,
			COUNT(*),
			COALESCE(SUM(input_tokens + output_tokens), 0),
			COALESCE(SUM(total_cost), 0)
		FROM bil_usage_logs
		WHERE created_at >= $1::date AND created_at < $2::date
		GROUP BY tenant_id
		ON CONFLICT (tenant_id, month)
		DO UPDATE SET
			total_requests = EXCLUDED.total_requests,
			total_tokens = EXCLUDED.total_tokens,
			total_cost = EXCLUDED.total_cost,
			updated_at = NOW()
	`, lastMonthStart, lastMonthEnd)
	if err != nil {
		g.Log().Errorf(ctx, "[Summary] MonthlyUsageSummary: %v", err)
	}
}

// MonthlyRevenueSummary 每月收入汇总
func MonthlyRevenueSummary(ctx context.Context) {
	lastMonth := time.Now().AddDate(0, -1, 0).Format("2006-01")
	lastMonthStart := lastMonth + "-01"
	lastMonthEnd := time.Now().Format("2006-01") + "-01"

	var rechargeTotal struct {
		Total float64 `json:"total"`
	}
	g.DB().Raw(`
		SELECT COALESCE(SUM(amount), 0) as total
		FROM bil_transactions
		WHERE type = 'recharge' AND created_at >= $1::date AND created_at < $2::date
	`, lastMonthStart, lastMonthEnd).Scan(ctx, &rechargeTotal)

	var consumptionTotal struct {
		Total float64 `json:"total"`
	}
	g.DB().Raw(`
		SELECT COALESCE(SUM(total_cost), 0) as total
		FROM bil_usage_logs
		WHERE created_at >= $1::date AND created_at < $2::date
	`, lastMonthStart, lastMonthEnd).Scan(ctx, &consumptionTotal)

	netRevenue := rechargeTotal.Total - consumptionTotal.Total

	_, err := g.DB().Exec(ctx, `
		INSERT INTO bil_monthly_revenue_summary (month, total_recharge, total_consumption, net_revenue)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (month)
		DO UPDATE SET
			total_recharge = EXCLUDED.total_recharge,
			total_consumption = EXCLUDED.total_consumption,
			net_revenue = EXCLUDED.net_revenue,
			updated_at = NOW()
	`, lastMonthStart, rechargeTotal.Total, consumptionTotal.Total, netRevenue)
	if err != nil {
		g.Log().Errorf(ctx, "[Summary] MonthlyRevenueSummary: %v", err)
	}
}
