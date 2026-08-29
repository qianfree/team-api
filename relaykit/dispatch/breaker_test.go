package dispatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func breakerPol() BreakerPolicy { return DefaultRoutingPolicy().Breaker }

func TestEffectiveBreakerState_LazyTransition(t *testing.T) {
	p := breakerPol()
	s := BreakerSnapshot{State: BreakerOpen, OpenedAtMs: 0, CooldownMs: int64(p.CooldownSeconds) * 1000}

	assert.Equal(t, BreakerOpen, EffectiveBreakerState(s, 29_999), "冷却期内保持 OPEN")
	assert.Equal(t, BreakerHalfOpen, EffectiveBreakerState(s, 30_000), "冷却期满转 HALF_OPEN")
	assert.Equal(t, BreakerClosed, EffectiveBreakerState(BreakerSnapshot{State: BreakerClosed}, 0))
}

func TestBreakerOnFailure_WindowThresholdOpens(t *testing.T) {
	p := breakerPol() // failThreshold=8

	s := BreakerSnapshot{State: BreakerClosed, FailWindowCount: 7}
	s = BreakerOnFailure(s, false, 1000, p)
	assert.Equal(t, BreakerClosed, s.State, "未达阈值不熔断")

	s.FailWindowCount = 8
	s = BreakerOnFailure(s, false, 1000, p)
	assert.Equal(t, BreakerOpen, s.State)
	assert.Equal(t, int64(1000), s.OpenedAtMs)
	assert.Equal(t, int64(30_000), s.CooldownMs)
}

func TestBreakerOnFailure_FatalOpensImmediately(t *testing.T) {
	p := breakerPol()
	s := BreakerSnapshot{State: BreakerClosed, FailWindowCount: 0}
	s = BreakerOnFailure(s, true, 1000, p)
	assert.Equal(t, BreakerOpen, s.State, "CHANNEL_FATAL 一次直达熔断")
}

func TestBreakerOnFailure_ProbeFailureDoublesCooldown(t *testing.T) {
	p := breakerPol() // cooldown 30s, max 300s

	// OPEN 冷却期满（有效态 HALF_OPEN）时失败 = 探测失败 → 回 OPEN，冷却翻倍
	s := BreakerSnapshot{State: BreakerOpen, OpenedAtMs: 0, CooldownMs: 30_000}
	s = BreakerOnFailure(s, false, 30_000, p)
	assert.Equal(t, BreakerOpen, s.State)
	assert.Equal(t, int64(30_000), s.OpenedAtMs, "重新计时")
	assert.Equal(t, int64(60_000), s.CooldownMs, "冷却翻倍")

	// 连续探测失败：冷却指数递增至上限封顶
	for range 10 {
		s = BreakerOnFailure(s, false, s.OpenedAtMs+s.CooldownMs, p)
	}
	assert.Equal(t, int64(300_000), s.CooldownMs, "封顶 5min")

	// 冷却期内的失败不重复转移
	s2 := BreakerSnapshot{State: BreakerOpen, OpenedAtMs: 100_000, CooldownMs: 60_000}
	got := BreakerOnFailure(s2, false, 110_000, p)
	assert.Equal(t, s2, got)
}

func TestBreakerOnSuccess_ProbeSuccessResets(t *testing.T) {
	// HALF_OPEN（有效态）成功 → CLOSED 并复位
	s := BreakerSnapshot{State: BreakerOpen, OpenedAtMs: 0, CooldownMs: 30_000, FailWindowCount: 9}
	s = BreakerOnSuccess(s, 30_000)
	assert.Equal(t, BreakerClosed, s.State)
	assert.Zero(t, s.CooldownMs)
	assert.Zero(t, s.FailWindowCount)

	// 冷却期内（有效态仍 OPEN）的成功不改变状态（不存在真实请求，防御性）
	s = BreakerSnapshot{State: BreakerOpen, OpenedAtMs: 0, CooldownMs: 30_000}
	s = BreakerOnSuccess(s, 10_000)
	assert.Equal(t, BreakerOpen, s.State)

	// CLOSED 成功保持 CLOSED
	s = BreakerOnSuccess(BreakerSnapshot{State: BreakerClosed, FailWindowCount: 3}, 0)
	assert.Equal(t, BreakerClosed, s.State)
	assert.Zero(t, s.FailWindowCount, "成功清失败计数")
}

func TestBreaker_AllTransitionPaths(t *testing.T) {
	p := breakerPol()

	// CLOSED → OPEN → HALF_OPEN(探测失败) → OPEN → HALF_OPEN(探测成功) → CLOSED
	s := BreakerSnapshot{State: BreakerClosed, FailWindowCount: p.FailThreshold}
	s = BreakerOnFailure(s, false, 0, p)
	assert.Equal(t, BreakerOpen, s.State)

	probeAt := s.OpenedAtMs + s.CooldownMs
	assert.Equal(t, BreakerHalfOpen, EffectiveBreakerState(s, probeAt))

	s = BreakerOnFailure(s, false, probeAt, p)
	assert.Equal(t, BreakerOpen, s.State)
	assert.Equal(t, int64(60_000), s.CooldownMs)

	probeAt = s.OpenedAtMs + s.CooldownMs
	s = BreakerOnSuccess(s, probeAt)
	assert.Equal(t, BreakerClosed, s.State)
}

func TestShouldAutoDisable(t *testing.T) {
	p := breakerPol() // autoDisableAfter=600s

	s := BreakerSnapshot{State: BreakerOpen, OpenedAtMs: 0}
	assert.False(t, ShouldAutoDisable(s, 599_999, p))
	assert.True(t, ShouldAutoDisable(s, 600_000, p))
	assert.False(t, ShouldAutoDisable(BreakerSnapshot{State: BreakerClosed}, 600_000, p))
}

func TestNextCooldownMs(t *testing.T) {
	p := breakerPol()
	assert.Equal(t, int64(30_000), nextCooldownMs(0, p))
	assert.Equal(t, int64(60_000), nextCooldownMs(30_000, p))
	assert.Equal(t, int64(300_000), nextCooldownMs(200_000, p), "封顶")
}
