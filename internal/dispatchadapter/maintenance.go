package dispatchadapter

import (
	"context"
	"math"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
)

// 维护任务（阶段 3）：
// 1. 健康快照落盘：Redis EWMA → chn_health_scores 每 5min 一次，仅供管理后台
//    仪表盘/审计展示，调度决策不读此表（替代旧体系的实时写入）。
// 2. 自动禁用：渠道级熔断本轮故障期（first_opened_ms）持续超过
//    policy.breaker.autoDisableAfterSeconds → 落库禁用 + 清绑定 + 目录失效。
//    受现有开关 channel_auto_disable_enabled 控制（默认关）。

const maintenanceInterval = 5 * time.Minute

// startMaintenance 启动维护循环（wire 组装时调用一次）。
func startMaintenance(ctx context.Context, stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(maintenanceInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runMaintenance(ctx)
			case <-stop:
				return
			}
		}
	}()
}

// runMaintenance 执行一轮健康快照 + 自动禁用检查。
func runMaintenance(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			g.Log().Warningf(ctx, "[Dispatch] 维护任务 panic: %v", r)
		}
	}()
	if catalog == nil || redisState == nil {
		return
	}

	pol := coordinator.Policy()
	autoDisable := lcommon.Config().GetBool(ctx, "channel_auto_disable_enabled")
	now := time.Now().UnixMilli()

	for channelID, models := range catalog.ChannelModels() {
		if len(models) == 0 {
			continue
		}

		// 按渠道聚合各模型的健康读值
		var succSum, latSum float64
		var channelRt RuntimeReadout
		for i, model := range models {
			rt := redisState.ReadRuntime(ctx, channelID, model)
			succSum += rt.SuccEwma
			latSum += rt.LatEwmaMs
			if i == 0 {
				channelRt = rt // 渠道级熔断状态各模型读值相同，取第一个
			}
		}
		avgSucc := succSum / float64(len(models))
		avgLat := latSum / float64(len(models))

		snapshotHealthScore(ctx, channelID, avgSucc, avgLat, pol.Health.Alpha)

		// 自动禁用：本轮故障期持续超过阈值（first_opened_ms 只在故障期存在，
		// 探测失败不重置——探测成功即被清除，见 luaBreakerFail/luaBreakerSuccess）
		if autoDisable && channelRt.FirstOpenedMs > 0 &&
			now-channelRt.FirstOpenedMs >= int64(pol.Breaker.AutoDisableAfterSeconds)*1000 {
			autoDisableChannel(ctx, channelID, now-channelRt.FirstOpenedMs)
		}
	}
}

// snapshotHealthScore 健康快照落盘（upsert）。
// health_score 用 succ^alpha 的百分制近似，与调度用的 healthFactor 同源。
func snapshotHealthScore(ctx context.Context, channelID int64, succ, latMs, alpha float64) {
	healthScore := math.Pow(clampFloat(succ, 0, 1), alpha) * 100

	affected, err := dao.ChnHealthScores.Ctx(ctx).
		Where("channel_id", channelID).
		Data(do.ChnHealthScores{
			SuccessRate:  succ * 100,
			LatencyMs:    latMs,
			HealthScore:  healthScore,
			CalculatedAt: gtime.Now(),
		}).
		UpdateAndGetAffected()
	if err != nil {
		g.Log().Debugf(ctx, "[Dispatch] 健康快照更新失败: channel=%d err=%v", channelID, err)
		return
	}
	if affected == 0 {
		_, err = dao.ChnHealthScores.Ctx(ctx).Data(do.ChnHealthScores{
			ChannelId:    channelID,
			SuccessRate:  succ * 100,
			LatencyMs:    latMs,
			HealthScore:  healthScore,
			CalculatedAt: gtime.Now(),
		}).Insert()
		if err != nil {
			g.Log().Debugf(ctx, "[Dispatch] 健康快照插入失败: channel=%d err=%v", channelID, err)
		}
	}
}

// autoDisableChannel 熔断长期不恢复 → 落库禁用（标记 auto_disabled 供自动恢复探测识别）
// + 清绑定 + 跨实例目录失效 + 全租户通知（沿用 channel_auto_disabled 通知模板）。
func autoDisableChannel(ctx context.Context, channelID int64, openDurationMs int64) {
	affected, err := dao.ChnChannels.Ctx(ctx).
		Where("id", channelID).
		Where("status", "active").
		Data(do.ChnChannels{Status: "disabled", AutoDisabled: 1}).
		UpdateAndGetAffected()
	if err != nil {
		g.Log().Warningf(ctx, "[Dispatch] 自动禁用渠道失败: channel=%d err=%v", channelID, err)
		return
	}
	if affected == 0 {
		return // 已被禁用或删除
	}
	g.Log().Warningf(ctx, "[Dispatch] 渠道 %d 熔断持续 %ds 未恢复，已自动禁用（channel_auto_disable_enabled）",
		channelID, openDurationMs/1000)
	InvalidateChannel(ctx, channelID)

	// 全租户通知（异步，失败仅记日志）
	var chName *string
	_ = dao.ChnChannels.Ctx(ctx).Where("id", channelID).Fields("name").Scan(&chName)
	channelName := ""
	if chName != nil {
		channelName = *chName
	}
	go func() {
		bgCtx := context.Background()
		engine := lcommon.NewNotificationEngine()
		if err := engine.SendToAllTenants(bgCtx, "channel_auto_disabled", g.Map{
			"channel_name": channelName,
			"threshold":    0, // 新语义：按熔断持续时长而非连续失败次数
		}, ""); err != nil {
			g.Log().Errorf(bgCtx, "[Dispatch] 自动禁用通知发送失败: %v", err)
		}
	}()
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
