package relay

import (
	"context"
	"math"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/relay/scheduler"
)

// UpdateHealthScoreDirect 同步更新健康度（供测试使用）
func UpdateHealthScoreDirect(ctx context.Context, channelID int64, success bool, latencyMs float64) {
	UpdateHealthScore(ctx, channelID, success, latencyMs)
}

// UpdateHealthScore 计算并更新渠道健康度
// 公式: 健康度 = 成功率×0.40 + 延迟分×0.25 + 稳定性×0.20 + 连续失败分×0.15
func UpdateHealthScore(ctx context.Context, channelID int64, success bool, latencyMs float64) {
	var row struct {
		SuccessRate         float64 `json:"success_rate"`
		LatencyMs           float64 `json:"latency_ms"`
		StabilityScore      float64 `json:"stability_score"`
		ConsecutiveFailures int     `json:"consecutive_failures"`
	}

	err := dao.ChnHealthScores.Ctx(ctx).
		Where("channel_id", channelID).
		Fields("success_rate, latency_ms, stability_score, consecutive_failures").
		Scan(&row)
	if err != nil {
		return
	}

	if success {
		// 更新成功率（指数移动平均）
		newSuccessRate := row.SuccessRate*0.9 + 100*0.1

		// 更新延迟（指数移动平均）
		newLatency := row.LatencyMs*0.7 + latencyMs*0.3

		// 更新稳定性（基于延迟波动）
		newStability := calcStability(newLatency, row.LatencyMs, row.StabilityScore)

		// 重置连续失败
		consecutiveFailures := 0

		// 计算综合健康度
		healthScore := calcHealthScore(newSuccessRate, newLatency, newStability, consecutiveFailures)

		dao.ChnHealthScores.Ctx(ctx).
			Where("channel_id", channelID).
			Data(do.ChnHealthScores{
				SuccessRate:         newSuccessRate,
				LatencyMs:           newLatency,
				StabilityScore:      newStability,
				ConsecutiveFailures: consecutiveFailures,
				HealthScore:         healthScore,
				CalculatedAt:        gtime.Now(),
			}).Update()

		// 检查是否需要自动恢复（连续失败已清零）
		checkAutoRecover(ctx, channelID)

	} else {
		// 失败：降低成功率
		newSuccessRate := row.SuccessRate * 0.9
		if newSuccessRate < 0 {
			newSuccessRate = 0
		}

		// 递增连续失败
		consecutiveFailures := row.ConsecutiveFailures + 1

		// 计算综合健康度
		healthScore := calcHealthScore(newSuccessRate, row.LatencyMs, row.StabilityScore, consecutiveFailures)

		dao.ChnHealthScores.Ctx(ctx).
			Where("channel_id", channelID).
			Data(do.ChnHealthScores{
				SuccessRate:         newSuccessRate,
				ConsecutiveFailures: consecutiveFailures,
				HealthScore:         healthScore,
				CalculatedAt:        gtime.Now(),
			}).Update()

		// 检查是否需要自动禁用
		checkAutoDisable(ctx, channelID, consecutiveFailures)
	}
}

// calcHealthScore 计算综合健康度（0-100）
func calcHealthScore(successRate, latencyMs, stabilityScore float64, consecutiveFailures int) float64 {
	// 1. 成功率分（0-100）权重 0.40
	successScore := successRate

	// 2. 延迟分（0-100）权重 0.25
	// <1s=100, 1-3s=80, 3-10s=50, >10s=20
	latencyScore := calcLatencyScore(latencyMs)

	// 3. 稳定性分（已有）权重 0.20
	stabScore := stabilityScore

	// 4. 连续失败分（0-100）权重 0.15
	// 0次=100, 1次=80, 2次=60, 3次=40, 4次=20, >=5次=0
	failScore := float64(100 - consecutiveFailures*20)
	if failScore < 0 {
		failScore = 0
	}

	health := successScore*0.40 + latencyScore*0.25 + stabScore*0.20 + failScore*0.15

	// 限制在 0-100 范围内
	if health > 100 {
		health = 100
	}
	if health < 0 {
		health = 0
	}

	// 保留两位小数
	return math.Round(health*100) / 100
}

// calcLatencyScore 根据延迟计算分数（0-100）
func calcLatencyScore(latencyMs float64) float64 {
	if latencyMs <= 1000 {
		return 100
	}
	if latencyMs <= 3000 {
		return 80 - (latencyMs-1000)/2000*30 // 80-50
	}
	if latencyMs <= 10000 {
		return 50 - (latencyMs-3000)/7000*30 // 50-20
	}
	return 20
}

// calcStability 计算稳定性分数（基于延迟变化）
func calcStability(newLatency, oldLatency, currentStability float64) float64 {
	if oldLatency <= 0 {
		return 100
	}

	// 延迟波动比例
	ratio := newLatency / oldLatency
	if ratio > 1 {
		ratio = 1 / ratio // 反转，使 >1 的增长变为 <1 的值
	}

	// 波动惩罚
	stability := currentStability*0.9 + ratio*100*0.1
	if stability > 100 {
		stability = 100
	}
	if stability < 0 {
		stability = 0
	}

	return math.Round(stability*100) / 100
}

// checkAutoDisable 检查渠道是否需要自动禁用
func checkAutoDisable(ctx context.Context, channelID int64, consecutiveFailures int) {
	enabled := common.Config().GetString(ctx, "channel_auto_disable_enabled")
	if enabled != "true" {
		return
	}

	threshold := common.Config().GetInt(ctx, "channel_auto_disable_threshold")
	if threshold <= 0 {
		threshold = 5
	}

	if consecutiveFailures < threshold {
		return
	}

	// CAS 更新：仅当状态为 active 时才禁用
	result, err := dao.ChnChannels.Ctx(ctx).
		Where("id", channelID).
		Where("status", "active").
		Data(g.Map{
			"status":        "disabled",
			"auto_disabled": 1,
		}).Update()
	if err != nil {
		g.Log().Errorf(ctx, "[AutoDisable] update channel %d failed: %v", channelID, err)
		return
	}

	if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
		g.Log().Warningf(ctx, "[AutoDisable] channel %d auto-disabled after %d consecutive failures", channelID, consecutiveFailures)

		// 清除该渠道的所有亲和性记录，避免后续请求继续路由到已禁用渠道
		scheduler.GetGlobalAffinity().DeleteByChannel(channelID)

		// 查询渠道名称用于通知
		var chName string
		dao.ChnChannels.Ctx(ctx).Where("id", channelID).Fields("name").Scan(&chName)

		go func() {
			bgCtx := context.Background()
			engine := common.NewNotificationEngine()
			if err := engine.SendToAllTenants(bgCtx, "channel_auto_disabled", g.Map{
				"channel_name": chName,
				"threshold":    threshold,
			}, ""); err != nil {
				g.Log().Errorf(bgCtx, "[AutoDisable] send notification failed: %v", err)
			}
		}()
	}
}

// checkAutoRecover 检查自动禁用的渠道是否应该恢复
func checkAutoRecover(ctx context.Context, channelID int64) {
	// 查询渠道是否为自动禁用状态
	type chRow struct {
		Status       string `json:"status"`
		AutoDisabled int    `json:"auto_disabled"`
	}
	var row chRow
	err := dao.ChnChannels.Ctx(ctx).
		Where("id", channelID).
		Fields("status, auto_disabled").
		Scan(&row)
	if err != nil || row.Status != "disabled" || row.AutoDisabled != 1 {
		return
	}

	// 成功请求 → 自动恢复
	_, err = dao.ChnChannels.Ctx(ctx).
		Where("id", channelID).
		Data(g.Map{
			"status":        "active",
			"auto_disabled": 0,
		}).Update()
	if err != nil {
		g.Log().Errorf(ctx, "[AutoRecover] re-enable channel %d failed: %v", channelID, err)
		return
	}

	g.Log().Infof(ctx, "[AutoRecover] channel %d auto-recovered", channelID)

	// 重置连续失败计数
	dao.ChnHealthScores.Ctx(ctx).
		Where("channel_id", channelID).
		Data(do.ChnHealthScores{ConsecutiveFailures: 0}).
		Update()
}
