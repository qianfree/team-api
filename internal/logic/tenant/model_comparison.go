package tenant

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
)

func (s *sTenant) ModelComparison(ctx context.Context, req *v1.ModelComparisonReq) (*v1.ModelComparisonRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	if req.Days <= 0 {
		req.Days = 7
	}

	models := strings.Split(req.Models, ",")
	for i, m := range models {
		models[i] = strings.TrimSpace(m)
	}
	if len(models) < 2 || len(models) > 4 {
		return nil, common.NewBadRequestError("请选择 2-4 个模型进行对比")
	}

	since := gtime.Now().AddDate(0, 0, -req.Days).Format("Y-m-d")

	// Build args for parameterized query
	args := []any{tenantID, since}
	for _, m := range models {
		args = append(args, m)
	}

	// Aggregate per-model stats (parameterized IN clause, no fmt.Sprintf)
	inClause := strings.Repeat("?,", len(models)-1) + "?"
	aggQuery := `
			SELECT
				model_name,
				COUNT(*) as requests,
				COUNT(*) FILTER (WHERE status = 'success') as success_count,
				COALESCE(AVG(latency_ms) FILTER (WHERE status = 'success'), 0) as avg_latency,
				COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms) FILTER (WHERE status = 'success'), 0) as p95_latency,
				COALESCE(SUM(total_cost), 0) as total_cost,
				COALESCE(SUM(input_tokens), 0) as input_tokens,
				COALESCE(SUM(output_tokens), 0) as output_tokens
			FROM bil_usage_logs
			WHERE tenant_id = ? AND created_at >= ? AND model_name IN (` + inClause + `)
			GROUP BY model_name
		`

	records, err := g.DB().Raw(aggQuery, args...).All()
	if err != nil {
		return nil, err
	}

	items := make([]v1.ModelComparisonItem, 0, len(models))
	var totalRequests int64
	var totalCost float64

	// Build a map from query results
	type aggRow struct {
		ModelName    string
		Requests     int64
		SuccessCount int64
		AvgLatency   float64
		P95Latency   float64
		TotalCost    float64
		InputTokens  int64
		OutputTokens int64
	}
	rowMap := make(map[string]aggRow)
	for _, r := range records {
		row := aggRow{
			ModelName:    r["model_name"].String(),
			Requests:     r["requests"].Int64(),
			SuccessCount: r["success_count"].Int64(),
			AvgLatency:   r["avg_latency"].Float64(),
			P95Latency:   r["p95_latency"].Float64(),
			TotalCost:    r["total_cost"].Float64(),
			InputTokens:  r["input_tokens"].Int64(),
			OutputTokens: r["output_tokens"].Int64(),
		}
		rowMap[row.ModelName] = row
	}

	// Build items for all requested models (even if no data)
	for _, modelName := range models {
		row, hasData := rowMap[modelName]
		if !hasData {
			items = append(items, v1.ModelComparisonItem{
				ModelName: modelName,
			})
			continue
		}

		successRate := float64(0)
		if row.Requests > 0 {
			successRate = float64(row.SuccessCount) / float64(row.Requests) * 100
		}
		avgCost := float64(0)
		if row.Requests > 0 {
			avgCost = row.TotalCost / float64(row.Requests)
		}

		items = append(items, v1.ModelComparisonItem{
			ModelName:         modelName,
			Requests:          row.Requests,
			SuccessRate:       math.Round(successRate*100) / 100,
			AvgLatencyMs:      math.Round(row.AvgLatency*100) / 100,
			P95LatencyMs:      math.Round(row.P95Latency*100) / 100,
			TotalCost:         math.Round(row.TotalCost*1000000) / 1000000,
			AvgCostPerRequest: math.Round(avgCost*1000000) / 1000000,
			InputTokens:       row.InputTokens,
			OutputTokens:      row.OutputTokens,
		})
		totalRequests += row.Requests
		totalCost += row.TotalCost
	}

	// Score and recommend
	scoreItems(items)
	recommended, reason := recommendBest(items)

	for i := range items {
		items[i].IsRecommended = items[i].ModelName == recommended
	}

	// Daily trends
	trends := fetchTrends(ctx, tenantID, since, models, args)

	return &v1.ModelComparisonRes{
		Summary: v1.ModelComparisonSummary{
			TotalRequests: totalRequests,
			TotalCost:     math.Round(totalCost*1000000) / 1000000,
			Recommended:   recommended,
			Reason:        reason,
		},
		Items:  items,
		Trends: trends,
	}, nil
}

func scoreItems(items []v1.ModelComparisonItem) {
	if len(items) == 0 {
		return
	}

	// Normalize each dimension to 0-100
	var minCost, maxCost, minLatency, maxLatency float64
	first := true
	for _, it := range items {
		if it.Requests == 0 {
			continue
		}
		if first {
			minCost, maxCost = it.AvgCostPerRequest, it.AvgCostPerRequest
			minLatency, maxLatency = it.AvgLatencyMs, it.AvgLatencyMs
			first = false
			continue
		}
		minCost = math.Min(minCost, it.AvgCostPerRequest)
		maxCost = math.Max(maxCost, it.AvgCostPerRequest)
		minLatency = math.Min(minLatency, it.AvgLatencyMs)
		maxLatency = math.Max(maxLatency, it.AvgLatencyMs)
	}

	costRange := maxCost - minCost
	latencyRange := maxLatency - minLatency

	for i := range items {
		if items[i].Requests == 0 {
			items[i].Score = 0
			continue
		}

		costScore := 100.0
		if costRange > 0 {
			costScore = (1 - (items[i].AvgCostPerRequest-minCost)/costRange) * 100
		}
		latencyScore := 100.0
		if latencyRange > 0 {
			latencyScore = (1 - (items[i].AvgLatencyMs-minLatency)/latencyRange) * 100
		}
		successScore := items[i].SuccessRate // already 0-100

		items[i].Score = math.Round((costScore*0.4+latencyScore*0.3+successScore*0.3)*100) / 100
	}
}

func recommendBest(items []v1.ModelComparisonItem) (string, string) {
	best := ""
	bestScore := -1.0
	for _, it := range items {
		if it.Score > bestScore && it.Requests > 0 {
			bestScore = it.Score
			best = it.ModelName
		}
	}
	if best == "" {
		return "", "数据不足，无法推荐"
	}
	return best, fmt.Sprintf("综合评分 %.1f（费用40%% + 延迟30%% + 成功率30%%）", bestScore)
}

func fetchTrends(ctx context.Context, tenantID int64, since string, models []string, baseArgs []any) []v1.ModelTrendDay {
	inClause := strings.Repeat("?,", len(models)-1) + "?"
	trendQuery := `
			SELECT
				DATE(created_at) as day,
				model_name,
				COUNT(*) as requests,
				COALESCE(SUM(total_cost), 0) as cost,
				COALESCE(AVG(latency_ms) FILTER (WHERE status = 'success'), 0) as latency
			FROM bil_usage_logs
			WHERE tenant_id = ? AND created_at >= ? AND model_name IN (` + inClause + `)
			GROUP BY DATE(created_at), model_name
			ORDER BY day
		`

	records, err := g.DB().Raw(trendQuery, baseArgs...).All()
	if err != nil {
		return nil
	}

	// Group by day
	dayMap := make(map[string][]v1.ModelTrendDayItem)
	for _, r := range records {
		day := r["day"].String()
		dayMap[day] = append(dayMap[day], v1.ModelTrendDayItem{
			ModelName: r["model_name"].String(),
			Requests:  r["requests"].Int64(),
			Cost:      math.Round(r["cost"].Float64()*1000000) / 1000000,
			LatencyMs: math.Round(r["latency"].Float64()*100) / 100,
		})
	}

	// Sort days for deterministic output
	days := make([]string, 0, len(dayMap))
	for day := range dayMap {
		days = append(days, day)
	}
	sort.Strings(days)

	trends := make([]v1.ModelTrendDay, 0, len(days))
	for _, day := range days {
		trends = append(trends, v1.ModelTrendDay{
			Date:    day,
			Details: dayMap[day],
		})
	}
	return trends
}
