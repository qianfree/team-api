package dispatchadapter

import (
	"context"
	"math/rand"
	"sync"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relaykit/dispatch"
)

// 全局单例（handler / worker / 任务提交端点经 Coordinator() 使用）。
var (
	wireOnce    sync.Once
	coordinator *dispatch.Coordinator
	catalog     *Catalog
	redisState  *RedisState
	wireStop    chan struct{}
)

// Coordinator 返回调度协调器单例，首次调用完成组装并启动后台任务：
// 目录刷新循环、pub/sub 失效订阅、健康上报 worker、策略热更新、维护任务。
func Coordinator(ctx context.Context) *dispatch.Coordinator {
	wireOnce.Do(func() {
		wireStop = make(chan struct{})

		pol, err := LoadRoutingPolicy(ctx)
		if err != nil {
			g.Log().Warningf(ctx, "[Dispatch] 启动时路由策略非法，使用默认策略: %v", err)
			pol = dispatch.DefaultRoutingPolicy()
		}

		// 组装依赖环：coordinator 持策略指针 → state/catalog 通过闭包读同一策略
		policyFn := func() *dispatch.RoutingPolicy { return coordinator.Policy() }

		redisState = NewRedisState(policyFn, func(channelID int64) (bool, int) {
			return catalog.StrictLookup(channelID)
		})
		catalog = NewCatalog(policyFn, nil, func(ctx context.Context, channelID int64, model string) RuntimeReadout {
			return redisState.ReadRuntime(ctx, channelID, model)
		})

		coordinator = dispatch.NewCoordinator(catalog, redisState, pol, dispatch.SystemClock{}, rand.Float64)

		redisState.Start(ctx)
		catalog.Start(ctx)
		StartPolicyRefresher(ctx, wireStop, coordinator)
		startMaintenance(ctx, wireStop)
	})
	return coordinator
}

// CatalogInstance 返回目录单例（handler 取 ForwardMeta 用，需先调用过 Coordinator）。
func CatalogInstance() *Catalog { return catalog }

// RefreshDispatchLease 长请求（流式/websocket）续期调度租约。
// 供 handler 的租约续期器直接调用（RouteSession 非并发安全，不经会话）。
func RefreshDispatchLease(ctx context.Context, channelID int64, requestID string) {
	if redisState != nil {
		redisState.RefreshLease(ctx, channelID, requestID)
	}
}

// ReportProbeOutcome 渠道测试（管理后台手动测试 / 自动探测 cron）结果喂给调度健康体系。
// 探测失败按 TRANSIENT 轻罚（0.93 档），成功正常回升并可关闭 HALF_OPEN 熔断。
func ReportProbeOutcome(ctx context.Context, channelID int64, model string, success bool, latencyMs float64) {
	Coordinator(ctx) // 确保已组装（含健康上报 worker）
	if redisState == nil {
		return
	}
	o := dispatch.Outcome{ChannelID: channelID, Model: model, Success: success, LatencyMs: latencyMs}
	if !success {
		o.Class = dispatch.ErrClassTransient
	}
	redisState.ReportOutcome(o)
}

// MarkChannelRecovered 渠道被手动启用/恢复时复位熔断并开启爬坡窗口（管理后台调用）。
func MarkChannelRecovered(ctx context.Context, channelID int64) {
	if redisState != nil {
		redisState.MarkChannelRecovered(ctx, channelID)
	}
}

// InvalidateChannel 渠道禁用/删除时的联动清理：清绑定 + 跨实例目录失效。
// 管理后台渠道写操作后调用（阶段 3 接线）。
func InvalidateChannel(ctx context.Context, channelID int64) {
	if redisState != nil {
		redisState.InvalidateChannelBindings(ctx, channelID)
	}
	PublishInvalidate(ctx)
	if catalog != nil {
		catalog.Invalidate()
	}
}

// Shutdown 停止全部后台任务（进程退出时调用）。
func Shutdown() {
	if wireStop != nil {
		close(wireStop)
	}
	if catalog != nil {
		catalog.Stop()
	}
	if redisState != nil {
		redisState.Stop()
	}
}
