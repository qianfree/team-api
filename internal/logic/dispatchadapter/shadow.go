package dispatchadapter

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/relaykit/dispatch"
)

// 阶段 2 影子模式（开发计划 §5 阶段 2）：
// 现有调度返回后并行调用新 Coordinator 计算一次初始决策，写新旧对比日志；
// 真实请求结果同时喂给新健康体系（dispatch:v1:* 键，与旧体系完全隔离）做数据预热。
// 只算不用，不影响实际路由。回滚 = 关闭开关或删除本文件与调用点。

const shadowFlagKey = "channel_dispatch_shadow_enabled"

// shadowFlagCache 开关本地缓存：避免每请求查 sys_options（未落库的 key 会穿透到 DB）。
var shadowFlagCache struct {
	mu        sync.Mutex
	val       bool
	checkedAt time.Time
}

// ShadowEnabled 影子模式是否开启（15s 本地缓存）。
func ShadowEnabled(ctx context.Context) bool {
	shadowFlagCache.mu.Lock()
	defer shadowFlagCache.mu.Unlock()
	if time.Since(shadowFlagCache.checkedAt) > 15*time.Second {
		shadowFlagCache.val = lcommon.Config().GetBool(ctx, shadowFlagKey)
		shadowFlagCache.checkedAt = time.Now()
	}
	return shadowFlagCache.val
}

// ---------------------------------------------------------------------------
// ctx 信号传递（handler 提取会话信号 → provider 内的影子对比点）
// ---------------------------------------------------------------------------

type shadowCtxKey struct{}

// ShadowContext handler 在进入重试循环前注入的影子上下文。
type ShadowContext struct {
	RequestID string
	Signals   dispatch.SessionSignals
	Replay    dispatch.Replayability
}

// WithShadowContext 把影子上下文注入 ctx。
func WithShadowContext(ctx context.Context, sc ShadowContext) context.Context {
	return context.WithValue(ctx, shadowCtxKey{}, sc)
}

func shadowFromCtx(ctx context.Context) ShadowContext {
	if sc, ok := ctx.Value(shadowCtxKey{}).(ShadowContext); ok {
		return sc
	}
	return ShadowContext{Replay: dispatch.ReplayCostly}
}

// ---------------------------------------------------------------------------
// 影子对比
// ---------------------------------------------------------------------------

// ShadowLegacy 现有调度器的选择结果（对比基准）。
type ShadowLegacy struct {
	ChannelID   int64   `json:"channel"`
	ChannelName string  `json:"channel_name"`
	Reason      string  `json:"reason"`
	Priority    int     `json:"priority"`
	Weight      int     `json:"weight"`
	HealthScore float64 `json:"health"`
}

// shadowLogEntry 对比日志结构（单行 JSON，前缀 [DispatchShadow]，统计脚本据此聚合）。
type shadowLogEntry struct {
	Event     string       `json:"event"`
	RequestID string       `json:"request_id,omitempty"`
	TenantID  int64        `json:"tenant_id"`
	Model     string       `json:"model"`
	Match     bool         `json:"match"`
	Legacy    ShadowLegacy `json:"legacy"`
	Shadow    shadowSide   `json:"shadow"`
	CostUs    int64        `json:"cost_us"` // 影子决策耗时（微秒），验收要求 p99 < 1ms
}

type shadowSide struct {
	ChannelID     int64                    `json:"channel"`
	ChannelName   string                   `json:"channel_name,omitempty"`
	KeyID         int64                    `json:"key_id,omitempty"`
	Reason        string                   `json:"reason"`
	Tier          string                   `json:"tier,omitempty"`
	SessionSource string                   `json:"session_source"`
	Breakdown     dispatch.WeightBreakdown `json:"breakdown"`
	Candidates    int                      `json:"candidates"`
	Excluded      dispatch.ExclusionStats  `json:"excluded"`
}

// ShadowCompare 并行计算新调度器的初始决策并写对比日志。
// 仅在初始选择（无排除渠道）时调用；异步执行，不阻塞请求热路径。
func ShadowCompare(ctx context.Context, tenantID, userID, apiKeyID int64, model string, scope []int64, legacy ShadowLegacy) {
	if !ShadowEnabled(ctx) {
		return
	}
	co := ShadowCoordinator(ctx)
	if co == nil {
		return
	}
	sc := shadowFromCtx(ctx)
	bg := context.WithoutCancel(ctx)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				g.Log().Warningf(bg, "[DispatchShadow] 影子决策 panic: %v", r)
			}
		}()

		profile := dispatch.RequestProfile{
			RequestID: sc.RequestID,
			TenantID:  tenantID,
			UserID:    userID,
			APIKeyID:  apiKeyID,
			Model:     model,
			Scope:     scope,
			Replay:    sc.Replay,
			Signals:   sc.Signals,
		}

		start := time.Now()
		sess := co.Route(bg, profile)
		d := sess.Next(bg)
		costUs := time.Since(start).Microseconds()

		entry := shadowLogEntry{
			Event:     "dispatch_shadow",
			RequestID: sc.RequestID,
			TenantID:  tenantID,
			Model:     model,
			Legacy:    legacy,
			CostUs:    costUs,
		}
		if d == nil {
			entry.Shadow = shadowSide{Reason: "no_candidate", SessionSource: string(sess.SessionKey().Source)}
		} else {
			entry.Match = d.Channel.ID == legacy.ChannelID
			entry.Shadow = shadowSide{
				ChannelID:     d.Channel.ID,
				ChannelName:   d.Channel.Name,
				KeyID:         d.KeyID,
				Reason:        string(d.Reason),
				Tier:          string(d.Channel.Tier),
				SessionSource: string(d.SessionKey.Source),
				Breakdown:     d.Breakdown,
				Candidates:    d.CandidateCount,
				Excluded:      d.Excluded,
			}
		}
		payload, err := json.Marshal(entry)
		if err != nil {
			return
		}
		g.Log().Infof(bg, "[DispatchShadow] %s", payload)
	}()
}

// ---------------------------------------------------------------------------
// 结果预热（真实请求结果 → 新健康体系）
// ---------------------------------------------------------------------------

// ShadowObserve 把现有调度路径的真实请求结果喂给新健康/熔断体系（dispatch:v1:* 键），
// 让影子决策的 healthFactor / breaker 有真实数据，同时为切换预热状态。
// 遵守修订 R6：CLIENT / CREDENTIAL 类不计渠道健康（凭证冷却在影子期不生效——KeyID 未知）。
func ShadowObserve(ctx context.Context, channelID int64, model string, success bool, latencyMs float64, statusCode int, err error) {
	if !ShadowEnabled(ctx) {
		return
	}
	_ = Coordinator(ctx) // 确保已组装（含健康上报 worker）
	if redisState == nil {
		return
	}

	class := dispatch.ErrClassNone
	if !success {
		class = dispatch.Classify(statusCode, err, dispatch.DeliveryResponseReceived)
		switch class {
		case dispatch.ErrClassClient, dispatch.ErrClassCredential, dispatch.ErrClassNone:
			return
		}
	}
	redisState.ReportOutcome(dispatch.Outcome{
		ChannelID: channelID,
		Model:     model,
		Success:   success,
		Class:     class,
		LatencyMs: latencyMs,
	})
}
