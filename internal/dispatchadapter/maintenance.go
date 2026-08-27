package dispatchadapter

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"
)

// 维护任务（阶段 3）：
//  1. 健康快照落盘：Redis EWMA → chn_health_scores 每 5min 一次，另可由
//     RequestHealthSnapshot 按渠道即时触发（渠道测试 / 重置健康度后），仅供管理后台
//     仪表盘/审计展示，调度决策不读此表（替代旧体系的实时写入）。
//     两种情况保留库中旧值而不落盘：Redis 读失败（读值不可信）、渠道全部模型无真实
//     上报（默认满分不是结论）——都不能拿乐观默认值覆盖历史分数。
//  2. 自动禁用：渠道级熔断本轮故障期（first_opened_ms）持续超过
//     policy.breaker.autoDisableAfterSeconds → 落库禁用 + 清绑定 + 目录失效。
//     受现有开关 channel_auto_disable_enabled 控制（默认关）。
//     Redis 读值降级时同样跳过——自动禁用不可逆，不能基于不可信读值触发。

const maintenanceInterval = 5 * time.Minute

// healthRefreshDelay 按需重算前的等待：健康上报是 fire-and-forget（异步 worker 消费），
// 渠道测试刚投递的探测结果可能尚未写入 Redis，立刻重算会读到上报前的旧值。
const healthRefreshDelay = 500 * time.Millisecond

// healthRefresh 单渠道健康快照的按需重算信号。缓冲队列满则丢弃——下一轮定时维护兜底。
var healthRefresh = make(chan int64, 256)

// RequestHealthSnapshot 请求立即重算某渠道的健康快照，供渠道测试 / 重置健康度后调用。
//
// 维护循环是纯 ticker（5 分钟一轮），此前管理员测试渠道成功后 Redis 立刻变、模型级能力
// 列表（直读 Redis）立刻变，唯独渠道健康分要等下一轮才动，看起来像"测试没生效"。
// 不阻塞调用方；调度引擎尚未组装时信号留在队列里，组装后被消费。
func RequestHealthSnapshot(channelID int64) {
	select {
	case healthRefresh <- channelID:
	default:
	}
}

// startMaintenance 启动维护循环（wire 组装时调用一次）。
func startMaintenance(ctx context.Context, stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(maintenanceInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runMaintenance(ctx)
			case channelID := <-healthRefresh:
				time.Sleep(healthRefreshDelay)
				refreshChannelHealth(ctx, channelID)
			case <-stop:
				return
			}
		}
	}()
}

// refreshChannelHealth 按需重算单个渠道的健康快照。
// 与 runMaintenance 共用聚合与落盘逻辑，跳过条件也一致（降级 / 全无数据保留旧值）。
func refreshChannelHealth(ctx context.Context, channelID int64) {
	defer func() {
		if r := recover(); r != nil {
			g.Log().Warningf(ctx, "[Dispatch] 健康快照按需重算 panic: %v", r)
		}
	}()
	if catalog == nil || redisState == nil {
		return
	}
	models := catalog.ChannelModels()[channelID]
	if len(models) == 0 {
		// 渠道不在调度目录（禁用 / 无启用能力行）：无健康数据可算，
		// TestChannel 侧已有 CatalogHasModel 告警提示该情形
		return
	}
	agg := aggregateChannelHealth(models, func(model string) RuntimeReadout {
		return redisState.ReadRuntime(ctx, channelID, model)
	})
	if agg.Degraded || agg.Counted == 0 {
		return
	}
	snapshotHealthScore(ctx, channelID, agg.AvgSucc, agg.AvgLat, HealthAlpha(), agg.Detail)
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

	channelModels := catalog.ChannelModels()
	covered := make([]int64, 0, len(channelModels))
	skippedDegraded := make([]int64, 0)
	skippedNoData := make([]int64, 0)
	for channelID, models := range channelModels {
		if len(models) == 0 {
			continue
		}

		agg := aggregateChannelHealth(models, func(model string) RuntimeReadout {
			return redisState.ReadRuntime(ctx, channelID, model)
		})

		// Redis 读失败：本轮读值不可信，跳过落盘与自动禁用判定。
		// 若照常落盘，乐观默认值会把全渠道健康分刷成 100，事故期间的历史趋势反而全绿。
		if agg.Degraded {
			skippedDegraded = append(skippedDegraded, channelID)
			continue
		}

		covered = append(covered, channelID)

		// 全部模型都没有真实上报（新渠道 / 长期无流量 / EWMA key 已过 TTL）：
		// 保留库中上一次的分数，不要用默认满分覆盖掉历史结论。
		if agg.Counted == 0 {
			skippedNoData = append(skippedNoData, channelID)
		} else {
			snapshotHealthScore(ctx, channelID, agg.AvgSucc, agg.AvgLat, pol.Health.Alpha, agg.Detail)
		}

		// 自动禁用：本轮故障期持续超过阈值（first_opened_ms 只在故障期存在，
		// 探测失败不重置——探测成功即被清除，见 luaBreakerFail/luaBreakerSuccess）
		if autoDisable && agg.ChannelRt.FirstOpenedMs > 0 &&
			now-agg.ChannelRt.FirstOpenedMs >= int64(pol.Breaker.AutoDisableAfterSeconds)*1000 {
			autoDisableChannel(ctx, channelID, now-agg.ChannelRt.FirstOpenedMs)
		}
	}

	// 覆盖摘要：目录只装载 active × 有启用能力行的渠道，未覆盖渠道的健康分不会落盘
	// （探测却正常的渠道长期停在历史分数时，先看这里是否被跳过）
	g.Log().Infof(ctx, "[ChannelHealth] 健康快照维护轮次完成: 覆盖渠道 %d 个 %v", len(covered), covered)
	if len(skippedDegraded) > 0 {
		g.Log().Warningf(ctx, "[ChannelHealth] 本轮 %d 个渠道因 Redis 健康读取失败跳过落盘（保留库中旧值）: %v",
			len(skippedDegraded), skippedDegraded)
	}
	if len(skippedNoData) > 0 {
		g.Log().Infof(ctx, "[ChannelHealth] 本轮 %d 个渠道全部模型无真实上报，保留库中旧值: %v",
			len(skippedNoData), skippedNoData)
	}
}

// channelHealthAggregate 单个渠道的健康聚合结果。
type channelHealthAggregate struct {
	AvgSucc   float64        // 有真实上报模型的成功率 EWMA 均值
	AvgLat    float64        // 同上，延迟均值
	Counted   int            // 参与均值的模型数；0 = 该渠道全部模型无真实上报
	Degraded  bool           // 任一模型读失败 → 本轮读值整体不可信
	Detail    string         // 各模型明细，供排查"是哪个模型拉低了均值"
	ChannelRt RuntimeReadout // 首个模型的读值（渠道级熔断状态各模型相同）
}

// aggregateChannelHealth 聚合渠道下各模型的健康读值。
//
// 只统计确有真实上报的模型（HasHealth）：无流量模型的读值是乐观默认满分，计入均值
// 会把真正故障的模型稀释掉——20 个模型的渠道死掉 1 个（succ→0.05）仍能算出 95 分绿灯，
// 而这恰恰是模型级熔断要隔离的典型场景。
//
// 任一模型读失败即整体标记 Degraded：Redis 不可用时读到的满分既非实测也非"确实无数据"，
// 拿它去覆盖库中的历史分数会销毁事故现场。
func aggregateChannelHealth(models []string, read func(model string) RuntimeReadout) channelHealthAggregate {
	var agg channelHealthAggregate
	var succSum, latSum float64
	parts := make([]string, 0, len(models))

	for i, model := range models {
		rt := read(model)
		if i == 0 {
			agg.ChannelRt = rt
		}
		if rt.Degraded {
			// 中断前已累加的部分一并作废：降级的聚合结果只保留 Degraded 与
			// ChannelRt，不留下会被误读为"有效均值"的半成品计数。
			return channelHealthAggregate{Degraded: true, ChannelRt: agg.ChannelRt}
		}
		if !rt.HasHealth {
			parts = append(parts, model+"=无数据")
			continue
		}
		succSum += rt.SuccEwma
		latSum += rt.LatEwmaMs
		agg.Counted++
		parts = append(parts, fmt.Sprintf("%s=%.3f", model, rt.SuccEwma))
	}

	agg.Detail = strings.Join(parts, " ")
	if agg.Counted > 0 {
		agg.AvgSucc = succSum / float64(agg.Counted)
		agg.AvgLat = latSum / float64(agg.Counted)
	}
	return agg
}

// snapshotHealthScore 健康快照落盘（upsert）。
// health_score 用 succ^alpha 的百分制近似，与调度用的 healthFactor 同源。
// 每轮固定输出一行明细（计算值 vs 库中值 + 各模型 succ）：健康分不动时可直接看出
// 是「算出来就是这个值」（某模型 succ 被拖低）还是「写入失败」（库中值持续落后于计算值）。
func snapshotHealthScore(ctx context.Context, channelID int64, succ, latMs, alpha float64, modelDetail string) {
	healthScore := math.Pow(clampFloat(succ, 0, 1), alpha) * 100

	// 先读旧值：updated_at 每轮都会变（affected 恒为 1），比对必须基于 health_score
	var prev *entity.ChnHealthScores
	if err := dao.ChnHealthScores.Ctx(ctx).Where("channel_id", channelID).Scan(&prev); err != nil {
		g.Log().Debugf(ctx, "[Dispatch] 健康快照旧值读取失败: channel=%d err=%v", channelID, err)
	}
	prevScore := -1.0 // 无记录
	if prev != nil {
		prevScore = prev.HealthScore
	}

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
		// 写入失败必须可见（此前 Debug 级静默，健康分冻结难以定位）
		g.Log().Warningf(ctx, "[ChannelHealth] 渠道 %d 健康快照更新失败: %v", channelID, err)
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
			g.Log().Warningf(ctx, "[ChannelHealth] 渠道 %d 健康快照插入失败: %v", channelID, err)
			return
		}
	}

	// 分数有实际变化才记 Info；稳态渠道降为 Debug（避免每 5 分钟 × 渠道数的固定噪音）。
	// 两种情况都带各模型 succ 明细：排查"健康分不符合预期"时可直接定位是哪个模型拉低了均值。
	if math.Abs(healthScore-prevScore) >= 0.05 {
		g.Log().Infof(ctx, "[ChannelHealth] 渠道 %d 健康分落盘: %.1f → %.1f | avg_succ %.4f | 模型: %s",
			channelID, prevScore, healthScore, succ, modelDetail)
	} else {
		g.Log().Debugf(ctx, "[ChannelHealth] 渠道 %d 健康快照无变化: %.1f | avg_succ %.4f | 模型: %s",
			channelID, healthScore, succ, modelDetail)
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
