package monitor

import (
	"context"
	"fmt"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/middleware"
	"math"
	"sort"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/logic/common"
)

// ⚠️ 平台域查询约定（P2-13）
//
// 本文件中的 GetAPIMetrics / GetTrafficCurve / GetLatencyHistogram / GetErrorRate /
// GetP99Latency / GetQPS 等聚合查询基于 bil_usage_logs 做【跨租户平台级】统计，
// 查询不带 tenant_id 过滤——这是管理后台（平台运营视角）所需的。
//
// 风险：这些函数均为 exported，若被错误地复用到租户控制台接口，会泄露其它租户的用量数据。
// 防护：除 admin 路由鉴权外，HTTP 入口（sMonitor 仪表盘相关方法）额外调用
// requireAdminScope 做调用方身份校验，即便有人误把接口接到租户路由也会被拒绝。
// 内部 cron 上下文（gctx.New，无 userType）放行。

// requireAdminScope 断言调用方为平台运营域（admin 或内部上下文）。
// 用于保护跨租户聚合的监控查询，防止被租户域误用导致数据泄露（P2-13）。
func requireAdminScope(ctx context.Context) error {
	switch middleware.GetUserType(ctx) {
	case "admin", "":
		// admin：平台运营后台；""：内部 cron/任务上下文（gctx.New 未设 userType），均放行
		return nil
	case "tenant":
		return fmt.Errorf("监控聚合查询为平台域只读接口，禁止租户域访问")
	default:
		return fmt.Errorf("未知调用方域，拒绝访问平台域监控查询: user_type=%s", middleware.GetUserType(ctx))
	}
}

// ===================== 流量流向桑基图（bil_usage_daily） =====================

// trafficFlowMaxDays 桑基图最大查询跨度（天）
const trafficFlowMaxDays = 90

// trafficFlowTopN 租户/模型维度保留的节点数，其余归入「其他」
const trafficFlowTopN = 12

// trafficFlowRow 桑基图原始聚合行：bil_usage_daily 按租户/模型/渠道/状态聚合
type trafficFlowRow struct {
	TenantID    int64   `json:"tenant_id"`
	TenantName  string  `json:"tenant_name"`
	ModelName   string  `json:"model_name"`
	ChannelID   int64   `json:"channel_id"`
	ChannelName string  `json:"channel_name"`
	Status      string  `json:"status"`
	Requests    int64   `json:"requests"`
	Tokens      int64   `json:"tokens"`
	Cost        float64 `json:"cost"`
}

// normalizeMonitorDateRange 规范化监控类接口的日期范围，返回 YYYY-MM-DD 的 [start, end]（含）。
// 缺省近 30 天；end 不超过今天；跨度上限 trafficFlowMaxDays。流量桑基图与模型性能共用。
func normalizeMonitorDateRange(startDate, endDate string) (string, string) {
	const layout = "2006-01-02"
	today := time.Now()
	end, err := time.Parse(layout, endDate)
	if err != nil || end.After(today) {
		end = today
	}
	start, err := time.Parse(layout, startDate)
	if err != nil || start.After(end) {
		start = end.AddDate(0, 0, -29)
	}
	// 跨度上限
	if end.Sub(start).Hours()/24 > float64(trafficFlowMaxDays-1) {
		start = end.AddDate(0, 0, -(trafficFlowMaxDays - 1))
	}
	return start.Format(layout), end.Format(layout)
}

// GetTrafficFlow 从 bil_usage_daily 聚合出流量流向桑基图数据。
// 维度链：租户 → 模型 → 渠道 → 状态；指标 metric: cost|tokens|requests。
// 租户与模型维度做 Top-N + 「其他」归桶，避免高基数导致图形过密。
func GetTrafficFlow(ctx context.Context, startDate, endDate, metric string) (any, error) {
	const query = `
		SELECT
			d.tenant_id                                                                       AS tenant_id,
			COALESCE(t.name, '(已删除 ' || d.tenant_id || ')')                                  AS tenant_name,
			d.model_name                                                                      AS model_name,
			d.channel_id                                                                      AS channel_id,
			COALESCE(c.name, CASE WHEN d.channel_id = 0 THEN '(无渠道)'
				ELSE '(已删除 ' || d.channel_id || ')' END)                                     AS channel_name,
			d.status                                                                          AS status,
			COALESCE(SUM(d.request_count), 0)                                                 AS requests,
			COALESCE(SUM(d.input_tokens + d.output_tokens), 0)                                AS tokens,
			COALESCE(SUM(d.total_cost), 0)                                                    AS cost
		FROM bil_usage_daily d
		LEFT JOIN tnt_tenants t ON t.id = d.tenant_id
		LEFT JOIN chn_channels c ON c.id = d.channel_id
		WHERE d.stat_date >= $1 AND d.stat_date <= $2
		GROUP BY d.tenant_id, t.name, d.model_name, d.channel_id, c.name, d.status
	`
	var rows []trafficFlowRow
	if err := g.DB().Ctx(ctx).Raw(query, startDate, endDate).Scan(&rows); err != nil {
		return nil, err
	}
	return buildTrafficFlowSankey(rows, metric), nil
}

// ===================== 模型性能指标（bil_usage_daily） =====================

// modelPerfRow 模型性能聚合行（bil_usage_daily 按 model_name 分组）
type modelPerfRow struct {
	ModelName       string  `json:"model_name"`
	RequestCount    int64   `json:"request_count"`
	SuccessCount    int64   `json:"success_count"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	TotalCost       float64 `json:"total_cost"`
	SumLatencyMs    int64   `json:"sum_latency_ms"`
	SumFirstTokenMs int64   `json:"sum_first_token_ms"`
}

// gradeSuccessRate 按成功率返回分级。阈值：≥99 excellent、≥95 good、≥90 warning、<90 critical。
func gradeSuccessRate(rate float64) string {
	switch {
	case rate >= 99:
		return "excellent"
	case rate >= 95:
		return "good"
	case rate >= 90:
		return "warning"
	default:
		return "critical"
	}
}

// perfModelAgg 模型性能合并聚合值（历史日表 + 当日 Redis 热桶）。
type perfModelAgg struct {
	requestCount    int64
	successCount    int64
	inputTokens     int64
	outputTokens    int64
	sumLatencyMs    int64
	sumFirstTokenMs int64
	totalCost       float64
}

// GetModelPerformance 从 bil_usage_daily 按模型聚合历史性能指标，并合并当日 Redis 实时热桶
// （见 common.RecordModelPerfMetrics，桶粒度小时），使页面当天即可看到最新数据，不再滞后一天。
// 指标口径与 bil_usage_daily 一致：延迟按 SUM 求和、视图以 SUM/COUNT 求均值（分位数不可跨桶加，p95/p99 留后续）。
func GetModelPerformance(ctx context.Context, startDate, endDate string) (any, error) {
	today := time.Now().Format("2006-01-02")

	// 历史段：stat_date < today（当天由 Redis 热桶实时提供，避免与每日 01:00 的日聚合重复/滞后）
	const query = `
		SELECT
			model_name,
			SUM(request_count)                                              AS request_count,
			SUM(CASE WHEN status = 'success' THEN request_count ELSE 0 END) AS success_count,
			COALESCE(SUM(input_tokens), 0)                                  AS input_tokens,
			COALESCE(SUM(output_tokens), 0)                                 AS output_tokens,
			COALESCE(SUM(total_cost), 0)                                    AS total_cost,
			COALESCE(SUM(sum_latency_ms), 0)                                AS sum_latency_ms,
			COALESCE(SUM(sum_first_token_ms), 0)                            AS sum_first_token_ms
		FROM bil_usage_daily
		WHERE stat_date >= $1 AND stat_date <= $2 AND stat_date < $3 AND model_name <> 'unknown'
		GROUP BY model_name
	`
	var rows []modelPerfRow
	if err := g.DB().Ctx(ctx).Raw(query, startDate, endDate, today).Scan(&rows); err != nil {
		return nil, err
	}

	// 当日 Redis 热桶（best-effort：仅当查询窗口包含今天时参与；Redis 异常则忽略当日数据，退化为历史日表）
	todayCounts := map[string]common.ModelPerfCounter{}
	if endDate == today {
		tc, err := common.GetModelPerfMetricsToday(ctx, today)
		if err != nil {
			g.Log().Warningf(ctx, "model performance today redis read failed, fallback to daily only: %v", err)
		} else {
			todayCounts = tc
		}
	}

	return map[string]any{"list": buildModelPerformanceList(rows, todayCounts)}, nil
}

// buildModelPerformanceList 将历史日表聚合行与当日 Redis 热桶合并为统一的模型性能列表，
// 统一重算派生指标（成功率/均延迟/均首Token/TPS/分级）并按 request_count 降序输出。
func buildModelPerformanceList(rows []modelPerfRow, todayCounts map[string]common.ModelPerfCounter) []map[string]any {
	agg := make(map[string]*perfModelAgg)
	ensure := func(model string) *perfModelAgg {
		a, ok := agg[model]
		if !ok {
			a = &perfModelAgg{}
			agg[model] = a
		}
		return a
	}
	for _, r := range rows {
		if r.ModelName == "" || r.ModelName == "unknown" {
			continue
		}
		a := ensure(r.ModelName)
		a.requestCount += r.RequestCount
		a.successCount += r.SuccessCount
		a.inputTokens += r.InputTokens
		a.outputTokens += r.OutputTokens
		a.sumLatencyMs += r.SumLatencyMs
		a.sumFirstTokenMs += r.SumFirstTokenMs
		a.totalCost += r.TotalCost
	}
	for model, c := range todayCounts {
		if c.Req <= 0 || model == "" || model == "unknown" {
			continue
		}
		a := ensure(model)
		a.requestCount += c.Req
		a.successCount += c.Ok
		a.inputTokens += c.Tin
		a.outputTokens += c.Tout
		a.sumLatencyMs += c.Lat
		a.sumFirstTokenMs += c.Ttft
		a.totalCost += billing.FromMicro(c.CostMicro).InexactFloat64()
	}

	list := make([]map[string]any, 0, len(agg))
	for model, a := range agg {
		var successRate, avgLatency, avgTTFT, tps float64
		if a.requestCount > 0 {
			successRate = float64(a.successCount) * 100 / float64(a.requestCount)
			avgLatency = float64(a.sumLatencyMs) / float64(a.requestCount)
			avgTTFT = float64(a.sumFirstTokenMs) / float64(a.requestCount)
		}
		if a.sumLatencyMs > 0 {
			tps = float64(a.outputTokens) / (float64(a.sumLatencyMs) / 1000)
		}
		list = append(list, map[string]any{
			"model_name":         model,
			"request_count":      a.requestCount,
			"success_count":      a.successCount,
			"success_rate":       math.Round(successRate*100) / 100,
			"grade":              gradeSuccessRate(successRate),
			"input_tokens":       a.inputTokens,
			"output_tokens":      a.outputTokens,
			"total_tokens":       a.inputTokens + a.outputTokens,
			"total_cost":         a.totalCost,
			"avg_latency_ms":     math.Round(avgLatency*100) / 100,
			"avg_first_token_ms": math.Round(avgTTFT*100) / 100,
			"tps":                math.Round(tps*100) / 100,
		})
	}
	// 默认按模型名称升序排列，便于运营按名称定位具体模型；用户可在前端表格点任意列切换排序。
	sort.Slice(list, func(i, j int) bool {
		return list[i]["model_name"].(string) < list[j]["model_name"].(string)
	})
	return list
}

// buildTrafficFlowSankey 将聚合行组装为 ECharts Sankey 的 {nodes, links}。
// 节点名加段前缀（租户/ 模型/ 渠道/ 状态/）保证全局唯一。
func buildTrafficFlowSankey(rows []trafficFlowRow, metric string) map[string]any {
	weight := func(r trafficFlowRow) float64 {
		switch metric {
		case "tokens":
			return float64(r.Tokens)
		case "requests":
			return float64(r.Requests)
		default:
			return r.Cost
		}
	}

	// 计算租户/模型总权重，用于 Top-N 归桶
	tenantTotal := map[string]float64{}
	modelTotal := map[string]float64{}
	for _, r := range rows {
		w := weight(r)
		tenantTotal[r.TenantName] += w
		modelTotal[r.ModelName] += w
	}
	keepTenant := topNKeys(tenantTotal, trafficFlowTopN)
	keepModel := topNKeys(modelTotal, trafficFlowTopN)
	tenantDisplay := func(name string) string {
		if keepTenant[name] {
			return name
		}
		return "(其他租户)"
	}
	modelDisplay := func(name string) string {
		if keepModel[name] {
			return name
		}
		return "(其他模型)"
	}

	const (
		pTenant  = "租户/"
		pModel   = "模型/"
		pChannel = "渠道/"
		pStatus  = "状态/"
	)

	type edge struct{ src, dst string }
	linkWeight := map[edge]float64{}
	nodeDepth := map[string]int{}
	addNode := func(name string, depth int) {
		if _, ok := nodeDepth[name]; !ok {
			nodeDepth[name] = depth
		}
	}
	addLink := func(src, dst string, w float64) {
		if w > 0 {
			linkWeight[edge{src, dst}] += w
		}
	}

	for _, r := range rows {
		w := weight(r)
		if w <= 0 {
			continue
		}
		tn := pTenant + tenantDisplay(r.TenantName)
		mn := pModel + modelDisplay(r.ModelName)
		cn := pChannel + r.ChannelName
		sn := pStatus + r.Status
		addNode(tn, 0)
		addNode(mn, 1)
		addNode(cn, 2)
		addNode(sn, 3)
		addLink(tn, mn, w)
		addLink(mn, cn, w)
		addLink(cn, sn, w)
	}

	nodes := make([]map[string]any, 0, len(nodeDepth))
	for name, depth := range nodeDepth {
		nodes = append(nodes, map[string]any{"name": name, "depth": depth})
	}
	links := make([]map[string]any, 0, len(linkWeight))
	for e, w := range linkWeight {
		links = append(links, map[string]any{
			"source": e.src,
			"target": e.dst,
			"value":  math.Round(w*1e6) / 1e6,
		})
	}

	return map[string]any{
		"metric": metric,
		"nodes":  nodes,
		"links":  links,
	}
}

// topNKeys 返回按值降序前 n 个 key 的集合
func topNKeys(m map[string]float64, n int) map[string]bool {
	type kv struct {
		k string
		v float64
	}
	arr := make([]kv, 0, len(m))
	for k, v := range m {
		arr = append(arr, kv{k, v})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].v > arr[j].v })
	out := make(map[string]bool, n)
	for i := 0; i < n && i < len(arr); i++ {
		out[arr[i].k] = true
	}
	return out
}

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
	err = g.DB().Ctx(ctx).Raw(`
		SELECT
			COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY latency_ms), 0) as p50,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms), 0) as p95,
			COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_ms), 0) as p99,
			COALESCE(AVG(latency_ms), 0) as avg
		FROM bil_usage_logs
		WHERE created_at >= ? AND latency_ms IS NOT NULL
	`, sinceStr).Scan(&lat)
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

	result, err := g.DB().Ctx(ctx).Raw(`
		SELECT
			DATE_TRUNC('minute', created_at) as time,
			COUNT(*) as requests,
			COALESCE(SUM(COALESCE(input_tokens,0) + COALESCE(output_tokens,0)), 0) as tokens,
			COALESCE(AVG(latency_ms), 0) as avg_latency
		FROM bil_usage_logs
		WHERE created_at >= ?
		GROUP BY DATE_TRUNC('minute', created_at)
		ORDER BY time ASC
	`, since).All()
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
	err := g.DB().Ctx(ctx).Raw(`
		SELECT
			COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY latency_ms), 0) as p50,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms), 0) as p95,
			COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_ms), 0) as p99,
			COALESCE(AVG(latency_ms), 0) as avg,
			COALESCE(MAX(latency_ms), 0) as max,
			COALESCE(MIN(latency_ms), 0) as min
		FROM bil_usage_logs
		WHERE created_at >= ? AND latency_ms IS NOT NULL
	`, since).Scan(&lat)
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

	result, err := g.DB().Ctx(ctx).Raw(`
		SELECT
			model_name,
			COUNT(*) as requests,
			COALESCE(SUM(COALESCE(input_tokens,0) + COALESCE(output_tokens,0)), 0) as tokens,
			COALESCE(SUM(total_cost), 0) as total_cost
		FROM bil_usage_logs
		WHERE created_at >= ?
		GROUP BY model_name
		ORDER BY requests DESC
		LIMIT 20
	`, since).All()
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

	result, err := g.DB().Ctx(ctx).Raw(`
		SELECT
			t.id as tenant_id,
			t.name as tenant_name,
			COUNT(*) as requests,
			COALESCE(SUM(ul.total_cost), 0) as total_cost
		FROM bil_usage_logs ul
		JOIN tnt_tenants t ON t.id = ul.tenant_id
		WHERE ul.created_at >= ?
		GROUP BY t.id, t.name
		ORDER BY requests DESC
		LIMIT 10
	`, since).All()
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
	err := g.DB().Ctx(ctx).Raw(`
		SELECT COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_ms), 0) as p99
		FROM bil_usage_logs
		WHERE created_at >= ? AND latency_ms IS NOT NULL
	`, since).Scan(&row)
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
