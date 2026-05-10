package task

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
)

// SnapshotHealthScores 快照所有渠道的当前健康度
func SnapshotHealthScores(ctx context.Context) error {
	type healthRow struct {
		ChannelID           int64   `json:"channel_id"`
		HealthScore         float64 `json:"health_score"`
		SuccessRate         float64 `json:"success_rate"`
		LatencyMs           float64 `json:"latency_ms"`
		StabilityScore      float64 `json:"stability_score"`
		ConsecutiveFailures int     `json:"consecutive_failures"`
	}

	var rows []healthRow
	err := dao.ChnHealthScores.Ctx(ctx).
		Fields("channel_id, health_score, success_rate, latency_ms, stability_score, consecutive_failures").
		Scan(&rows)
	if err != nil {
		g.Log().Errorf(ctx, "[Cron] query health scores failed: %v", err)
		return err
	}

	if len(rows) == 0 {
		g.Log().Debug(ctx, "[Cron] no health scores to snapshot")
		return nil
	}

	now := gtime.Now()
	for _, row := range rows {
		_, err = dao.ChnHealthSnapshots.Ctx(ctx).Insert(do.ChnHealthSnapshots{
			ChannelId:           row.ChannelID,
			HealthScore:         row.HealthScore,
			SuccessRate:         row.SuccessRate,
			LatencyMs:           row.LatencyMs,
			StabilityScore:      row.StabilityScore,
			ConsecutiveFailures: row.ConsecutiveFailures,
			SnapshotAt:          now,
		})
		if err != nil {
			g.Log().Warningf(ctx, "[Cron] insert snapshot for channel %d failed: %v", row.ChannelID, err)
		}
	}

	// 清理过期快照
	retentionDays := common.Config().GetInt(ctx, "health_snapshot_retention_days")
	if retentionDays <= 0 {
		retentionDays = 7
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	_, err = dao.ChnHealthSnapshots.Ctx(ctx).
		Where("snapshot_at < ?", cutoff).
		Delete()
	if err != nil {
		g.Log().Warningf(ctx, "[Cron] cleanup old snapshots failed: %v", err)
	}

	return nil
}
