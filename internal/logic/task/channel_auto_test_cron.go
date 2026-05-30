package task

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/admin"
	do "github.com/qianfree/team-api/internal/model/do"
)

// AutoTestChannels 自动测试所有活跃渠道，并尝试恢复自动禁用的渠道
func AutoTestChannels(ctx context.Context) {
	type channelInfo struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		Type      int    `json:"type"`
		BaseURL   string `json:"base_url"`
		TestModel string `json:"test_model"`
	}

	// 1. 测试活跃渠道
	var channels []channelInfo
	err := dao.ChnChannels.Ctx(ctx).
		Where("status", "active").
		Where("test_model != ?", "").
		Fields("id, name, type, base_url, test_model").
		Scan(&channels)
	if err != nil {
		g.Log().Errorf(ctx, "[Cron] query active channels failed: %v", err)
		return
	}

	successCount := 0
	failCount := 0

	for _, ch := range channels {
		testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		result, err := admin.New().TestChannel(testCtx, &v1.ChannelTestReq{ID: ch.ID})
		cancel()

		if err != nil {
			failCount++
			g.Log().Warningf(ctx, "[Cron] channel %s (%d) test error: %v", ch.Name, ch.ID, err)
			continue
		}

		if result.Success {
			successCount++
		} else {
			failCount++
			if result.Error != "" {
				g.Log().Warningf(ctx, "[Cron] channel %s (%d) test failed: %s", ch.Name, ch.ID, result.Error)
			}
		}
	}

	// 2. 尝试恢复自动禁用的渠道
	testAndRecoverDisabledChannels(ctx)
}

// testAndRecoverDisabledChannels 测试自动禁用的渠道，测试通过则恢复
func testAndRecoverDisabledChannels(ctx context.Context) int {
	var disabledChannels []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	err := dao.ChnChannels.Ctx(ctx).
		Where("status", "disabled").
		Where("auto_disabled", 1).
		Where("test_model != ?", "").
		Fields("id, name").
		Scan(&disabledChannels)
	if err != nil || len(disabledChannels) == 0 {
		return 0
	}

	recovered := 0
	for _, ch := range disabledChannels {
		testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		result, err := admin.New().TestChannel(testCtx, &v1.ChannelTestReq{ID: ch.ID})
		cancel()

		if err != nil || !result.Success {
			g.Log().Debugf(ctx, "[Cron] auto-disabled channel %s (%d) recovery test failed", ch.Name, ch.ID)
			continue
		}

		// 恢复渠道
		_, err = dao.ChnChannels.Ctx(ctx).
			Where("id", ch.ID).
			Data(do.ChnChannels{
				Status:       "active",
				AutoDisabled: 0,
			}).Update()
		if err != nil {
			g.Log().Errorf(ctx, "[Cron] recover channel %s (%d) failed: %v", ch.Name, ch.ID, err)
			continue
		}

		// 重置连续失败计数
		dao.ChnHealthScores.Ctx(ctx).
			Where("channel_id", ch.ID).
			Data(do.ChnHealthScores{ConsecutiveFailures: 0}).
			Update()

		recovered++
		g.Log().Infof(ctx, "[Cron] auto-disabled channel %s (%d) recovered", ch.Name, ch.ID)
	}

	return recovered
}

// CleanupExpiredAffinities 清理过期的亲和性记录
func CleanupExpiredAffinities(ctx context.Context) {
	g.Log().Debug(ctx, "[Cron] cleaning up expired affinities")

	_, err := dao.ChnChannelAffinities.Ctx(ctx).
		Where("expires_at < ?", time.Now()).
		Delete()
	if err != nil {
		g.Log().Errorf(ctx, "[Cron] cleanup expired affinities failed: %v", err)
		return
	}

	g.Log().Debug(ctx, "[Cron] expired affinities cleaned up")
}
