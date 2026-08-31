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

// budgetAlertThreshold 预算/额度使用率达到该比例即进入告警列表
const budgetAlertThreshold = 0.8

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

	// 消费环比 + 无效支出（同表同租户同区间，合并为一条查询）。
	//
	// 上月只截取与本月已过时长相同的窗口，否则月初永远显示"环比大跌"。
	// 三个边界统一格式化成与上面「本月统计」完全相同的字符串形式再传参：
	// 本月起点必须与 monthStart 逐字一致，否则服务器时区与数据库会话时区不同时，
	// 同屏的「本月消费」与 month.total_cost 会对不上。
	const boundLayout = "2006-01-02 15:04:05"
	curStartT := time.Date(now.Year(), time.Month(now.Month()), 1, 0, 0, 0, 0, now.Location())
	curStartStr := monthStart + " 00:00:00"
	prevStartStr := curStartT.AddDate(0, -1, 0).Format(boundLayout)
	prevEndStr := curStartT.AddDate(0, -1, 0).Add(now.Time.Sub(curStartT)).Format(boundLayout)

	var trendRow struct {
		CurCost         float64 `json:"cur_cost"`
		PrevCost        float64 `json:"prev_cost"`
		WastedCost      float64 `json:"wasted_cost"`
		FailedRequests  int64   `json:"failed_requests"`
		RetriedRequests int64   `json:"retried_requests"`
	}
	err = g.DB().Ctx(ctx).Raw(`
		SELECT
			COALESCE(SUM(CASE WHEN created_at >= ? THEN total_cost END), 0) AS cur_cost,
			COALESCE(SUM(CASE WHEN created_at >= ? AND created_at < ? THEN total_cost END), 0) AS prev_cost,
			COALESCE(SUM(CASE WHEN created_at >= ? AND status <> 'success' THEN total_cost END), 0) AS wasted_cost,
			COUNT(*) FILTER (WHERE created_at >= ? AND status <> 'success') AS failed_requests,
			COUNT(*) FILTER (WHERE created_at >= ? AND retry_index > 0) AS retried_requests
		FROM bil_usage_logs
		WHERE tenant_id = ? AND created_at >= ?
	`, curStartStr, prevStartStr, prevEndStr, curStartStr, curStartStr, curStartStr, tenantID, prevStartStr).Scan(&trendRow)
	if err != nil {
		return nil, err
	}

	costTrend := v1.TenantCostTrend{
		CurrentCost:  roundUSD(trendRow.CurCost),
		PreviousCost: roundUSD(trendRow.PrevCost),
		HasPrevious:  trendRow.PrevCost > 0,
	}
	if trendRow.PrevCost > 0 {
		costTrend.DeltaPercent = math.Round((trendRow.CurCost-trendRow.PrevCost)/trendRow.PrevCost*10000) / 100
	}

	waste := v1.TenantWasteStat{
		WastedCost:      roundUSD(trendRow.WastedCost),
		FailedRequests:  trendRow.FailedRequests,
		RetriedRequests: trendRow.RetriedRequests,
	}
	if trendRow.CurCost > 0 {
		waste.SharePercent = math.Round(trendRow.WastedCost/trendRow.CurCost*10000) / 100
	}

	// 实时 RPM/TPM（Redis 滑动窗口，本租户维度；Redis 不可用时为 0）
	rpm, tpm := common.GetRealtimeMetrics(ctx, tenantID)

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
		Rpm:         rpm,
		Tpm:         tpm,
		ActiveKeys:  activeKeys,
		MemberCount: memberCount,
		CostTrend:   costTrend,
		Waste:       waste,
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

// BudgetAlerts checks member and project budget usage and returns those above the alert threshold.
func (s *sTenant) BudgetAlerts(ctx context.Context, req *v1.TenantBudgetAlertsReq) (*v1.TenantBudgetAlertsRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)

	// 项目侧：复用与预算执行同源的汇总
	projectRows, err := tenantProjectBudgets(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	projects := make([]v1.TenantProjectAlert, 0, len(projectRows))
	for _, r := range projectRows {
		if r.Budget <= 0 {
			continue
		}
		ratio := r.Used / r.Budget
		if ratio < budgetAlertThreshold {
			continue
		}
		projects = append(projects, v1.TenantProjectAlert{
			Id:           r.Id,
			Name:         r.Name,
			Budget:       roundUSD(r.Budget),
			Used:         roundUSD(r.Used),
			UsagePercent: math.Round(ratio*10000) / 100,
		})
	}

	// 成员侧：额度是控制线（超限直接拒绝调用，不走钱包兜底），因此按使用比例预警。
	// 末尾那段周期守卫不能省：周期额度是「惰性重置」——只在成员下次发起调用时
	// 由 relay 链路上的 needsReset/resetMemberQuota 清零（internal/logic/billing/member_quota.go）。
	// 上个周期用满、之后再没调用过的成员，其 quota_used 会一直停在上限，
	// 不加守卫就会永远出现在告警里。判定口径与 needsReset 一致：按 UTC 比较周期。
	var memberRows []struct {
		Id           int64      `json:"id"`
		Username     string     `json:"username"`
		DisplayName  string     `json:"display_name"`
		QuotaLimit   float64    `json:"quota_limit"`
		QuotaUsed    float64    `json:"quota_used"`
		QuotaPeriod  string     `json:"quota_period"`
		QuotaResetAt *time.Time `json:"quota_reset_at"`
	}
	err = g.DB().Ctx(ctx).Raw(`
		SELECT
			id,
			username,
			COALESCE(display_name, '') AS display_name,
			COALESCE(quota_limit, 0) AS quota_limit,
			COALESCE(quota_used, 0) AS quota_used,
			COALESCE(quota_period, '') AS quota_period,
			quota_reset_at
		FROM tnt_users
		WHERE tenant_id = ? AND status = 'active'
			AND COALESCE(quota_type, 'none') <> 'none'
			AND COALESCE(quota_limit, 0) > 0
			AND COALESCE(quota_used, 0) / quota_limit >= ?
			AND (
				COALESCE(quota_type, 'none') <> 'periodic'
				OR quota_period IS NULL OR quota_period = ''
				OR quota_reset_at IS NULL
				OR (quota_period = 'day'
					AND (quota_reset_at AT TIME ZONE 'UTC') >= date_trunc('day',   now() AT TIME ZONE 'UTC'))
				OR (quota_period = 'week'
					AND (quota_reset_at AT TIME ZONE 'UTC') >= date_trunc('week',  now() AT TIME ZONE 'UTC'))
				OR (quota_period = 'month'
					AND (quota_reset_at AT TIME ZONE 'UTC') >= date_trunc('month', now() AT TIME ZONE 'UTC'))
			)
		ORDER BY COALESCE(quota_used, 0) / quota_limit DESC
	`, tenantID, budgetAlertThreshold).Scan(&memberRows)
	if err != nil {
		return nil, err
	}

	members := make([]v1.TenantMemberAlert, 0, len(memberRows))
	for _, r := range memberRows {
		alert := v1.TenantMemberAlert{
			Id:           r.Id,
			Username:     r.Username,
			DisplayName:  r.DisplayName,
			QuotaLimit:   roundUSD(r.QuotaLimit),
			QuotaUsed:    roundUSD(r.QuotaUsed),
			UsagePercent: math.Round(r.QuotaUsed/r.QuotaLimit*10000) / 100,
		}
		if r.QuotaPeriod != "" {
			if next := calcNextReset(r.QuotaResetAt, r.QuotaPeriod); next != nil {
				alert.NextResetAt = next.Format("2006-01-02 15:04:05")
			}
		}
		members = append(members, alert)
	}

	return &v1.TenantBudgetAlertsRes{Members: members, Projects: projects}, nil
}

// GetMemberUsageRanking returns top members by usage cost in a given date range,
// including the change versus the previous window of the same length.
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

	// 当前窗口 [curStart, now)，对比窗口 [prevStart, curStart)，两者等长。
	// 对比窗口可能落进已被清理的分区（usage_log_cleanup 按 usage_log_retention_days
	// 丢弃过期分区，默认 90 天），此时上期数据不是"为 0"而是"不存在"——
	// 若照常相减会显示成 -100% 的假暴跌，因此直接不查、并告诉前端隐藏环比列。
	now := gtime.Now()
	curStart := now.AddDate(0, 0, -days).StartOfDay().Time
	retentionDays := common.Config().GetInt(ctx, "usage_log_retention_days")
	if retentionDays <= 0 {
		retentionDays = 90
	}
	prevAvailable := days*2 <= retentionDays
	prevStart := curStart
	if prevAvailable {
		prevStart = now.AddDate(0, 0, -days*2).StartOfDay().Time
	}

	type memberUsageRow struct {
		UserId       int64   `json:"user_id"`
		Username     string  `json:"username"`
		DisplayName  string  `json:"display_name"`
		Requests     int64   `json:"requests"`
		InputTokens  int64   `json:"input_tokens"`
		OutputTokens int64   `json:"output_tokens"`
		TotalCost    float64 `json:"total_cost"`
		PrevCost     float64 `json:"prev_cost"`
		QuotaLimit   float64 `json:"quota_limit"`
		QuotaUsed    float64 `json:"quota_used"`
		QuotaType    string  `json:"quota_type"`
	}

	var records []memberUsageRow
	err := g.DB().Ctx(ctx).Raw(`
		SELECT
			u.id AS user_id,
			u.username,
			COALESCE(u.display_name, '') AS display_name,
			COUNT(*) FILTER (WHERE ul.created_at >= ?) AS requests,
			COALESCE(SUM(CASE WHEN ul.created_at >= ? THEN ul.input_tokens END), 0) AS input_tokens,
			COALESCE(SUM(CASE WHEN ul.created_at >= ? THEN ul.output_tokens END), 0) AS output_tokens,
			COALESCE(SUM(CASE WHEN ul.created_at >= ? THEN ul.total_cost END), 0) AS total_cost,
			COALESCE(SUM(CASE WHEN ul.created_at < ? THEN ul.total_cost END), 0) AS prev_cost,
			COALESCE(u.quota_limit, 0) AS quota_limit,
			COALESCE(u.quota_used, 0) AS quota_used,
			COALESCE(u.quota_type, 'none') AS quota_type
		FROM bil_usage_logs ul
		JOIN tnt_users u ON u.id = ul.user_id
		WHERE ul.tenant_id = ? AND ul.created_at >= ?
		GROUP BY u.id, u.username, u.display_name, u.quota_limit, u.quota_used, u.quota_type
		ORDER BY total_cost DESC
		LIMIT ?
	`, curStart, curStart, curStart, curStart, curStart, tenantID, prevStart, limit).Scan(&records)
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = []memberUsageRow{}
	}

	list := make([]v1.TenantMemberUsageItem, 0, len(records))
	for _, r := range records {
		item := v1.TenantMemberUsageItem{
			UserId:       r.UserId,
			Username:     r.Username,
			DisplayName:  r.DisplayName,
			Requests:     r.Requests,
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
			TotalCost:    roundUSD(r.TotalCost),
			HasPrevious:  prevAvailable && r.PrevCost > 0,
			HasQuota:     r.QuotaType != "none" && r.QuotaLimit > 0,
		}
		if item.HasPrevious {
			item.DeltaPercent = math.Round((r.TotalCost-r.PrevCost)/r.PrevCost*10000) / 100
		}
		if item.HasQuota {
			item.QuotaPercent = math.Round(r.QuotaUsed/r.QuotaLimit*10000) / 100
		}
		list = append(list, item)
	}

	return &v1.TenantMemberUsageRankingRes{List: list, PrevAvailable: prevAvailable}, nil
}

// TeamHealth returns tenant-wide reliability and performance metrics for the current month.
func (s *sTenant) TeamHealth(ctx context.Context, req *v1.TenantTeamHealthReq) (*v1.TenantTeamHealthRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	monthStart := gtime.Now().Format("Y-m") + "-01 00:00:00"

	var row struct {
		Total            int64   `json:"total"`
		Success          int64   `json:"success"`
		AvgFirstTokenMs  float64 `json:"avg_first_token_ms"`
		P95Ms            float64 `json:"p95_ms"`
		CacheReadTokens  int64   `json:"cache_read_tokens"`
		TotalInputTokens int64   `json:"total_input_tokens"`
	}
	// 分位数在 SQL 端算（照 model_comparison.go 的先例），不把明细拉回 Go —
	// 租户级数据量远大于个人级，不能照搬 personal_dashboard 的 LIMIT 5000 做法
	err := g.DB().Ctx(ctx).Raw(`
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'success') AS success,
			COALESCE(AVG(first_token_ms), 0) AS avg_first_token_ms,
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms)
				FILTER (WHERE status = 'success' AND latency_ms IS NOT NULL), 0) AS p95_ms,
			COALESCE(SUM(cache_read_tokens), 0) AS cache_read_tokens,
			COALESCE(SUM(input_tokens), 0) AS total_input_tokens
		FROM bil_usage_logs
		WHERE tenant_id = ? AND created_at >= ?
	`, tenantID, monthStart).Scan(&row)
	if err != nil {
		return nil, err
	}

	res := &v1.TenantTeamHealthRes{
		TotalRequests:   row.Total,
		P95Ms:           math.Round(row.P95Ms*100) / 100,
		AvgFirstTokenMs: math.Round(row.AvgFirstTokenMs*100) / 100,
		CacheReadTokens: row.CacheReadTokens,
	}
	if row.Total > 0 {
		res.SuccessRate = math.Round(float64(row.Success)/float64(row.Total)*10000) / 10000
	}
	// input_tokens 已统一为「含缓存总输入」口径，直接作分母
	if row.TotalInputTokens > 0 {
		res.CacheHitRatio = math.Round(float64(row.CacheReadTokens)/float64(row.TotalInputTokens)*10000) / 10000
	}
	return res, nil
}

// ProjectBudget returns budget usage for all active projects of the tenant.
func (s *sTenant) ProjectBudget(ctx context.Context, req *v1.TenantProjectBudgetReq) (*v1.TenantProjectBudgetRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)

	rows, err := tenantProjectBudgets(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	list := make([]v1.TenantProjectBudgetItem, 0, len(rows))
	for _, r := range rows {
		item := v1.TenantProjectBudgetItem{
			Id:        r.Id,
			Name:      r.Name,
			Budget:    roundUSD(r.Budget),
			Used:      roundUSD(r.Used),
			HasBudget: r.Budget > 0,
		}
		if r.Budget > 0 {
			item.UsagePercent = math.Round(r.Used/r.Budget*10000) / 100
		}
		list = append(list, item)
	}

	return &v1.TenantProjectBudgetRes{List: list}, nil
}

// projectBudgetRow 是项目预算汇总的中间行。
type projectBudgetRow struct {
	Id     int64   `json:"id"`
	Name   string  `json:"name"`
	Budget float64 `json:"budget"`
	Used   float64 `json:"used"`
}

// tenantProjectBudgets 汇总租户全部活跃项目的预算与累计已用。
//
// 已用金额取自 bil_transactions，与 CheckProjectBudget 的执行口径保持一致 ——
// 若改用 bil_usage_logs，会出现「页面显示未超预算、但项目已被系统停用」的错位。
// 注意 tnt_projects.budget 是累计预算而非月度预算，因此这里不做时间过滤。
func tenantProjectBudgets(ctx context.Context, tenantID int64) ([]projectBudgetRow, error) {
	var rows []projectBudgetRow
	err := g.DB().Ctx(ctx).Raw(`
		SELECT
			p.id,
			p.name,
			COALESCE(p.budget, 0) AS budget,
			COALESCE(t.used, 0) AS used
		FROM tnt_projects p
		LEFT JOIN (
			SELECT project_id, SUM(-amount) AS used
			FROM bil_transactions
			WHERE tenant_id = ? AND type = 'consume' AND project_id IS NOT NULL
			GROUP BY project_id
		) t ON t.project_id = p.id
		WHERE p.tenant_id = ? AND p.status = 'active'
		ORDER BY (CASE WHEN COALESCE(p.budget, 0) > 0
			THEN COALESCE(t.used, 0) / p.budget ELSE 0 END) DESC
	`, tenantID, tenantID).Scan(&rows)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []projectBudgetRow{}
	}
	return rows, nil
}
