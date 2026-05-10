package monitor

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	ws "github.com/qianfree/team-api/internal/handler/ws"
)

// StartMonitorPusher 启动监控数据 WebSocket 推送服务。
// realtime 数据每 3 秒推送一次，dashboard 数据每 30 秒推送一次。
// 仅当有 admin WS 连接时才执行数据采集和推送。
func StartMonitorPusher(ctx context.Context) {
	go realtimePusher(ctx)
	go dashboardPusher(ctx)
	g.Log().Info(ctx, "[MonitorPusher] started")
}

func realtimePusher(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hub := ws.GetHub()
			if hub == nil || hub.ClientCount() == 0 {
				continue
			}
			data := GetRealtimeData()
			ws.PublishToAllAdmins(ctx, ws.ChannelMonitor, "realtime", data)
		}
	}
}

func dashboardPusher(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hub := ws.GetHub()
			if hub == nil || hub.ClientCount() == 0 {
				continue
			}
			data, err := GetDashboardData(ctx, 5)
			if err != nil {
				g.Log().Warningf(ctx, "[MonitorPusher] dashboard data error: %v", err)
				continue
			}
			ws.PublishToAllAdmins(ctx, ws.ChannelMonitor, "dashboard", data)
		}
	}
}
