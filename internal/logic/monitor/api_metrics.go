package monitor

import (
	"context"
	"fmt"
	"github.com/qianfree/team-api/internal/dao"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// APIMetricsResult holds aggregated API metrics.
type APIMetricsResult struct {
	QPS        float64          `json:"qps"`
	TPM        float64          `json:"tpm"`
	Latency    LatencyMetrics   `json:"latency"`
	ErrorRates ErrorRateMetrics `json:"error_rates"`
}

// LatencyMetrics holds latency percentile data.
type LatencyMetrics struct {
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
	Avg float64 `json:"avg"`
}

// ErrorRateMetrics holds error rate data.
type ErrorRateMetrics struct {
	Rate4xx float64 `json:"rate_4xx"`
	Rate5xx float64 `json:"rate_5xx"`
	Total   float64 `json:"total"`
}

// GetAPIMetrics aggregates API metrics from bil_usage_logs for the last N minutes.
func GetAPIMetrics(ctx context.Context, minutes int) (*APIMetricsResult, error) {
	if minutes <= 0 {
		minutes = 5
	}
	if minutes > 60 {
		minutes = 60
	}

	since := time.Now().Add(-time.Duration(minutes) * time.Minute)
	sinceStr := since.Format("2006-01-02 15:04:05")

	// QPS + TPM + error rates
	type apiStats struct {
		Total      int   `json:"total"`
		Errors     int   `json:"errors"`
		TotalToken int64 `json:"total_tokens"`
	}
	var stats apiStats
	err := dao.BilUsageLogs.Ctx(ctx).
		Where("created_at >= ?", sinceStr).
		Fields("COUNT(*) as total, COALESCE(SUM(CASE WHEN status != 'success' THEN 1 ELSE 0 END), 0) as errors, COALESCE(SUM(COALESCE(input_tokens,0) + COALESCE(output_tokens,0)), 0) as total_tokens").
		Scan(&stats)
	if err != nil {
		return nil, err
	}

	seconds := float64(minutes * 60)
	qps := float64(stats.Total) / seconds
	tpm := float64(stats.TotalToken) / float64(minutes)
	errorRate := float64(0)
	if stats.Total > 0 {
		errorRate = float64(stats.Errors) / float64(stats.Total) * 100
	}

	// Latency percentiles
	type latencyRow struct {
		P50 float64 `json:"p50"`
		P95 float64 `json:"p95"`
		P99 float64 `json:"p99"`
		Avg float64 `json:"avg"`
	}
	var lat latencyRow
	err = g.DB().Ctx(ctx).Raw(fmt.Sprintf(`
		SELECT
			COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY latency_ms), 0) as p50,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms), 0) as p95,
			COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_ms), 0) as p99,
			COALESCE(AVG(latency_ms), 0) as avg
		FROM bil_usage_logs
		WHERE created_at >= '%s' AND latency_ms IS NOT NULL
	`, sinceStr)).Scan(&lat)
	if err != nil {
		g.Log().Warningf(ctx, "get latency metrics: %v", err)
	}

	return &APIMetricsResult{
		QPS: qps,
		TPM: tpm,
		Latency: LatencyMetrics{
			P50: lat.P50,
			P95: lat.P95,
			P99: lat.P99,
			Avg: lat.Avg,
		},
		ErrorRates: ErrorRateMetrics{
			Total: errorRate,
		},
	}, nil
}

// GetTrafficCurve returns per-minute traffic data for the last N minutes.
func GetTrafficCurve(ctx context.Context, minutes int) ([]map[string]any, error) {
	if minutes <= 0 {
		minutes = 30
	}
	if minutes > 60 {
		minutes = 60
	}

	since := time.Now().Add(-time.Duration(minutes) * time.Minute).Format("2006-01-02 15:04:05")

	result, err := g.DB().Ctx(ctx).Raw(fmt.Sprintf(`
		SELECT
			DATE_TRUNC('minute', created_at) as time,
			COUNT(*) as requests,
			COALESCE(SUM(COALESCE(input_tokens,0) + COALESCE(output_tokens,0)), 0) as tokens,
			COALESCE(AVG(latency_ms), 0) as avg_latency
		FROM bil_usage_logs
		WHERE created_at >= '%s'
		GROUP BY DATE_TRUNC('minute', created_at)
		ORDER BY time ASC
	`, since)).All()
	if err != nil {
		return nil, err
	}

	records := result.List()

	return records, nil
}

// GetLatencyHistogram returns P50/P95/P99 latency for the last N minutes.
func GetLatencyHistogram(ctx context.Context, minutes int) (map[string]any, error) {
	if minutes <= 0 {
		minutes = 5
	}
	if minutes > 60 {
		minutes = 60
	}

	since := time.Now().Add(-time.Duration(minutes) * time.Minute).Format("2006-01-02 15:04:05")

	type latRow struct {
		P50 float64 `json:"p50"`
		P95 float64 `json:"p95"`
		P99 float64 `json:"p99"`
		Avg float64 `json:"avg"`
		Max float64 `json:"max"`
		Min float64 `json:"min"`
	}
	var lat latRow
	err := g.DB().Ctx(ctx).Raw(fmt.Sprintf(`
		SELECT
			COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY latency_ms), 0) as p50,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms), 0) as p95,
			COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_ms), 0) as p99,
			COALESCE(AVG(latency_ms), 0) as avg,
			COALESCE(MAX(latency_ms), 0) as max,
			COALESCE(MIN(latency_ms), 0) as min
		FROM bil_usage_logs
		WHERE created_at >= '%s' AND latency_ms IS NOT NULL
	`, since)).Scan(&lat)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"p50": lat.P50,
		"p95": lat.P95,
		"p99": lat.P99,
		"avg": lat.Avg,
		"max": lat.Max,
		"min": lat.Min,
	}, nil
}

// GetModelDistribution returns model usage distribution for the last N minutes.
func GetModelDistribution(ctx context.Context, minutes int) ([]map[string]any, error) {
	if minutes <= 0 {
		minutes = 60
	}

	since := time.Now().Add(-time.Duration(minutes) * time.Minute).Format("2006-01-02 15:04:05")

	result, err := g.DB().Ctx(ctx).Raw(fmt.Sprintf(`
		SELECT
			model_name,
			COUNT(*) as requests,
			COALESCE(SUM(COALESCE(input_tokens,0) + COALESCE(output_tokens,0)), 0) as tokens,
			COALESCE(SUM(total_cost), 0) as total_cost
		FROM bil_usage_logs
		WHERE created_at >= '%s'
		GROUP BY model_name
		ORDER BY requests DESC
		LIMIT 20
	`, since)).All()
	if err != nil {
		return nil, err
	}

	records := result.List()

	return records, nil
}

// GetTenantRanking returns top tenants by request count for the last N minutes.
func GetTenantRanking(ctx context.Context, minutes int) ([]map[string]any, error) {
	if minutes <= 0 {
		minutes = 60
	}

	since := time.Now().Add(-time.Duration(minutes) * time.Minute).Format("2006-01-02 15:04:05")

	result, err := g.DB().Ctx(ctx).Raw(fmt.Sprintf(`
		SELECT
			t.id as tenant_id,
			t.name as tenant_name,
			COUNT(*) as requests,
			COALESCE(SUM(ul.total_cost), 0) as total_cost
		FROM bil_usage_logs ul
		JOIN tnt_tenants t ON t.id = ul.tenant_id
		WHERE ul.created_at >= '%s'
		GROUP BY t.id, t.name
		ORDER BY requests DESC
		LIMIT 10
	`, since)).All()
	if err != nil {
		return nil, err
	}

	records := result.List()

	return records, nil
}

// GetErrorRate returns the current API error rate percentage.
func GetErrorRate(ctx context.Context) (float64, error) {
	since := time.Now().Add(-5 * time.Minute).Format("2006-01-02 15:04:05")

	type errRow struct {
		Total  int `json:"total"`
		Errors int `json:"errors"`
	}
	var row errRow
	err := dao.BilUsageLogs.Ctx(ctx).
		Where("created_at >= ?", since).
		Fields("COUNT(*) as total, COALESCE(SUM(CASE WHEN status != 'success' THEN 1 ELSE 0 END), 0) as errors").
		Scan(&row)
	if err != nil {
		return 0, err
	}
	if row.Total == 0 {
		return 0, nil
	}
	return float64(row.Errors) / float64(row.Total) * 100, nil
}

// GetP99Latency returns the current P99 latency in milliseconds.
func GetP99Latency(ctx context.Context) (float64, error) {
	since := time.Now().Add(-5 * time.Minute).Format("2006-01-02 15:04:05")

	type p99Row struct {
		P99 float64 `json:"p99"`
	}
	var row p99Row
	err := g.DB().Ctx(ctx).Raw(fmt.Sprintf(`
		SELECT COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_ms), 0) as p99
		FROM bil_usage_logs
		WHERE created_at >= '%s' AND latency_ms IS NOT NULL
	`, since)).Scan(&row)
	if err != nil {
		return 0, err
	}
	return row.P99, nil
}

// GetQPS returns the current requests per second.
func GetQPS(ctx context.Context) (float64, error) {
	since := time.Now().Add(-1 * time.Minute).Format("2006-01-02 15:04:05")

	count, err := dao.BilUsageLogs.Ctx(ctx).
		Where("created_at >= ?", since).
		Count()
	if err != nil {
		return 0, err
	}
	return float64(count) / 60.0, nil
}
