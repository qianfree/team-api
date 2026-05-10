package monitor

import (
	"context"
	"github.com/qianfree/team-api/internal/dao"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// GetDashboardData returns combined monitoring dashboard data.
func GetDashboardData(ctx context.Context, minutes int) (map[string]any, error) {
	if minutes <= 0 {
		minutes = 5
	}

	result := make(map[string]any)

	// System metrics
	if snapshot := GetLatestMetrics(); snapshot != nil {
		result["system"] = map[string]any{
			"cpu":       snapshot.CPU,
			"memory":    snapshot.Memory,
			"disk":      snapshot.Disk,
			"network":   snapshot.Network,
			"runtime":   snapshot.Runtime,
			"timestamp": snapshot.Timestamp,
		}
	}

	// API metrics
	apiMetrics, err := GetAPIMetrics(ctx, minutes)
	if err != nil {
		g.Log().Warningf(ctx, "get api metrics: %v", err)
	} else {
		result["api"] = apiMetrics
	}

	// DB pool
	dbPool, err := GetDBPoolMetrics(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "get db pool metrics: %v", err)
	} else {
		result["db_pool"] = dbPool
	}

	// Redis pool
	redisPool, err := GetRedisPoolMetrics(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "get redis pool metrics: %v", err)
	} else {
		result["redis_pool"] = redisPool
	}

	// Alert stats
	alertStats, err := GetAlertStats(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "get alert stats: %v", err)
	} else {
		result["alerts"] = alertStats
	}

	return result, nil
}

// GetSystemMetricsHistory returns system metrics for the given duration.
func GetSystemMetricsHistory(ctx context.Context, minutes int) ([]SystemMetricsSnapshot, error) {
	if minutes <= 0 {
		minutes = 60
	}
	return GetMetricsHistory(time.Duration(minutes) * time.Minute), nil
}

// GetSystemMetricsFromDB returns system metrics from the database for a longer time range.
func GetSystemMetricsFromDB(ctx context.Context, minutes int) ([]map[string]any, error) {
	if minutes <= 0 {
		minutes = 60
	}

	since := time.Now().Add(-time.Duration(minutes) * time.Minute)
	result, err := dao.OpsSystemMetrics.Ctx(ctx).
		Where("collected_at >= ?", since).
		OrderAsc("collected_at").
		Fields("metric_type, metric_data, collected_at").
		All()
	if err != nil {
		return nil, err
	}

	records := result.List()

	return records, nil
}
