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
		g.Log().Errorf(ctx, "[ChannelProbe] 自动探测查询活跃渠道失败: %v", err)
		return
	}
	if len(channels) == 0 {
		g.Log().Infof(ctx, "[ChannelProbe] 自动探测开始: 无待探测渠道（活跃且配置了 test_model 的渠道为 0）")
		return
	}
	g.Log().Infof(ctx, "[ChannelProbe] 自动探测开始: 待探测活跃渠道 %d 个", len(channels))

	successCount := 0
	failCount := 0

	for _, ch := range channels {
		testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		result, err := admin.New().TestChannel(testCtx, &v1.ChannelTestReq{ID: ch.ID})
		cancel()

		if err != nil {
			failCount++
			// 探测请求尚未发出（无可用 Key/渠道不存在等），TestChannel 内部无结果日志，此处记录
			g.Log().Warningf(ctx, "[ChannelProbe] 渠道 %s (%d) 探测执行出错: %v", ch.Name, ch.ID, err)
			continue
		}

		if result.Success {
			successCount++
		} else {
			failCount++
		}
		// 成功/失败明细日志由 TestChannel 统一打印（含模型/延迟/错误），此处只做轮次汇总
	}

	g.Log().Infof(ctx, "[ChannelProbe] 自动探测完成: 共 %d 个渠道 | 成功 %d | 失败 %d",
		len(channels), successCount, failCount)
}
