package tenant

import (
	"context"
	"math"
	"time"

	"github.com/gogf/gf/v2/frame/g"

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

	// Query 3: latency percentiles + cache stats
	type latencyRow struct {
		AvgMs             float64 `json:"avg_ms"`
		AvgFirstTokenMs   float64 `json:"avg_first_token_ms"`
		CacheCreationTkns int64   `json:"cache_creation_tokens"`
		CacheReadTkns     int64   `json:"cache_read_tokens"`
		TotalInputTkns    int64   `json:"total_input_tokens"`
	}
	var lat latencyRow
	err = g.DB().Ctx(ctx).Raw(`
		SELECT
			COALESCE(AVG(latency_ms), 0) as avg_ms,
			COALESCE(AVG(first_token_ms), 0) as avg_first_token_ms,
			COALESCE(SUM(cache_creation_tokens), 0) as cache_creation_tokens,
			COALESCE(SUM(cache_read_tokens), 0) as cache_read_tokens,
			COALESCE(SUM(input_tokens), 0) as total_input_tokens
		FROM bil_usage_logs
		WHERE user_id = ? AND tenant_id = ? AND created_at >= ?
	`, userID, tenantID, monthStart).Scan(&lat)
	if err != nil {
		return nil, err
	}

	// Percentile calculation: fetch latency values and compute in Go
	latency := v1.PersonalLatency{
		AvgMs:           math.Round(lat.AvgMs*100) / 100,
		AvgFirstTokenMs: math.Round(lat.AvgFirstTokenMs*100) / 100,
	}
	var latencyVals []float64
	err = g.DB().Ctx(ctx).Raw(`
		SELECT latency_ms FROM bil_usage_logs
		WHERE user_id = ? AND tenant_id = ? AND created_at >= ? AND latency_ms IS NOT NULL AND status = 'success'
		ORDER BY latency_ms ASC
		LIMIT 5000
	`, userID, tenantID, monthStart).Scan(&latencyVals)
	if err == nil && len(latencyVals) > 0 {
		latency.P50Ms = math.Round(percentile(latencyVals, 0.50)*100) / 100
		latency.P95Ms = math.Round(percentile(latencyVals, 0.95)*100) / 100
		latency.P99Ms = math.Round(percentile(latencyVals, 0.99)*100) / 100
	}

	cache := v1.PersonalCache{
		CacheCreationTokens: lat.CacheCreationTkns,
		CacheReadTokens:     lat.CacheReadTkns,
		TotalInputTokens:    lat.TotalInputTkns,
	}
	totalForRatio := float64(lat.TotalInputTkns + lat.CacheCreationTkns + lat.CacheReadTkns)
	if totalForRatio > 0 {
		cache.HitRatio = math.Round(float64(lat.CacheReadTkns)/totalForRatio*10000) / 10000
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
	err = g.DB().Ctx(ctx).Raw(`
		SELECT quota_type, COALESCE(quota_limit, 0) as quota_limit,
			COALESCE(quota_used, 0) as quota_used, quota_period,
			COALESCE(TO_CHAR(quota_reset_at, 'YYYY-MM-DD HH24:MI:SS'), '') as quota_reset_at
		FROM tnt_users WHERE id = ? AND tenant_id = ?
	`, userID, tenantID).Scan(&qRow)
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
		ErrorRate:    errorRate,
		Latency:      latency,
		Cache:        cache,
		RequestTypes: reqTypeItems,
		Quota:        quota,
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

// percentile computes the p-th percentile from a sorted slice of float64 values.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	rank := p * float64(n-1)
	lower := int(math.Floor(rank))
	upper := lower + 1
	if upper >= n {
		return sorted[n-1]
	}
	frac := rank - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}
