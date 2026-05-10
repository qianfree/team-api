package monitor

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// DBPoolMetrics holds database connection pool statistics.
type DBPoolMetrics struct {
	ActiveConnections int `json:"active_connections"`
	IdleConnections   int `json:"idle_connections"`
	TotalConnections  int `json:"total_connections"`
	MaxConnections    int `json:"max_connections"`
	WaitingQueries    int `json:"waiting_queries"`
}

// GetDBPoolMetrics returns the current database connection pool statistics.
func GetDBPoolMetrics(ctx context.Context) (*DBPoolMetrics, error) {
	m := &DBPoolMetrics{}

	// PostgreSQL pg_stat_activity for connection counts
	type pgStat struct {
		Active int `json:"active"`
		Idle   int `json:"idle"`
		Total  int `json:"total"`
		Wait   int `json:"wait"`
	}
	var pg pgStat
	err := g.DB().Ctx(ctx).Raw(`
		SELECT
			COALESCE(SUM(CASE WHEN state = 'active' THEN 1 ELSE 0 END), 0) as active,
			COALESCE(SUM(CASE WHEN state = 'idle' THEN 1 ELSE 0 END), 0) as idle,
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN wait_event_type IS NOT NULL THEN 1 ELSE 0 END), 0) as wait
		FROM pg_stat_activity
		WHERE datname = current_database()
	`).Scan(&pg)
	if err != nil {
		g.Log().Warningf(ctx, "get pg_stat_activity: %v", err)
	} else {
		m.ActiveConnections = pg.Active
		m.IdleConnections = pg.Idle
		m.TotalConnections = pg.Total
		m.WaitingQueries = pg.Wait
	}

	// Max connections from pg_settings
	type maxConn struct {
		Setting int `json:"setting"`
	}
	var mc maxConn
	err = g.DB().Ctx(ctx).Raw(`SELECT setting::int FROM pg_settings WHERE name = 'max_connections'`).Scan(&mc)
	if err == nil {
		m.MaxConnections = mc.Setting
	} else {
		g.Log().Warningf(ctx, "get max_connections: %v", err)
	}

	return m, nil
}

// GetDBActiveConnections returns the number of active database connections.
func GetDBActiveConnections(ctx context.Context) (float64, error) {
	type countRow struct {
		Count int `json:"count"`
	}
	var row countRow
	err := g.DB().Ctx(ctx).Raw(`
		SELECT COUNT(*) as count
		FROM pg_stat_activity
		WHERE datname = current_database() AND state = 'active'
	`).Scan(&row)
	if err != nil {
		return 0, gerror.Wrapf(err, "get active connections")
	}
	return float64(row.Count), nil
}
