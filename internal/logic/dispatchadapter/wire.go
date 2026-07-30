package dispatchadapter

import (
	"context"
	"math/rand"
	"sync"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relaykit/dispatch"
)

// 全局单例（阶段 2 影子模式 / 阶段 3 正式接线时由 handler 调用 Coordinator()）。
var (
	wireOnce    sync.Once
	coordinator *dispatch.Coordinator
	shadowCo    *dispatch.Coordinator // 影子协调器：共享目录与状态，但租约/探测为 dry-run
	catalog     *Catalog
	redisState  *RedisState
	wireStop    chan struct{}
)

// Coordinator 返回调度协调器单例，首次调用完成组装并启动后台任务：
// 目录刷新循环、pub/sub 失效订阅、健康上报 worker、策略热更新。
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
		// 影子协调器：绑定读写走真实状态（预热粘性数据），租约/探测 dry-run（不产生幻影占用）
		shadowCo = dispatch.NewCoordinator(catalog, &dryRunState{redisState}, pol, dispatch.SystemClock{}, rand.Float64)

		redisState.Start(ctx)
		catalog.Start(ctx)
		StartPolicyRefresher(ctx, wireStop, coordinator, shadowCo)
		startMaintenance(ctx, wireStop)
	})
	return coordinator
}

// ShadowCoordinator 返回影子协调器（阶段 2 对比用，只算不用）。
func ShadowCoordinator(ctx context.Context) *dispatch.Coordinator {
	Coordinator(ctx)
	return shadowCo
}

// dryRunState 影子模式的 StatePort 包装：容量租约与探测令牌不产生副作用——
// 影子决策不真正发请求，占用租约会造成幻影 inflight、消耗探测令牌会挤占真实探测。
// 绑定读写保持真实（预热会话粘性数据，验证 bind 命中率）。
type dryRunState struct {
	*RedisState
}

func (d *dryRunState) AcquireLease(_ context.Context, _ int64, _ int, _ string) bool { return true }
func (d *dryRunState) RefreshLease(_ context.Context, _ int64, _ string)             {}
func (d *dryRunState) ReleaseLease(_ context.Context, _ int64, _ string)             {}
func (d *dryRunState) TryProbeToken(_ context.Context, _ int64) bool                 { return true }

// CatalogInstance 返回目录单例（handler 取 ForwardMeta 用，需先调用过 Coordinator）。
func CatalogInstance() *Catalog { return catalog }

// RefreshDispatchLease 长请求（流式/websocket）续期调度租约。
// 供 handler 的租约续期器直接调用（RouteSession 非并发安全，不经会话）。
func RefreshDispatchLease(ctx context.Context, channelID int64, requestID string) {
	if redisState != nil {
		redisState.RefreshLease(ctx, channelID, requestID)
	}
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
