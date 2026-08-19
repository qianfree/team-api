package dispatch

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func retryPol() RetryPolicy { return DefaultRoutingPolicy().Retry }

func TestDecide_硬规则优先(t *testing.T) {
	p := retryPol()

	// 已向客户端写出响应：任何类别都终止
	for _, cls := range []ErrorClass{ErrClassTransient, ErrClassRateLimit, ErrClassCredential, ErrClassChannelFatal, ErrClassModelFatal, ErrClassTimeout} {
		d, _ := Decide(cls, DeliveryResponseStarted, ReplaySafe, 0, AttemptState{}, p)
		assert.Equal(t, DecisionAbort, d, "ResponseStarted 必须 Abort: %s", cls)

		d, _ = Decide(cls, DeliveryResponseReceived, ReplaySafe, 0, AttemptState{ResponseStarted: true}, p)
		assert.Equal(t, DecisionAbort, d, "state.ResponseStarted 必须 Abort: %s", cls)
	}

	// ReplayUnsafe + MaybeSent 无条件 Abort（状态码/类别无关）
	for _, cls := range []ErrorClass{ErrClassTransient, ErrClassRateLimit, ErrClassCredential, ErrClassChannelFatal, ErrClassModelFatal, ErrClassTimeout} {
		d, _ := Decide(cls, DeliveryMaybeSent, ReplayUnsafe, 0, AttemptState{HasAlternateKey: true}, p)
		assert.Equal(t, DecisionAbort, d, "ReplayUnsafe+MaybeSent 必须 Abort: %s", cls)
	}

	// 总时限耗尽
	d, _ := Decide(ErrClassTransient, DeliveryResponseReceived, ReplaySafe, 0, AttemptState{ElapsedMs: 30_000}, p)
	assert.Equal(t, DecisionAbort, d)
}

func TestDecide_客户端错误不重试(t *testing.T) {
	p := retryPol()
	d, _ := Decide(ErrClassClient, DeliveryResponseReceived, ReplaySafe, 0, AttemptState{}, p)
	assert.Equal(t, DecisionAbort, d)
	d, _ = Decide(ErrClassNone, DeliveryResponseReceived, ReplaySafe, 0, AttemptState{}, p)
	assert.Equal(t, DecisionAbort, d)
}

func TestDecide_瞬时错误原地重试与退避(t *testing.T) {
	p := retryPol() // inPlaceBudget=2, base=100ms, max=1000ms

	d, backoff := Decide(ErrClassTransient, DeliveryResponseReceived, ReplayCostly, 0, AttemptState{InPlaceUsed: 0}, p)
	assert.Equal(t, DecisionInPlaceRetry, d)
	assert.Equal(t, int64(100), backoff)

	d, backoff = Decide(ErrClassTransient, DeliveryResponseReceived, ReplayCostly, 0, AttemptState{InPlaceUsed: 1}, p)
	assert.Equal(t, DecisionInPlaceRetry, d)
	assert.Equal(t, int64(200), backoff, "指数退避 100→200")

	// 原地预算耗尽 → failover
	d, _ = Decide(ErrClassTransient, DeliveryResponseReceived, ReplayCostly, 0, AttemptState{InPlaceUsed: 2}, p)
	assert.Equal(t, DecisionFailover, d)

	// failover 预算也耗尽 → Abort
	d, _ = Decide(ErrClassTransient, DeliveryResponseReceived, ReplayCostly, 0, AttemptState{InPlaceUsed: 2, FailoverUsed: 2}, p)
	assert.Equal(t, DecisionAbort, d)
}

func TestDecide_MaybeSent禁止原地(t *testing.T) {
	p := retryPol()
	// TRANSIENT + MaybeSent：可能已送达，原地重发有重复风险 → 直接 failover
	d, _ := Decide(ErrClassTransient, DeliveryMaybeSent, ReplayCostly, 0, AttemptState{}, p)
	assert.Equal(t, DecisionFailover, d)
}

func TestDecide_超时不原地重试(t *testing.T) {
	p := retryPol()
	// TIMEOUT（含 504）不原地重试
	d, _ := Decide(ErrClassTimeout, DeliveryResponseReceived, ReplayCostly, 0, AttemptState{}, p)
	assert.Equal(t, DecisionFailover, d)

	d, _ = Decide(ErrClassTimeout, DeliveryResponseReceived, ReplayCostly, 0, AttemptState{FailoverUsed: 2}, p)
	assert.Equal(t, DecisionAbort, d)

	// ReplayUnsafe 的超时（未送达除外）直接 Abort：默认 unsafe failover 预算为 0
	d, _ = Decide(ErrClassTimeout, DeliveryResponseReceived, ReplayUnsafe, 0, AttemptState{}, p)
	assert.Equal(t, DecisionAbort, d)
}

func TestDecide_限流RetryAfter(t *testing.T) {
	p := retryPol() // rateLimitWaitMax=2000ms

	// Retry-After 短 → 原地等待，退避 = Retry-After
	d, backoff := Decide(ErrClassRateLimit, DeliveryResponseReceived, ReplayCostly, 1500, AttemptState{}, p)
	assert.Equal(t, DecisionInPlaceRetry, d)
	assert.Equal(t, int64(1500), backoff)

	// Retry-After 超限 → failover
	d, _ = Decide(ErrClassRateLimit, DeliveryResponseReceived, ReplayCostly, 5000, AttemptState{}, p)
	assert.Equal(t, DecisionFailover, d)

	// 无 Retry-After → failover
	d, _ = Decide(ErrClassRateLimit, DeliveryResponseReceived, ReplayCostly, 0, AttemptState{}, p)
	assert.Equal(t, DecisionFailover, d)
}

func TestDecide_凭证轮换(t *testing.T) {
	p := retryPol() // credRotateBudget=1

	// 有备用 Key 且预算未耗尽 → 轮换
	d, backoff := Decide(ErrClassCredential, DeliveryResponseReceived, ReplayCostly, 0, AttemptState{HasAlternateKey: true}, p)
	assert.Equal(t, DecisionRotateCredential, d)
	assert.Zero(t, backoff)

	// 无备用 Key → 升级 failover（等价渠道级致命）
	d, _ = Decide(ErrClassCredential, DeliveryResponseReceived, ReplayCostly, 0, AttemptState{HasAlternateKey: false}, p)
	assert.Equal(t, DecisionFailover, d)

	// 轮换预算耗尽 → failover
	d, _ = Decide(ErrClassCredential, DeliveryResponseReceived, ReplayCostly, 0, AttemptState{HasAlternateKey: true, CredRotationsUsed: 1}, p)
	assert.Equal(t, DecisionFailover, d)

	// failover 预算也耗尽 → Abort
	d, _ = Decide(ErrClassCredential, DeliveryResponseReceived, ReplayCostly, 0, AttemptState{CredRotationsUsed: 1, FailoverUsed: 2}, p)
	assert.Equal(t, DecisionAbort, d)
}

func TestDecide_渠道致命零原地(t *testing.T) {
	p := retryPol()
	d, _ := Decide(ErrClassChannelFatal, DeliveryResponseReceived, ReplayCostly, 0, AttemptState{}, p)
	assert.Equal(t, DecisionFailover, d)
}

func TestDecide_模型致命直接failover(t *testing.T) {
	p := retryPol()
	// 模型致命与渠道致命同语义：零原地重试、立即 failover
	d, _ := Decide(ErrClassModelFatal, DeliveryResponseReceived, ReplayCostly, 0, AttemptState{}, p)
	assert.Equal(t, DecisionFailover, d)

	// failover 预算耗尽 → Abort
	d, _ = Decide(ErrClassModelFatal, DeliveryResponseReceived, ReplayCostly, 0, AttemptState{FailoverUsed: 2}, p)
	assert.Equal(t, DecisionAbort, d)
}

func TestDecide_ReplayUnsafe预算收紧(t *testing.T) {
	p := retryPol() // unsafe 预算默认 0/0

	// 已收到错误响应（非 NotSent）→ Abort
	d, _ := Decide(ErrClassTransient, DeliveryResponseReceived, ReplayUnsafe, 0, AttemptState{}, p)
	assert.Equal(t, DecisionAbort, d)

	// 明确未送达但预算为 0 → 仍 Abort
	d, _ = Decide(ErrClassTransient, DeliveryNotSent, ReplayUnsafe, 0, AttemptState{}, p)
	assert.Equal(t, DecisionAbort, d)

	// 放宽 unsafe failover 预算后，仅 NotSent 允许 failover
	p.UnsafeFailoverBudget = 1
	d, _ = Decide(ErrClassTransient, DeliveryNotSent, ReplayUnsafe, 0, AttemptState{}, p)
	assert.Equal(t, DecisionFailover, d)
	d, _ = Decide(ErrClassTransient, DeliveryResponseReceived, ReplayUnsafe, 0, AttemptState{}, p)
	assert.Equal(t, DecisionAbort, d, "非 NotSent 即使有预算也不允许 failover")
}

// TestDecide_属性_任意错误序列不突破预算 属性测试：模拟协调器的预算记账，
// 用随机错误序列驱动 FSM，断言任何序列都不会突破预算且必然终止。
func TestDecide_属性_任意错误序列不突破预算(t *testing.T) {
	p := retryPol()
	classes := []ErrorClass{ErrClassClient, ErrClassTransient, ErrClassRateLimit, ErrClassCredential, ErrClassChannelFatal, ErrClassModelFatal, ErrClassTimeout}
	deliveries := []DeliveryState{DeliveryNotSent, DeliveryMaybeSent, DeliveryResponseReceived}
	replays := []Replayability{ReplaySafe, ReplayCostly, ReplayUnsafe}

	rng := rand.New(rand.NewSource(42))
	for run := range 5000 {
		replay := replays[rng.Intn(len(replays))]
		s := AttemptState{}
		steps := 0
		for {
			steps++
			require.Less(t, steps, 50, "run %d: FSM 必须在有限步内终止", run)

			cls := classes[rng.Intn(len(classes))]
			delivery := deliveries[rng.Intn(len(deliveries))]
			s.HasAlternateKey = rng.Intn(2) == 0
			retryAfter := int64(rng.Intn(4000))

			d, _ := Decide(cls, delivery, replay, retryAfter, s, p)
			switch d {
			case DecisionAbort:
				// 终止
			case DecisionInPlaceRetry:
				s.InPlaceUsed++
				require.LessOrEqual(t, s.InPlaceUsed, p.InPlaceBudget, "原地预算不得突破")
				continue
			case DecisionRotateCredential:
				s.CredRotationsUsed++
				require.LessOrEqual(t, s.CredRotationsUsed, p.CredRotateBudget, "凭证轮换预算不得突破")
				continue
			case DecisionFailover:
				s.FailoverUsed++
				require.LessOrEqual(t, s.FailoverUsed, p.FailoverBudget, "failover 预算不得突破")
				// 换渠道：渠道级计数清零（与协调器行为一致）
				s.InPlaceUsed = 0
				s.CredRotationsUsed = 0
				continue
			}
			break
		}
	}
}

func TestBackoffMs_封顶(t *testing.T) {
	p := retryPol() // base=100, max=1000
	assert.Equal(t, int64(100), backoffMs(0, p))
	assert.Equal(t, int64(200), backoffMs(1, p))
	assert.Equal(t, int64(400), backoffMs(2, p))
	assert.Equal(t, int64(800), backoffMs(3, p))
	assert.Equal(t, int64(1000), backoffMs(4, p), "封顶")
	assert.Equal(t, int64(1000), backoffMs(60, p), "大次数不溢出")
}

func TestRoutingPolicy_默认值合法(t *testing.T) {
	assert.NoError(t, DefaultRoutingPolicy().Validate())
}

func TestRoutingPolicy_校验拒绝非法配置(t *testing.T) {
	mutate := func(f func(*RoutingPolicy)) *RoutingPolicy {
		p := DefaultRoutingPolicy()
		f(p)
		return p
	}
	tests := []struct {
		name string
		pol  *RoutingPolicy
	}{
		{"nil", nil},
		{"tierFactors 为空", mutate(func(p *RoutingPolicy) { p.TierFactors = nil })},
		{"未知层级", mutate(func(p *RoutingPolicy) { p.TierFactors[Tier("vip")] = 1 })},
		{"负层级因子", mutate(func(p *RoutingPolicy) { p.TierFactors[TierSecondary] = -0.1 })},
		{"primary 因子为 0", mutate(func(p *RoutingPolicy) { p.TierFactors[TierPrimary] = 0 })},
		{"alpha 非正", mutate(func(p *RoutingPolicy) { p.Health.Alpha = 0 })},
		{"gamma 为负", mutate(func(p *RoutingPolicy) { p.Load.Gamma = -1 })},
		{"cost max < min", mutate(func(p *RoutingPolicy) { p.Cost.Max = 0.1 })},
		{"ramp floor 越界", mutate(func(p *RoutingPolicy) { p.Ramp.Floor = 1.5 })},
		{"绑定 TTL 非正", mutate(func(p *RoutingPolicy) { p.Binding.TTLSeconds = 0 })},
		{"守卫阈值越界", mutate(func(p *RoutingPolicy) { p.Binding.KeepHealthMin = 2 })},
		{"负预算", mutate(func(p *RoutingPolicy) { p.Retry.FailoverBudget = -1 })},
		{"退避上限小于基数", mutate(func(p *RoutingPolicy) { p.Retry.BackoffMaxMs = 10 })},
		{"熔断阈值非正", mutate(func(p *RoutingPolicy) { p.Breaker.FailThreshold = 0 })},
		{"模型级熔断阈值非正", mutate(func(p *RoutingPolicy) { p.Breaker.ModelFailThreshold = 0 })},
		{"冷却上限小于起始", mutate(func(p *RoutingPolicy) { p.Breaker.CooldownMaxSeconds = 1 })},
		{"副本数非正", mutate(func(p *RoutingPolicy) { p.Degrade.MaxReplicas = 0 })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, tt.pol.Validate())
		})
	}
}

func TestRetryDecision_String(t *testing.T) {
	assert.Equal(t, "inplace", DecisionInPlaceRetry.String())
	assert.Equal(t, "cred_rotate", DecisionRotateCredential.String())
	assert.Equal(t, "failover", DecisionFailover.String())
	assert.Equal(t, "abort", DecisionAbort.String())
}
