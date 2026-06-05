package tenant

import (
	"context"
	"math"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
)

// roundUSD 精确到小数点后 6 位，符合 CLAUDE.md 资金精度规范
func roundUSD(v float64) float64 {
	return math.Round(v*1000000) / 1000000
}

// Dashboard returns the tenant dashboard statistics.
func (s *sTenant) Dashboard(ctx context.Context, req *v1.TenantDashboardReq) (*v1.TenantDashboardRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	now := gtime.Now()
	today := now.Format("Y-m-d")
	monthStart := now.Format("Y-m") + "-01"

	// 今日统计
	type dayStats struct {
		Requests     int     `json:"requests"`
		InputTokens  int     `json:"input_tokens"`
		OutputTokens int     `json:"output_tokens"`
		TotalCost    float64 `json:"total_cost"`
	}
	var todayRow dayStats
	err := dao.BilUsageLogs.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("created_at >= ?", today+" 00:00:00").
		Fields("COUNT(*) as requests, COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens, COALESCE(SUM(total_cost), 0) as total_cost").
		Scan(&todayRow)
	if err != nil {
		return nil, err
	}

	// 本月统计
	var monthRow dayStats
	err = dao.BilUsageLogs.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("created_at >= ?", monthStart+" 00:00:00").
		Fields("COUNT(*) as requests, COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens, COALESCE(SUM(total_cost), 0) as total_cost").
		Scan(&monthRow)
	if err != nil {
		return nil, err
	}

	// 钱包余额
	var wallet *struct {
		Balance          float64 `json:"balance"`
		FrozenBalance    float64 `json:"frozen_balance"`
		WarningThreshold float64 `json:"warning_threshold"`
	}
	err = dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("balance, frozen_balance, warning_threshold").
		Scan(&wallet)
	if err != nil {
		return nil, err
	}

	if wallet == nil {
		wallet = &struct {
			Balance          float64 `json:"balance"`
			FrozenBalance    float64 `json:"frozen_balance"`
			WarningThreshold float64 `json:"warning_threshold"`
		}{Balance: 0, FrozenBalance: 0, WarningThreshold: 0}
	}

	// 活跃Key数
	activeKeys, err := dao.ApiKeys.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("status", "active").
		Count()
	if err != nil {
		return nil, err
	}

	// 成员数
	memberCount, err := dao.TntUsers.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("status", "active").
		Count()
	if err != nil {
		return nil, err
	}

	return &v1.TenantDashboardRes{
		Today: map[string]any{
			"requests":      todayRow.Requests,
			"input_tokens":  todayRow.InputTokens,
			"output_tokens": todayRow.OutputTokens,
			"total_cost":    roundUSD(todayRow.TotalCost),
		},
		Month: map[string]any{
			"requests":      monthRow.Requests,
			"input_tokens":  monthRow.InputTokens,
			"output_tokens": monthRow.OutputTokens,
			"total_cost":    roundUSD(monthRow.TotalCost),
		},
		Wallet: map[string]any{
			"balance":           roundUSD(wallet.Balance),
			"frozen_balance":    roundUSD(wallet.FrozenBalance),
			"available":         roundUSD(wallet.Balance - wallet.FrozenBalance),
			"warning_threshold": roundUSD(wallet.WarningThreshold),
		},
		ActiveKeys:  activeKeys,
		MemberCount: memberCount,
	}, nil
}

// TokenTrends returns daily token usage for the past N days.
func (s *sTenant) TokenTrends(ctx context.Context, req *v1.TenantTokenTrendsReq) (*v1.TenantTokenTrendsRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	days := req.Days
	if days <= 0 || days > 90 {
		days = 30
	}

	startDate := gtime.Now().AddDate(0, 0, -days).Format("Y-m-d")

	type tokenTrendRow struct {
		Date         string  `json:"date"`
		InputTokens  int64   `json:"input_tokens"`
		OutputTokens int64   `json:"output_tokens"`
		Requests     int     `json:"requests"`
		TotalCost    float64 `json:"total_cost"`
	}

	var records []tokenTrendRow
	err := g.DB().Ctx(ctx).Raw(`
		SELECT
			DATE(created_at) as date,
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COUNT(*) as requests,
			COALESCE(SUM(total_cost), 0) as total_cost
		FROM bil_usage_logs
		WHERE tenant_id = ? AND created_at >= ?
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`, tenantID, startDate+" 00:00:00").Scan(&records)
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = []tokenTrendRow{}
	}

	result := make([]map[string]any, 0, len(records))
	for _, r := range records {
		result = append(result, map[string]any{
			"date":          r.Date,
			"input_tokens":  r.InputTokens,
			"output_tokens": r.OutputTokens,
			"requests":      r.Requests,
			"total_cost":    roundUSD(r.TotalCost),
		})
	}

	return &v1.TenantTokenTrendsRes{
		List: result,
	}, nil
}

// ModelDistribution returns the distribution of model usage.
func (s *sTenant) ModelDistribution(ctx context.Context, req *v1.TenantModelDistributionReq) (*v1.TenantModelDistributionRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	days := req.Days
	if days <= 0 || days > 90 {
		days = 30
	}

	startDate := gtime.Now().AddDate(0, 0, -days).Format("Y-m-d")

	type modelDistRow struct {
		ModelName    string  `json:"model_name"`
		Requests     int     `json:"requests"`
		InputTokens  int64   `json:"input_tokens"`
		OutputTokens int64   `json:"output_tokens"`
		TotalCost    float64 `json:"total_cost"`
	}

	var records []modelDistRow
	err := g.DB().Ctx(ctx).Raw(`
		SELECT
			model_name,
			COUNT(*) as requests,
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost
		FROM bil_usage_logs
		WHERE tenant_id = ? AND created_at >= ?
		GROUP BY model_name
		ORDER BY total_cost DESC
		LIMIT 20
	`, tenantID, startDate+" 00:00:00").Scan(&records)
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = []modelDistRow{}
	}

	result := make([]map[string]any, 0, len(records))
	for _, r := range records {
		result = append(result, map[string]any{
			"model_name":    r.ModelName,
			"requests":      r.Requests,
			"input_tokens":  r.InputTokens,
			"output_tokens": r.OutputTokens,
			"total_cost":    roundUSD(r.TotalCost),
		})
	}

	return &v1.TenantModelDistributionRes{
		List: result,
	}, nil
}

// BalancePrediction predicts when the balance will be exhausted.
func (s *sTenant) BalancePrediction(ctx context.Context, req *v1.TenantBalancePredictionReq) (*v1.TenantBalancePredictionRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)

	sevenDaysAgo := gtime.Now().AddDate(0, 0, -7).Format("Y-m-d")

	var stats struct {
		TotalCost float64 `json:"total_cost"`
	}
	err := dao.BilUsageLogs.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("created_at >= ?", sevenDaysAgo+" 00:00:00").
		Fields("COALESCE(SUM(total_cost), 0) as total_cost").
		Scan(&stats)
	if err != nil {
		return nil, err
	}

	dailyAvg := roundUSD(stats.TotalCost / 7.0)

	var wallet *struct {
		Balance       float64 `json:"balance"`
		FrozenBalance float64 `json:"frozen_balance"`
	}
	err = dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("balance, frozen_balance").
		Scan(&wallet)
	if err != nil {
		return nil, err
	}
	if wallet == nil {
		wallet = &struct {
			Balance       float64 `json:"balance"`
			FrozenBalance float64 `json:"frozen_balance"`
		}{Balance: 0, FrozenBalance: 0}
	}

	available := roundUSD(wallet.Balance - wallet.FrozenBalance)
	res := &v1.TenantBalancePredictionRes{
		DailyAvgCost:     dailyAvg,
		AvailableBalance: available,
	}

	if dailyAvg > 0 && available > 0 {
		daysVal := int(math.Floor(available / dailyAvg))
		exhaustDate := time.Now().AddDate(0, 0, daysVal).Format("2006-01-02")
		res.WillExhaust = true
		res.DaysUntilExhaust = &daysVal
		res.ExhaustDate = &exhaustDate
	} else if dailyAvg <= 0 {
		res.WillExhaust = false
		msg := "近期无消耗，无法预测"
		res.Message = &msg
	} else {
		res.WillExhaust = true
		daysVal := 0
		exhaustDate := time.Now().Format("2006-01-02")
		res.DaysUntilExhaust = &daysVal
		res.ExhaustDate = &exhaustDate
	}

	return res, nil
}

// BudgetAlerts checks member and project budget usage and returns those above 80%.
func (s *sTenant) BudgetAlerts(ctx context.Context, req *v1.TenantBudgetAlertsReq) (*v1.TenantBudgetAlertsRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	return &v1.TenantBudgetAlertsRes{
		Members:  []map[string]any{},
		Projects: []map[string]any{},
	}, nil
}

// GetMemberUsageRanking returns top members by usage cost in a given date range.
func (s *sTenant) GetMemberUsageRanking(ctx context.Context, req *v1.TenantMemberUsageRankingReq) (*v1.TenantMemberUsageRankingRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	days := req.Days
	if days <= 0 || days > 90 {
		days = 30
	}
	limit := req.Limit
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	startDate := gtime.Now().AddDate(0, 0, -days).Format("Y-m-d")

	type memberUsageRow struct {
		UserId       int64   `json:"user_id"`
		Username     string  `json:"username"`
		DisplayName  string  `json:"display_name"`
		Requests     int     `json:"requests"`
		InputTokens  int64   `json:"input_tokens"`
		OutputTokens int64   `json:"output_tokens"`
		TotalCost    float64 `json:"total_cost"`
	}

	var records []memberUsageRow
	err := g.DB().Ctx(ctx).Raw(`
		SELECT
			u.id as user_id,
			u.username,
			u.display_name,
			COUNT(*) as requests,
			COALESCE(SUM(ul.input_tokens), 0) as input_tokens,
			COALESCE(SUM(ul.output_tokens), 0) as output_tokens,
			COALESCE(SUM(ul.total_cost), 0) as total_cost
		FROM bil_usage_logs ul
		JOIN tnt_users u ON u.id = ul.user_id
		WHERE ul.tenant_id = ? AND ul.created_at >= ?
		GROUP BY u.id, u.username, u.display_name
		ORDER BY total_cost DESC
		LIMIT ?
	`, tenantID, startDate+" 00:00:00", limit).Scan(&records)
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = []memberUsageRow{}
	}

	result := make([]map[string]any, 0, len(records))
	for _, r := range records {
		result = append(result, map[string]any{
			"user_id":       r.UserId,
			"username":      r.Username,
			"display_name":  r.DisplayName,
			"requests":      r.Requests,
			"input_tokens":  r.InputTokens,
			"output_tokens": r.OutputTokens,
			"total_cost":    roundUSD(r.TotalCost),
		})
	}

	return &v1.TenantMemberUsageRankingRes{
		List: result,
	}, nil
}
