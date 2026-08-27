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
		return
	}

	successCount := 0
	failCount := 0

	for _, ch := range channels {
		if probeOneChannel(ctx, ch.ID, ch.Name) {
			successCount++
		} else {
			failCount++
		}
		// 失败明细由 TestChannel 以 Warning 打印（含模型/延迟/错误），此处仅 Debug 级轮次汇总
	}

	g.Log().Debugf(ctx, "[ChannelProbe] 自动探测完成: 共 %d 个渠道 | 成功 %d | 失败 %d",
		len(channels), successCount, failCount)
}

// probeOneChannel 探测单个渠道，返回是否成功。
// 独立超时 + panic 兜底：单个渠道无论超时、报错还是 panic，都不影响本轮后续渠道
// （panic 若逃逸到 cron 层虽有 runHandlerSafely 兜底进程，但本轮剩余渠道会被整体跳过）。
func probeOneChannel(ctx context.Context, channelID int64, channelName string) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
			g.Log().Errorf(ctx, "[ChannelProbe] 渠道 %s (%d) 探测 panic，跳过该渠道继续下一个: %v",
				channelName, channelID, r)
		}
	}()

	testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := admin.New().TestChannel(testCtx, &v1.ChannelTestReq{ID: channelID})
	if err != nil {
		// 探测请求尚未发出（无可用 Key/渠道不存在等），TestChannel 内部无结果日志，此处记录
		g.Log().Warningf(ctx, "[ChannelProbe] 渠道 %s (%d) 探测执行出错: %v", channelName, channelID, err)
		return false
	}
	return result.Success
}
