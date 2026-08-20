package task

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/admin"
)

// AutoTestChannels 自动测试所有活跃渠道（禁用渠道不自动探测，
// 由管理员手动测试确认恢复后再启用，避免未经确认的流量进入）
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
}
