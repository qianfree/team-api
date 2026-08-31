package tenant

import (
	"context"
	"math"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/shopspring/decimal"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/middleware"
)

// PersonalDashboard returns the personal dashboard overview for the current user.
func (s *sTenant) PersonalDashboard(ctx context.Context, req *v1.PersonalDashboardReq) (*v1.PersonalDashboardRes, error) {
	userID := middleware.GetUserID(ctx)
	tenantID := middleware.GetTenantID(ctx)

	now := time.Now()
	todayStart := now.Format("2006-01-02") + " 00:00:00"
	monthStart := now.Format("2006-01") + "-01 00:00:00"

	// Query 1: today + month basic stats
	type statsRow struct {
		TodayRequests     int     `json:"today_requests"`
		TodayInputTokens  int64   `json:"today_input_tokens"`
		TodayOutputTokens int64   `json:"today_output_tokens"`
		TodayTotalCost    float64 `json:"today_total_cost"`
		MonthRequests     int     `json:"month_requests"`
		MonthInputTokens  int64   `json:"month_input_tokens"`
		MonthOutputTokens int64   `json:"month_output_tokens"`
		MonthTotalCost    float64 `json:"month_total_cost"`
	}
	var stats statsRow
	err := g.DB().Ctx(ctx).Raw(`
		SELECT
			COUNT(CASE WHEN created_at >= ? THEN 1 END) as today_requests,
			COALESCE(SUM(CASE WHEN created_at >= ? THEN input_tokens ELSE 0 END), 0) as today_input_tokens,
			COALESCE(SUM(CASE WHEN created_at >= ? THEN output_tokens ELSE 0 END), 0) as today_output_tokens,
			COALESCE(SUM(CASE WHEN created_at >= ? THEN total_cost ELSE 0 END), 0) as today_total_cost,
			COUNT(*) as month_requests,
			COALESCE(SUM(input_tokens), 0) as month_input_tokens,
			COALESCE(SUM(output_tokens), 0) as month_output_tokens,
			COALESCE(SUM(total_cost), 0) as month_total_cost
		FROM bil_usage_logs
		WHERE user_id = ? AND tenant_id = ? AND created_at >= ?
	`, todayStart, todayStart, todayStart, todayStart, userID, tenantID, monthStart).Scan(&stats)
	if err != nil {
		return nil, err
	}

	// Query 2: error rate + request type distribution
	type reqTypeRow struct {
		RequestType int `json:"request_type"`
		Total       int `json:"total"`
		Success     int `json:"success"`
		ErrCount    int `json:"error"`
		Timeout     int `json:"timeout"`
		Cancelled   int `json:"cancelled"`
	}
	var reqTypeRows []reqTypeRow
	err = g.DB().Ctx(ctx).Raw(`
		SELECT
			request_type,
			COUNT(*) as total,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success,
			SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as error,
			SUM(CASE WHEN status = 'timeout' THEN 1 ELSE 0 END) as timeout,
			SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END) as cancelled
		FROM bil_usage_logs
		WHERE user_id = ? AND tenant_id = ? AND created_at >= ?
		GROUP BY request_type
	`, userID, tenantID, monthStart).Scan(&reqTypeRows)
	if err != nil {
		return nil, err
	}

	errorRate := v1.PersonalErrorRate{}
	reqTypeItems := []v1.PersonalReqTypeItem{}
	reqTypeLabels := map[int]string{1: "同步", 2: "流式", 3: "异步", 4: "WebSocket"}
	reqTypeNames := map[int]string{1: "sync", 2: "stream", 3: "async", 4: "websocket"}
	totalReqs := 0
	for _, r := range reqTypeRows {
		errorRate.Total += r.Total
		errorRate.Success += r.Success
		errorRate.Error += r.ErrCount
		errorRate.Timeout += r.Timeout
		errorRate.Cancelled += r.Cancelled
		totalReqs += r.Total
		reqTypeItems = append(reqTypeItems, v1.PersonalReqTypeItem{
			Type:       reqTypeNames[r.RequestType],
			Label:      reqTypeLabels[r.RequestType],
			Requests:   r.Total,
			Percentage: 0,
		})
	}
	if errorRate.Total > 0 {
		errorRate.Rate = math.Round(float64(errorRate.Success)/float64(errorRate.Total)*10000) / 10000
	}
	for i := range reqTypeItems {
		if totalReqs > 0 {
			reqTypeItems[i].Percentage = math.Round(float64(reqTypeItems[i].Requests)/float64(totalReqs)*10000) / 100
		}
	}

	// Query 3: 延迟与缓存统计，按模型分组。
	//
	// 分组是为了推导「缓存省了多少钱」：input_cost 只给非缓存的基础输入计价
	// （见 billing/pricing.go），因此模型内的混合输入单价 = input_cost / base_input_tokens；
	// 用该单价给 cache_read_tokens 重新计价、再减去实际缓存费用，差额即节省额。
	// 跨模型混算会严重失真（不同模型单价可差十几倍），所以必须逐模型算再求和。
	//
	// 分组后把 AVG 换成 SUM/COUNT 在 Go 里重新合并——COUNT(*) FILTER (WHERE x IS NOT NULL)
	// 与 AVG 的分母口径一致，因此合并结果与原先的整体 AVG 完全相等。
	type cacheRow struct {
		LatencyCount      int64           `json:"latency_count"`
		FirstTokenCount   int64           `json:"first_token_count"`
		SumLatencyMs      float64         `json:"sum_latency_ms"`
		SumFirstTokenMs   float64         `json:"sum_first_token_ms"`
		CacheCreationTkns int64           `json:"cache_creation_tokens"`
		CacheReadTkns     int64           `json:"cache_read_tokens"`
		TotalInputTkns    int64           `json:"total_input_tokens"`
		BaseInputTkns     int64           `json:"base_input_tokens"`
		InputCost         decimal.Decimal `json:"input_cost"`
		CacheReadCost     decimal.Decimal `json:"cache_read_cost"`
	}
	var cacheRows []cacheRow
	err = g.DB().Ctx(ctx).Raw(`
		SELECT
			COUNT(*) FILTER (WHERE latency_ms IS NOT NULL) AS latency_count,
			COUNT(*) FILTER (WHERE first_token_ms IS NOT NULL) AS first_token_count,
			COALESCE(SUM(latency_ms), 0) AS sum_latency_ms,
			COALESCE(SUM(first_token_ms), 0) AS sum_first_token_ms,
			COALESCE(SUM(cache_creation_tokens), 0) AS cache_creation_tokens,
			COALESCE(SUM(cache_read_tokens), 0) AS cache_read_tokens,
			COALESCE(SUM(input_tokens), 0) AS total_input_tokens,
			COALESCE(SUM(GREATEST(input_tokens - cache_read_tokens - cache_creation_tokens, 0)), 0) AS base_input_tokens,
			COALESCE(SUM(input_cost), 0) AS input_cost,
			COALESCE(SUM(cache_read_cost), 0) AS cache_read_cost
		FROM bil_usage_logs
		WHERE user_id = ? AND tenant_id = ? AND created_at >= ?
		GROUP BY model_name
	`, userID, tenantID, monthStart).Scan(&cacheRows)
	if err != nil {
		return nil, err
	}

	var (
		latencyCount      int64
		firstTokenCount   int64
		sumLatencyMs      float64
		sumFirstTokenMs   float64
		cacheCreationTkns int64
		cacheReadTkns     int64
		totalInputTkns    int64
	)
	savedCost := billing.Zero
	for _, r := range cacheRows {
		latencyCount += r.LatencyCount
		firstTokenCount += r.FirstTokenCount
		sumLatencyMs += r.SumLatencyMs
		sumFirstTokenMs += r.SumFirstTokenMs
		cacheCreationTkns += r.CacheCreationTkns
		cacheReadTkns += r.CacheReadTkns
		totalInputTkns += r.TotalInputTkns

		if r.BaseInputTkns <= 0 || r.CacheReadTkns <= 0 {
			continue
		}
		unitPrice := billing.DivideMoney(r.InputCost, decimal.NewFromInt(r.BaseInputTkns))
		saved := billing.SubtractMoney(
			billing.MultiplyMoney(unitPrice, decimal.NewFromInt(r.CacheReadTkns)),
			r.CacheReadCost,
		)
		if billing.IsPositive(saved) {
			savedCost = billing.AddMoney(savedCost, saved)
		}
	}

	// Percentile calculation: fetch latency values and compute in Go
	latency := v1.PersonalLatency{}
	if latencyCount > 0 {
		latency.AvgMs = math.Round(sumLatencyMs/float64(latencyCount)*100) / 100
	}
	if firstTokenCount > 0 {
		latency.AvgFirstTokenMs = math.Round(sumFirstTokenMs/float64(firstTokenCount)*100) / 100
	}
	// 分位数交给 SQL 算。
	//
	// 原实现是 `ORDER BY latency_ms ASC LIMIT 5000` 再在 Go 里求分位，
	// 取到的是「最快的 5000 条」——月成功调用超过 5000 次的用户，报出来的 P95
	// 实际约等于真实分布的第 (5000/N)×95 百分位，而且错在「让人安心」的方向：
	// 月调 5 万次的用户会看到 P95 几百毫秒，真实值可能是几秒。
	// PERCENTILE_CONT 在库内对全量排序，无论多少行都是准确值。
	var latRow struct {
		P50 float64 `json:"p50"`
		P95 float64 `json:"p95"`
		P99 float64 `json:"p99"`
	}
	err = g.DB().Ctx(ctx).Raw(`
		SELECT
			COALESCE(PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY latency_ms), 0) AS p50,
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms), 0) AS p95,
			COALESCE(PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY latency_ms), 0) AS p99
		FROM bil_usage_logs
		WHERE user_id = ? AND tenant_id = ? AND created_at >= ?
			AND latency_ms IS NOT NULL AND status = 'success'
	`, userID, tenantID, monthStart).Scan(&latRow)
	if err != nil {
		return nil, err
	}
	latency.P50Ms = math.Round(latRow.P50*100) / 100
	latency.P95Ms = math.Round(latRow.P95*100) / 100
	latency.P99Ms = math.Round(latRow.P99*100) / 100

	cache := v1.PersonalCache{
		CacheCreationTokens: cacheCreationTkns,
		CacheReadTokens:     cacheReadTkns,
		TotalInputTokens:    totalInputTkns,
		SavedCost:           billing.InexactFloat64(billing.RoundMoney(savedCost)),
	}
	// input_tokens 已统一为「含缓存总输入」口径（Claude 渠道入库时补加缓存），分母直接取总和
	if totalInputTkns > 0 {
		cache.HitRatio = math.Round(float64(cacheReadTkns)/float64(totalInputTkns)*10000) / 10000
	}

	// Query 4: quota status
	var quota *v1.PersonalQuotaStatus
	type quotaRow struct {
		QuotaType    string  `json:"quota_type"`
		QuotaLimit   float64 `json:"quota_limit"`
		QuotaUsed    float64 `json:"quota_used"`
		QuotaPeriod  string  `json:"quota_period"`
		QuotaResetAt string  `json:"quota_reset_at"`
	}
	var qRow *quotaRow
	err = dao.TntUsers.Ctx(ctx).
		Where("id", userID).Where("tenant_id", tenantID).
		Fields("quota_type, COALESCE(quota_limit, 0) as quota_limit, COALESCE(quota_used, 0) as quota_used, quota_period, COALESCE(TO_CHAR(quota_reset_at, 'YYYY-MM-DD HH24:MI:SS'), '') as quota_reset_at").
		Scan(&qRow)
	if err == nil && qRow != nil && qRow.QuotaType != "" && qRow.QuotaType != "none" {
		q := &v1.PersonalQuotaStatus{
			QuotaType:   qRow.QuotaType,
			QuotaLimit:  qRow.QuotaLimit,
			QuotaUsed:   qRow.QuotaUsed,
			Period:      qRow.QuotaPeriod,
			NextResetAt: qRow.QuotaResetAt,
		}
		if qRow.QuotaLimit > 0 {
			q.UsagePercent = math.Round(qRow.QuotaUsed/qRow.QuotaLimit*10000) / 100
		}
		quota = q
	}

	// Query 5: 最近失败的调用（排障入口）。
	// created_at >= monthStart 不是装饰：bil_usage_logs 没有 user_id 索引，
	// 必须靠时间下界把扫描压在 (tenant_id, created_at) 索引的窄区间内。
	// 这里用 status IN 枚举而非 <> 'success'，以便走 (status, created_at) 索引。
	var failures []v1.PersonalFailureItem
	err = g.DB().Ctx(ctx).Raw(`
		SELECT
			status,
			model_name,
			LEFT(COALESCE(error_message, ''), 500) AS error_message,
			TO_CHAR(created_at, 'YYYY-MM-DD HH24:MI:SS') AS created_at
		FROM bil_usage_logs
		WHERE user_id = ? AND tenant_id = ? AND created_at >= ?
			AND status IN ('error', 'timeout', 'cancelled')
		ORDER BY created_at DESC
		LIMIT 5
	`, userID, tenantID, monthStart).Scan(&failures)
	if err != nil {
		return nil, err
	}
	if failures == nil {
		failures = []v1.PersonalFailureItem{}
	}

	return &v1.PersonalDashboardRes{
		Today: v1.PersonalDayStats{
			Requests:     stats.TodayRequests,
			InputTokens:  stats.TodayInputTokens,
			OutputTokens: stats.TodayOutputTokens,
			TotalCost:    stats.TodayTotalCost,
		},
		Month: v1.PersonalDayStats{
			Requests:     stats.MonthRequests,
			InputTokens:  stats.MonthInputTokens,
			OutputTokens: stats.MonthOutputTokens,
			TotalCost:    stats.MonthTotalCost,
		},
		ErrorRate:      errorRate,
		Latency:        latency,
		Cache:          cache,
		RequestTypes:   reqTypeItems,
		Quota:          quota,
		RecentFailures: failures,
	}, nil
}

// PersonalTokenTrends returns daily token usage trends for the current user.
func (s *sTenant) PersonalTokenTrends(ctx context.Context, req *v1.PersonalTokenTrendsReq) (*v1.PersonalTokenTrendsRes, error) {
	userID := middleware.GetUserID(ctx)
	tenantID := middleware.GetTenantID(ctx)
	days := req.Days
	if days <= 0 || days > 90 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02") + " 00:00:00"

	var records []v1.PersonalTrendPoint
	err := g.DB().Ctx(ctx).Raw(`
		SELECT
			TO_CHAR(DATE(created_at), 'YYYY-MM-DD') as date,
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COUNT(*) as requests,
			COALESCE(SUM(total_cost), 0) as total_cost
		FROM bil_usage_logs
		WHERE user_id = ? AND tenant_id = ? AND created_at >= ?
		GROUP BY DATE(created_at)
		ORDER BY DATE(created_at) ASC
	`, userID, tenantID, startDate).Scan(&records)
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = []v1.PersonalTrendPoint{}
	}

	return &v1.PersonalTokenTrendsRes{List: records}, nil
}

// PersonalModelDistribution returns model usage distribution for the current user.
func (s *sTenant) PersonalModelDistribution(ctx context.Context, req *v1.PersonalModelDistReq) (*v1.PersonalModelDistRes, error) {
	userID := middleware.GetUserID(ctx)
	tenantID := middleware.GetTenantID(ctx)
	days := req.Days
	if days <= 0 || days > 90 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02") + " 00:00:00"

	var records []v1.PersonalModelItem
	err := g.DB().Ctx(ctx).Raw(`
		SELECT
			model_name,
			COUNT(*) as requests,
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost
		FROM bil_usage_logs
		WHERE user_id = ? AND tenant_id = ? AND created_at >= ?
		GROUP BY model_name
		ORDER BY total_cost DESC
		LIMIT 20
	`, userID, tenantID, startDate).Scan(&records)
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = []v1.PersonalModelItem{}
	}

	return &v1.PersonalModelDistRes{List: records}, nil
}

// PersonalApiKeyUsage returns per-API-key usage breakdown for the current user.
func (s *sTenant) PersonalApiKeyUsage(ctx context.Context, req *v1.PersonalApiKeyUsageReq) (*v1.PersonalApiKeyUsageRes, error) {
	userID := middleware.GetUserID(ctx)
	tenantID := middleware.GetTenantID(ctx)
	days := req.Days
	if days <= 0 || days > 90 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02") + " 00:00:00"

	var records []v1.PersonalApiKeyItem
	err := g.DB().Ctx(ctx).Raw(`
		SELECT
			ul.api_key_id,
			COALESCE(k.name, '未命名密钥') as key_name,
			COALESCE(k.key_prefix, '') as key_prefix,
			COUNT(*) as requests,
			COALESCE(SUM(ul.input_tokens), 0) as input_tokens,
			COALESCE(SUM(ul.output_tokens), 0) as output_tokens,
			COALESCE(SUM(ul.total_cost), 0) as total_cost
		FROM bil_usage_logs ul
		LEFT JOIN api_keys k ON k.id = ul.api_key_id
		WHERE ul.user_id = ? AND ul.tenant_id = ? AND ul.created_at >= ?
		GROUP BY ul.api_key_id, k.name, k.key_prefix
		ORDER BY total_cost DESC
		LIMIT 20
	`, userID, tenantID, startDate).Scan(&records)
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = []v1.PersonalApiKeyItem{}
	}

	return &v1.PersonalApiKeyUsageRes{List: records}, nil
}
