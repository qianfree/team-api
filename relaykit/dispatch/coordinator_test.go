package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 假端口
// ---------------------------------------------------------------------------

type fakeCatalog struct{ channels []Channel }

func (f *fakeCatalog) Snapshot(_ context.Context, _ int64, _ string, _ []int64) []Channel {
	out := make([]Channel, len(f.channels))
	copy(out, f.channels)
	return out
}

// probeTokenKey fakeState 探测令牌的复合键：model 空串 = 渠道级，非空 = 渠道×模型级。
type probeTokenKey struct {
	ch    int64
	model string
}

type fakeState struct {
	bindings    map[string]int64
	setBinds    int
	touchBinds  int
	invalidated []int64
	outcomes    []Outcome
	probeGrant  map[probeTokenKey]bool // 默认拒绝
	leaseDeny   map[int64]bool
	acquired    map[int64]int
	released    map[int64]int
	refreshed   map[int64]int
	cooled      map[int64]bool
}

func newFakeState() *fakeState {
	return &fakeState{
		bindings:   map[string]int64{},
		probeGrant: map[probeTokenKey]bool{},
		leaseDeny:  map[int64]bool{},
		acquired:   map[int64]int{},
		released:   map[int64]int{},
		refreshed:  map[int64]int{},
		cooled:     map[int64]bool{},
	}
}

func (f *fakeState) GetBinding(_ context.Context, key string) (int64, bool) {
	id, ok := f.bindings[key]
	return id, ok
}
func (f *fakeState) SetBinding(_ context.Context, key string, channelID int64, _ time.Duration) {
	f.bindings[key] = channelID
	f.setBinds++
}
func (f *fakeState) TouchBinding(_ context.Context, _ string, _ time.Duration) { f.touchBinds++ }
func (f *fakeState) InvalidateChannelBindings(_ context.Context, channelID int64) {
	f.invalidated = append(f.invalidated, channelID)
}
func (f *fakeState) ReportOutcome(o Outcome) { f.outcomes = append(f.outcomes, o) }
func (f *fakeState) TryProbeToken(_ context.Context, id int64, model string) bool {
	return f.probeGrant[probeTokenKey{id, model}]
}
func (f *fakeState) AcquireLease(_ context.Context, id int64, _ int, _ string) bool {
	if f.leaseDeny[id] {
		return false
	}
	f.acquired[id]++
	return true
}
func (f *fakeState) RefreshLease(_ context.Context, id int64, _ string) { f.refreshed[id]++ }
func (f *fakeState) ReleaseLease(_ context.Context, id int64, _ string) { f.released[id]++ }
func (f *fakeState) IsCredentialCooled(_ context.Context, keyID int64) bool {
	return f.cooled[keyID]
}
func (f *fakeState) CoolCredential(_ context.Context, keyID int64, _ time.Duration) {
	f.cooled[keyID] = true
}

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func zeroEntropy() float64 { return 0 }

func newTestCoordinator(state *fakeState, channels ...Channel) (*Coordinator, *fakeClock) {
	clock := &fakeClock{t: time.UnixMilli(1_700_000_000_000)}
	co := NewCoordinator(&fakeCatalog{channels: channels}, state, nil, clock, zeroEntropy)
	return co, clock
}

func testProfile() RequestProfile {
	return RequestProfile{
		RequestID: "req-1",
		TenantID:  1, UserID: 2, APIKeyID: 3,
		Model:   "gpt-4o",
		Replay:  ReplayCostly,
		Signals: SessionSignals{HeaderSessionID: "sess-1"},
	}
}

// ---------------------------------------------------------------------------
// 场景测试
// ---------------------------------------------------------------------------

func TestCoordinator_首次选择与绑定命中(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()
	co, _ := newTestCoordinator(state,
		healthyChannel(1, TierPrimary, 10),
		healthyChannel(2, TierPrimary, 10),
	)

	// 首次请求：HRW 选择 + 写绑定
	s1 := co.Route(ctx, testProfile())
	d1 := s1.Next(ctx)
	require.NotNil(t, d1)
	assert.Equal(t, ReasonHRW, d1.Reason)
	assert.Equal(t, 1, state.setBinds)
	assert.Equal(t, d1.Channel.ID, state.bindings[s1.SessionKey().Key])

	s1.Finish(ctx, true, 120)
	assert.Equal(t, 1, state.touchBinds, "成功请求滑动续期绑定")
	require.Len(t, state.outcomes, 1)
	assert.True(t, state.outcomes[0].Success)
	assert.Equal(t, 1, state.released[d1.Channel.ID], "结束释放租约")

	// 第二个请求：绑定守卫命中，绝对稳定
	s2 := co.Route(ctx, testProfile())
	d2 := s2.Next(ctx)
	require.NotNil(t, d2)
	assert.Equal(t, ReasonBind, d2.Reason)
	assert.Equal(t, d1.Channel.ID, d2.Channel.ID)
}

func TestCoordinator_守卫_健康跌破重绑(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()

	sick := healthyChannel(1, TierPrimary, 10)
	sick.SuccEwma = 0.5 // healthFactor 0.25 < keepHealthMin 0.5
	co, _ := newTestCoordinator(state, sick, healthyChannel(2, TierPrimary, 10))

	s := co.Route(ctx, testProfile())
	state.bindings[s.SessionKey().Key] = 1 // 预置绑定到病渠道

	d := s.Next(ctx)
	require.NotNil(t, d)
	assert.NotEqual(t, ReasonBind, d.Reason, "被绑渠道健康跌破守卫下限必须重跑 HRW")
	assert.Equal(t, 1, state.setBinds, "重绑写入新绑定")
}

func TestCoordinator_守卫_饱和渠道重绑(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()

	full := healthyChannel(1, TierPrimary, 10)
	full.SoftLimit = 10
	full.Inflight = 10 // 原始余量 0 < keepHeadroomMin 0.1
	co, _ := newTestCoordinator(state, full, healthyChannel(2, TierPrimary, 10))

	s := co.Route(ctx, testProfile())
	state.bindings[s.SessionKey().Key] = 1

	d := s.Next(ctx)
	require.NotNil(t, d)
	assert.Equal(t, int64(2), d.Channel.ID, "饱和渠道权重为 0，必然重绑到渠道 2")
	assert.NotEqual(t, ReasonBind, d.Reason)
}

// TestCoordinator_守卫_glm缺陷回归 溢出到 secondary 建立的绑定必须保持稳定：
// 守卫判据与 tier/cost 解耦，不得因 tierFactor 低而每请求重绑（规避 keepThreshold 缺陷）。
func TestCoordinator_守卫_glm缺陷回归(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()
	co, _ := newTestCoordinator(state,
		healthyChannel(1, TierPrimary, 10),
		healthyChannel(2, TierSecondary, 10), // tierFactor 0.15，健康与余量均充足
	)

	s := co.Route(ctx, testProfile())
	state.bindings[s.SessionKey().Key] = 2 // 溢出期建立的 secondary 绑定

	for range 20 {
		s = co.Route(ctx, testProfile())
		d := s.Next(ctx)
		require.NotNil(t, d)
		assert.Equal(t, int64(2), d.Channel.ID)
		assert.Equal(t, ReasonBind, d.Reason, "secondary 绑定在健康充足时必须稳定命中")
	}
	assert.Zero(t, state.setBinds, "不得发生任何重绑")
}

func TestCoordinator_502原地重试与预算耗尽(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()
	co, _ := newTestCoordinator(state,
		healthyChannel(1, TierPrimary, 10),
		healthyChannel(2, TierPrimary, 10),
	)

	s := co.Route(ctx, testProfile())
	d1 := s.Next(ctx)
	require.NotNil(t, d1)
	first := d1.Channel.ID

	// 第一次 502 → 原地退避重试，Next 返回同渠道同 Key
	dec, backoff := s.Report(ctx, 502, nil, DeliveryResponseReceived, 50, 0)
	assert.Equal(t, DecisionInPlaceRetry, dec)
	assert.Equal(t, 100*time.Millisecond, backoff)
	d2 := s.Next(ctx)
	require.NotNil(t, d2)
	assert.Equal(t, first, d2.Channel.ID)

	// 第二次 502 → 仍原地，退避翻倍
	dec, backoff = s.Report(ctx, 502, nil, DeliveryResponseReceived, 50, 0)
	assert.Equal(t, DecisionInPlaceRetry, dec)
	assert.Equal(t, 200*time.Millisecond, backoff)
	s.Next(ctx)

	// 第三次 502 → 原地预算耗尽，failover 换渠道；绑定不删除
	dec, _ = s.Report(ctx, 502, nil, DeliveryResponseReceived, 50, 0)
	assert.Equal(t, DecisionFailover, dec)
	d3 := s.Next(ctx)
	require.NotNil(t, d3)
	assert.NotEqual(t, first, d3.Channel.ID)
	assert.Equal(t, 1, state.released[first], "failover 释放原渠道租约")
	assert.Contains(t, state.bindings, s.SessionKey().Key, "失败不删除绑定")

	// 三次失败均计入健康上报
	assert.Len(t, state.outcomes, 3)
	for _, o := range state.outcomes {
		assert.False(t, o.Success)
		assert.Equal(t, ErrClassTransient, o.Class)
	}
}

func TestCoordinator_401凭证轮换链路(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()

	ch1 := healthyChannel(1, TierPrimary, 100) // 高权重确保先选中
	ch1.KeyIDs = []int64{11, 12}
	ch2 := healthyChannel(2, TierPrimary, 0.001)
	ch2.KeyIDs = []int64{21}
	co, _ := newTestCoordinator(state, ch1, ch2)

	s := co.Route(ctx, testProfile())
	d := s.Next(ctx)
	require.NotNil(t, d)
	require.Equal(t, int64(1), d.Channel.ID)
	assert.Equal(t, int64(11), d.KeyID, "按序取第一个 active Key")

	// 第一次 401 → 冷却 Key 11，轮换到 Key 12
	dec, _ := s.Report(ctx, 401, nil, DeliveryResponseReceived, 30, 0)
	assert.Equal(t, DecisionRotateCredential, dec)
	assert.True(t, state.cooled[11], "失效 Key 必须冷却")
	assert.Empty(t, state.outcomes, "凭证错误不计渠道健康")

	d = s.Next(ctx)
	require.NotNil(t, d)
	assert.Equal(t, int64(1), d.Channel.ID, "轮换保持同渠道")
	assert.Equal(t, int64(12), d.KeyID)
	assert.Equal(t, ReasonCredRotate, d.Reason)

	// 第二次 401 → 轮换预算耗尽，failover 到渠道 2
	dec, _ = s.Report(ctx, 401, nil, DeliveryResponseReceived, 30, 0)
	assert.Equal(t, DecisionFailover, dec)
	assert.True(t, state.cooled[12])

	d = s.Next(ctx)
	require.NotNil(t, d)
	assert.Equal(t, int64(2), d.Channel.ID)
	assert.Equal(t, int64(21), d.KeyID)
}

func TestCoordinator_单Key渠道401直接failover(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()

	ch1 := healthyChannel(1, TierPrimary, 100)
	ch1.KeyIDs = []int64{11}
	co, _ := newTestCoordinator(state, ch1, healthyChannel(2, TierPrimary, 0.001))

	s := co.Route(ctx, testProfile())
	d := s.Next(ctx)
	require.Equal(t, int64(1), d.Channel.ID)

	dec, _ := s.Report(ctx, 401, nil, DeliveryResponseReceived, 30, 0)
	assert.Equal(t, DecisionFailover, dec, "唯一 Key 冷却后无可轮换的 Key，凭证错误直接 failover")
	assert.True(t, state.cooled[11])
}

func TestCoordinator_全Key冷却渠道跳过(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()
	state.cooled[11] = true

	ch1 := healthyChannel(1, TierPrimary, 100)
	ch1.KeyIDs = []int64{11}
	ch2 := healthyChannel(2, TierPrimary, 0.001)
	co, _ := newTestCoordinator(state, ch1, ch2)

	s := co.Route(ctx, testProfile())
	state.bindings[s.SessionKey().Key] = 1 // 即使绑定指向它

	d := s.Next(ctx)
	require.NotNil(t, d)
	assert.Equal(t, int64(2), d.Channel.ID, "全 Key 冷却的渠道本请求不可用")
	assert.Equal(t, 1, d.Excluded.Request)
}

func TestCoordinator_租约拒绝不扣预算(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()
	state.leaseDeny[1] = true

	co, _ := newTestCoordinator(state,
		healthyChannel(1, TierPrimary, 100),
		healthyChannel(2, TierPrimary, 0.001),
	)

	s := co.Route(ctx, testProfile())
	state.bindings[s.SessionKey().Key] = 1

	d := s.Next(ctx)
	require.NotNil(t, d)
	assert.Equal(t, int64(2), d.Channel.ID)
	assert.Equal(t, 1, d.Excluded.Lease)
	assert.Zero(t, s.attempt.FailoverUsed, "租约失败不扣 failover 预算")
}

func TestCoordinator_熔断排除与探测放行(t *testing.T) {
	ctx := context.Background()

	open := healthyChannel(1, TierPrimary, 100)
	open.Breaker = BreakerOpen
	half := healthyChannel(2, TierPrimary, 100)
	half.Breaker = BreakerHalfOpen
	normal := healthyChannel(3, TierPrimary, 0.001)

	// 探测令牌被拒：HALF_OPEN 渠道也不可用，落到普通渠道
	state := newFakeState()
	co, _ := newTestCoordinator(state, open, half, normal)
	s := co.Route(ctx, testProfile())
	d := s.Next(ctx)
	require.NotNil(t, d)
	assert.Equal(t, int64(3), d.Channel.ID)
	assert.Equal(t, 2, d.Excluded.Breaker, "OPEN 排除 + 探测令牌拒绝")

	// 探测令牌放行：HALF_OPEN 渠道作为探测请求
	state2 := newFakeState()
	state2.probeGrant[probeTokenKey{2, ""}] = true
	co2, _ := newTestCoordinator(state2, open, half, normal)
	s2 := co2.Route(ctx, testProfile())
	d2 := s2.Next(ctx)
	require.NotNil(t, d2)
	assert.Equal(t, int64(2), d2.Channel.ID)
	assert.Equal(t, ReasonProbe, d2.Reason)
}

func TestCoordinator_模型级熔断排除(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()

	ch1 := healthyChannel(1, TierPrimary, 100)
	ch1.ModelBreaker = BreakerOpen // 渠道级正常，仅该模型熔断
	co, _ := newTestCoordinator(state, ch1, healthyChannel(2, TierPrimary, 0.001))

	s := co.Route(ctx, testProfile())
	d := s.Next(ctx)
	require.NotNil(t, d)
	assert.Equal(t, int64(2), d.Channel.ID)
}

// TestCoordinator_模型级半开探测 模型级熔断冷却期满（HALF_OPEN）走模型级探测令牌：
// 渠道级熔断 CLOSED 时不再被渠道级令牌误拒（修复模型级熔断无法靠流量恢复的死锁）。
func TestCoordinator_模型级半开探测(t *testing.T) {
	ctx := context.Background()

	modelHalf := healthyChannel(1, TierPrimary, 100)
	modelHalf.ModelBreaker = BreakerHalfOpen // 仅模型级半开，渠道级 CLOSED
	normal := healthyChannel(2, TierPrimary, 0.001)

	// 模型级令牌放行 → 作为探测请求选中
	state := newFakeState()
	state.probeGrant[probeTokenKey{1, "gpt-4o"}] = true
	co, _ := newTestCoordinator(state, modelHalf, normal)
	s := co.Route(ctx, testProfile())
	d := s.Next(ctx)
	require.NotNil(t, d)
	assert.Equal(t, int64(1), d.Channel.ID)
	assert.Equal(t, ReasonProbe, d.Reason)

	// 模型级令牌被拒 → 排除落到普通渠道（每探测窗口只放行一个）
	state2 := newFakeState()
	co2, _ := newTestCoordinator(state2, modelHalf, normal)
	s2 := co2.Route(ctx, testProfile())
	d2 := s2.Next(ctx)
	require.NotNil(t, d2)
	assert.Equal(t, int64(2), d2.Channel.ID)
	assert.Equal(t, 1, d2.Excluded.Breaker, "模型级探测拒绝计入 Breaker 排除")
}

// TestCoordinator_双级别半开组合 渠道级与模型级同时 HALF_OPEN：两枚令牌都取，
// 任一被拒即排除；模型级令牌先取（被拒不消耗渠道级令牌）。
func TestCoordinator_双级别半开组合(t *testing.T) {
	ctx := context.Background()

	both := healthyChannel(1, TierPrimary, 100)
	both.Breaker = BreakerHalfOpen
	both.ModelBreaker = BreakerHalfOpen
	normal := healthyChannel(2, TierPrimary, 0.001)

	// 模型级放行 + 渠道级拒绝 → 排除
	state := newFakeState()
	state.probeGrant[probeTokenKey{1, "gpt-4o"}] = true
	co, _ := newTestCoordinator(state, both, normal)
	s := co.Route(ctx, testProfile())
	d := s.Next(ctx)
	require.NotNil(t, d)
	assert.Equal(t, int64(2), d.Channel.ID, "渠道级令牌被拒时整体排除")

	// 双放行 → 探测请求
	state2 := newFakeState()
	state2.probeGrant[probeTokenKey{1, "gpt-4o"}] = true
	state2.probeGrant[probeTokenKey{1, ""}] = true
	co2, _ := newTestCoordinator(state2, both, normal)
	s2 := co2.Route(ctx, testProfile())
	d2 := s2.Next(ctx)
	require.NotNil(t, d2)
	assert.Equal(t, int64(1), d2.Channel.ID)
	assert.Equal(t, ReasonProbe, d2.Reason)
}

func TestCoordinator_tier扩组兜底(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()

	// 运营把 secondary 配成纯冷备（tierFactor=0）
	pol := DefaultRoutingPolicy()
	pol.TierFactors[TierSecondary] = 0

	open := healthyChannel(1, TierPrimary, 10)
	open.Breaker = BreakerOpen
	cold := healthyChannel(2, TierSecondary, 10)

	clock := &fakeClock{t: time.UnixMilli(1_700_000_000_000)}
	co := NewCoordinator(&fakeCatalog{channels: []Channel{open, cold}}, state, pol, clock, zeroEntropy)

	s := co.Route(ctx, testProfile())
	d := s.Next(ctx)
	require.NotNil(t, d, "primary 全灭时纯冷备必须被扩组启用")
	assert.Equal(t, int64(2), d.Channel.ID)
}

func TestCoordinator_溢出reason(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()

	full := healthyChannel(1, TierPrimary, 10)
	full.SoftLimit = 10
	full.Inflight = 10 // 饱和 → 权重坍缩为 0，但仍在候选集
	co, _ := newTestCoordinator(state, full, healthyChannel(2, TierSecondary, 10))

	s := co.Route(ctx, testProfile())
	d := s.Next(ctx)
	require.NotNil(t, d)
	assert.Equal(t, int64(2), d.Channel.ID)
	assert.Equal(t, ReasonOverflow, d.Reason, "primary 在场但饱和 → 溢出")
}

func TestCoordinator_ReplayUnsafe_MaybeSent必Abort(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()
	co, _ := newTestCoordinator(state, healthyChannel(1, TierPrimary, 10))

	p := testProfile()
	p.Replay = ReplayUnsafe
	s := co.Route(ctx, p)
	require.NotNil(t, s.Next(ctx))

	dec, _ := s.Report(ctx, 0, errors.New("unexpected EOF"), DeliveryMaybeSent, 100, 0)
	assert.Equal(t, DecisionAbort, dec, "图片/视频/任务提交在可能已送达时禁止重放")
	assert.Nil(t, s.Next(ctx))
}

func TestCoordinator_504不原地重试(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()
	co, _ := newTestCoordinator(state,
		healthyChannel(1, TierPrimary, 100),
		healthyChannel(2, TierPrimary, 0.001),
	)

	s := co.Route(ctx, testProfile())
	d := s.Next(ctx)
	first := d.Channel.ID

	dec, _ := s.Report(ctx, 504, nil, DeliveryMaybeSent, 30_000, 0)
	assert.Equal(t, DecisionFailover, dec, "504 默认不原地重试")
	d = s.Next(ctx)
	require.NotNil(t, d)
	assert.NotEqual(t, first, d.Channel.ID)
}

func TestCoordinator_429RetryAfter原地等待(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()
	co, _ := newTestCoordinator(state, healthyChannel(1, TierPrimary, 10))

	s := co.Route(ctx, testProfile())
	require.NotNil(t, s.Next(ctx))

	dec, backoff := s.Report(ctx, 429, nil, DeliveryResponseReceived, 20, 1500*time.Millisecond)
	assert.Equal(t, DecisionInPlaceRetry, dec)
	assert.Equal(t, 1500*time.Millisecond, backoff, "遵从 Retry-After（entropy=0 无 jitter）")
}

func TestCoordinator_客户端错误Abort且不上报(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()
	co, _ := newTestCoordinator(state, healthyChannel(1, TierPrimary, 10))

	s := co.Route(ctx, testProfile())
	require.NotNil(t, s.Next(ctx))

	dec, _ := s.Report(ctx, 400, nil, DeliveryResponseReceived, 10, 0)
	assert.Equal(t, DecisionAbort, dec)
	assert.Empty(t, state.outcomes, "客户端错误不计渠道健康")
}

func TestCoordinator_总时限耗尽Abort(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()
	co, clock := newTestCoordinator(state,
		healthyChannel(1, TierPrimary, 10),
		healthyChannel(2, TierPrimary, 10),
	)

	s := co.Route(ctx, testProfile())
	require.NotNil(t, s.Next(ctx))

	clock.advance(31 * time.Second) // 超过 totalDeadline 30s
	dec, _ := s.Report(ctx, 502, nil, DeliveryResponseReceived, 50, 0)
	assert.Equal(t, DecisionAbort, dec)
}

func TestCoordinator_全渠道耗尽返回nil(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()
	co, _ := newTestCoordinator(state,
		healthyChannel(1, TierPrimary, 10),
		healthyChannel(2, TierPrimary, 10),
		healthyChannel(3, TierPrimary, 10),
	)

	s := co.Route(ctx, testProfile())
	seen := map[int64]bool{}
	for {
		d := s.Next(ctx)
		if d == nil {
			break
		}
		assert.False(t, seen[d.Channel.ID], "排除后的渠道不得重复返回")
		seen[d.Channel.ID] = true
		// 渠道致命错误 → 持续 failover
		s.Report(ctx, 402, nil, DeliveryResponseReceived, 10, 0)
	}
	assert.LessOrEqual(t, len(seen), 3, "failover 预算 2 → 最多尝试 3 个渠道")
	s.Finish(ctx, false, 0)
}

func TestCoordinator_jitter叠加(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()
	clock := &fakeClock{t: time.UnixMilli(0)}
	co := NewCoordinator(
		&fakeCatalog{channels: []Channel{healthyChannel(1, TierPrimary, 10)}},
		state, nil, clock,
		func() float64 { return 1.0 }, // 最大熵 → jitter = 50%
	)

	s := co.Route(ctx, testProfile())
	require.NotNil(t, s.Next(ctx))

	_, backoff := s.Report(ctx, 502, nil, DeliveryResponseReceived, 10, 0)
	assert.Equal(t, 150*time.Millisecond, backoff, "100ms + 50% jitter")
}

func TestCoordinator_RefreshLease透传(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()
	co, _ := newTestCoordinator(state, healthyChannel(1, TierPrimary, 10))

	s := co.Route(ctx, testProfile())
	d := s.Next(ctx)
	require.NotNil(t, d)
	s.RefreshLease(ctx)
	assert.Equal(t, 1, state.refreshed[d.Channel.ID])
}

func TestCoordinator_Finish幂等且失败不续绑(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()
	co, _ := newTestCoordinator(state, healthyChannel(1, TierPrimary, 10))

	s := co.Route(ctx, testProfile())
	d := s.Next(ctx)
	require.NotNil(t, d)

	s.Finish(ctx, false, 0)
	s.Finish(ctx, false, 0) // 重复调用无副作用
	assert.Equal(t, 1, state.released[d.Channel.ID])
	assert.Zero(t, state.touchBinds, "失败不续期绑定")
	assert.Empty(t, state.outcomes, "失败结果由 Report 上报，Finish 不重复上报")
	assert.Nil(t, s.Next(ctx), "Finish 后不再返回决策")
}

func TestCoordinator_策略热更新(t *testing.T) {
	co, _ := newTestCoordinator(newFakeState(), healthyChannel(1, TierPrimary, 10))
	p2 := DefaultRoutingPolicy()
	p2.Version = 2
	co.UpdatePolicy(p2)
	assert.Equal(t, 2, co.Policy().Version)
	co.UpdatePolicy(nil)
	assert.Equal(t, 2, co.Policy().Version, "nil 策略不生效")
}

// TestCoordinator_防抖_headroom抖动 绑定守卫防抖：被绑渠道余量在守卫线上方抖动时，
// 绑定必须保持稳定（重绑次数为 0）——纯 HRW 会横跳，守卫是防抖层。
func TestCoordinator_防抖_headroom抖动(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()

	ch1 := healthyChannel(1, TierPrimary, 10)
	ch1.SoftLimit = 100
	ch2 := healthyChannel(2, TierPrimary, 10)
	catalog := &fakeCatalog{channels: []Channel{ch1, ch2}}
	clock := &fakeClock{t: time.UnixMilli(0)}
	co := NewCoordinator(catalog, state, nil, clock, zeroEntropy)

	// 建立绑定
	s := co.Route(ctx, testProfile())
	d := s.Next(ctx)
	require.NotNil(t, d)
	bound := d.Channel.ID
	rebinds := state.setBinds

	// 被绑渠道负载在 20%~85% 间抖动（原始余量始终 ≥ keepHeadroomMin 0.1）
	loads := []int{20, 60, 85, 40, 80, 25, 70}
	for _, inflight := range loads {
		if bound == 1 {
			catalog.channels[0].Inflight = inflight
		} else {
			catalog.channels[1].Inflight = inflight
			catalog.channels[1].SoftLimit = 100
		}
		s = co.Route(ctx, testProfile())
		d = s.Next(ctx)
		require.NotNil(t, d)
		assert.Equal(t, bound, d.Channel.ID, "inflight=%d 时绑定必须保持", inflight)
		assert.Equal(t, ReasonBind, d.Reason)
	}
	assert.Equal(t, rebinds, state.setBinds, "抖动期间重绑次数必须为 0")
}

// TestCoordinator_租户级策略覆盖 profile.Policy 非空时本会话使用租户策略而非全局策略。
func TestCoordinator_租户级策略覆盖(t *testing.T) {
	ctx := context.Background()
	state := newFakeState()
	co, _ := newTestCoordinator(state,
		healthyChannel(1, TierPrimary, 10),
		healthyChannel(2, TierPrimary, 10),
	)

	// 租户覆盖：原地重试预算 0 → 瞬时错误直接 failover（全局默认为 2 → 原地重试）
	tenantPol := DefaultRoutingPolicy()
	tenantPol.Retry.InPlaceBudget = 0

	p := testProfile()
	p.Policy = tenantPol
	s := co.Route(ctx, p)
	d := s.Next(ctx)
	require.NotNil(t, d)

	dec, _ := s.Report(ctx, 502, nil, DeliveryResponseReceived, 50, 0)
	assert.Equal(t, DecisionFailover, dec, "租户策略原地预算 0 应直接 failover")

	// 对照组：无覆盖走全局策略 → 原地重试
	s2 := co.Route(ctx, testProfile())
	require.NotNil(t, s2.Next(ctx))
	dec2, _ := s2.Report(ctx, 502, nil, DeliveryResponseReceived, 50, 0)
	assert.Equal(t, DecisionInPlaceRetry, dec2, "无覆盖应使用全局策略原地重试")
}
