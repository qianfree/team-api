package dispatchadapter

import (
	"context"
	"math/rand"
	"slices"
	"sync"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"

	"github.com/qianfree/team-api/internal/dao"
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

		// 后台循环（目录刷新 / pub-sub 失效 / 健康上报 / 策略热更新 / 维护）必须用
		// 独立的长生命周期 context：Coordinator 首次由某个请求惰性触发组装，若把该
		// 请求 ctx 传给后台 goroutine，请求一结束 ctx 即被 cancel，之后所有后台 DB/
		// Redis 读取都会 context canceled（例如策略刷新器每 30s 读 channel_routing_policy
		// 失败，导致路由策略热更新形同虚设）。
		bgCtx := gctx.New()

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

		redisState.Start(bgCtx)
		catalog.Start(bgCtx)
		StartPolicyRefresher(bgCtx, wireStop, coordinator)
		startMaintenance(bgCtx, wireStop)
	})
	return coordinator
}

// CatalogInstance 返回目录单例（handler 取 ForwardMeta 用，需先调用过 Coordinator）。
func CatalogInstance() *Catalog { return catalog }

// HealthAlpha 返回当前路由策略的健康指数 α（健康分 = succ_ewma^α × 100）。
//
// 管理后台展示健康分必须与调度 healthFactor、维护快照落盘同源：三处都用 succ^α，
// 唯独模型级能力列表曾用线性 succ×100，而前端两处共用同一套 80/50 阈值，导致
// succ=0.89 时模型行显示 89「健康」、渠道行显示 79「降级」，看起来像 bug。
//
// 调度引擎未组装（首次 relay 请求前）时返回默认策略的 α，不触发组装
// ——管理后台读接口不应有装配整个调度引擎的副作用（与 CatalogInstance 同策略）。
func HealthAlpha() float64 {
	if coordinator == nil {
		return dispatch.DefaultRoutingPolicy().Health.Alpha
	}
	return coordinator.Policy().Health.Alpha
}

// RefreshDispatchLease 长请求（流式/websocket）续期调度租约。
// 供 handler 的租约续期器直接调用（RouteSession 非并发安全，不经会话）。
func RefreshDispatchLease(ctx context.Context, channelID int64, requestID string) {
	if redisState != nil {
		redisState.RefreshLease(ctx, channelID, requestID)
	}
}

// ReportProbeOutcome 渠道测试（管理后台手动测试 / 自动探测 cron）结果喂给调度健康体系。
// model 必须传平台模型名（chn_abilities.model_name），与调度健康键一致，不要传上游名。
// 探测失败只喂熔断窗口不衰减健康分（防周期性探测把无流量渠道健康分拖垮）；
// 探测成功正常回升健康并可关闭 HALF_OPEN 熔断。
func ReportProbeOutcome(ctx context.Context, channelID int64, model string, success bool, latencyMs float64) {
	Coordinator(ctx) // 确保已组装（含健康上报 worker）
	if redisState == nil {
		return
	}
	o := dispatch.Outcome{ChannelID: channelID, Model: model, Success: success, LatencyMs: latencyMs, Probe: true}
	if !success {
		o.Class = dispatch.ErrClassTransient
	}
	redisState.ReportOutcome(o)
}

// CatalogHasModel 判断渠道×模型当前是否在调度目录中（探测可观测性检查用）。
// 不在目录的探测目标（渠道无启用的能力行、或模型未配置能力）健康 EWMA 写入后无人消费：
// 调度不会选它、维护快照也不会落盘该渠道的健康分——管理后台将看不到探测效果。
// 需已调用过 Coordinator 完成组装；未组装时返回 false。
func CatalogHasModel(channelID int64, model string) bool {
	if catalog == nil {
		return false
	}
	return slices.Contains(catalog.ChannelModels()[channelID], model)
}

// MarkChannelRecovered 渠道被手动启用/恢复时复位熔断并开启爬坡窗口（管理后台调用）。
func MarkChannelRecovered(ctx context.Context, channelID int64) {
	if redisState != nil {
		redisState.MarkChannelRecovered(ctx, channelID)
	}
}

// ResetChannelHealth 重置渠道健康度（管理后台"重置健康度"按钮）：熔断复位 + 成功率恢复，
// 渠道立即恢复被调度选择的能力。models 从目录快照取（该渠道服务的模型列表）。
func ResetChannelHealth(ctx context.Context, channelID int64) {
	if redisState == nil {
		return
	}
	var models []string
	if catalog != nil {
		models = catalog.ChannelModels()[channelID]
	}
	redisState.ResetHealth(ctx, channelID, models)
}

// ResetModelHealth 重置指定模型的健康度（只重置该模型，不影响其他模型）
func ResetModelHealth(ctx context.Context, channelID int64, modelName string) {
	if redisState == nil {
		return
	}
	redisState.ResetHealth(ctx, channelID, []string{modelName})
}

// ClearChannelCredentialCooldown 解除渠道下所有 Key 的凭证冷却，返回被解除的 Key 数量。
//
// 更换 Key / 重置健康度后必须调用。凭证冷却按 keyID 打标（dispatch:v1:credcd:<keyID>，
// 401/403 时写入、默认 300s TTL），而管理后台更换 Key 是原地改同一行、keyID 不变：
// 不清标记的话调度器仍认为该 Key 在冷却中，渠道全部 Key 冷却即整体不可用
// （日志表现为「无可用渠道 … 凭证全部冷却×N」），且期间从不发起上游请求，
// 新 Key 一次都得不到验证，只能干等 TTL 自愈。
//
// 不按 status 过滤：被禁用后重新启用的 Key 同样需要解除。
func ClearChannelCredentialCooldown(ctx context.Context, channelID int64) int {
	var keys []struct {
		ID int64 `json:"id"`
	}
	if err := dao.ChnChannelKeys.Ctx(ctx).
		Where("channel_id", channelID).
		Fields("id").
		Scan(&keys); err != nil {
		g.Log().Warningf(ctx, "[Dispatch] 解除凭证冷却时查询渠道 Key 失败: channel=%d err=%v", channelID, err)
		return 0
	}
	for _, k := range keys {
		ClearCredentialCooldownByKey(ctx, k.ID)
	}
	return len(keys)
}

// ClearCredentialCooldownByKey 解除单个 Key 的凭证冷却。
// 凭证被就地更新（换 Key / OAuth 令牌刷新）后必须调用：keyID 不变，
// 旧凭证留下的冷却标记会继续把新凭证挡在调度之外，直到 TTL 自然到期。
func ClearCredentialCooldownByKey(ctx context.Context, keyID int64) {
	if redisState != nil {
		redisState.ClearCredentialCooldown(ctx, keyID) // 含本地镜像
		return
	}
	// 调度引擎尚未组装：没有本地镜像可清，只需删 Redis 标记
	clearCredCooldownKey(ctx, keyID)
}

// InvalidateChannel 渠道禁用/删除时的联动清理：清绑定 + 跨实例目录失效。
// 管理后台渠道写操作（更新/删除/Key 变更/能力变更）后调用。
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
