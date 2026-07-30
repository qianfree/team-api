package dispatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func healthyChannel(id int64, tier Tier, weight float64) Channel {
	return Channel{
		ID:            id,
		Tier:          tier,
		BaseWeight:    weight,
		SuccEwma:      1.0,
		RampElapsedMs: -1,
	}
}

func TestEffectiveWeight_健康满分基准(t *testing.T) {
	pol := DefaultRoutingPolicy()
	w, bd := EffectiveWeight(healthyChannel(1, TierPrimary, 10), pol)
	assert.InDelta(t, 10.0, w, 1e-9)
	assert.Equal(t, 1.0, bd.Tier)
	assert.Equal(t, 1.0, bd.Health)
	assert.Equal(t, 1.0, bd.Headroom)
	assert.Equal(t, 1.0, bd.Cost)
	assert.Equal(t, 1.0, bd.Ramp)
}

func TestEffectiveWeight_层级偏置(t *testing.T) {
	pol := DefaultRoutingPolicy()
	wp, _ := EffectiveWeight(healthyChannel(1, TierPrimary, 10), pol)
	ws, _ := EffectiveWeight(healthyChannel(2, TierSecondary, 10), pol)
	wr, _ := EffectiveWeight(healthyChannel(3, TierReserve, 10), pol)
	assert.InDelta(t, wp*0.15, ws, 1e-9)
	assert.InDelta(t, wp*0.02, wr, 1e-9)

	// 未知层级（策略未配置）权重为 0
	wu, _ := EffectiveWeight(healthyChannel(4, Tier("unknown"), 10), pol)
	assert.Zero(t, wu)
}

func TestEffectiveWeight_健康因子(t *testing.T) {
	pol := DefaultRoutingPolicy() // alpha=2

	c := healthyChannel(1, TierPrimary, 10)
	c.SuccEwma = 0.9
	_, bd := EffectiveWeight(c, pol)
	assert.InDelta(t, 0.81, bd.Health, 1e-9, "succ=0.9 → 0.81")

	c.SuccEwma = 0.5
	_, bd = EffectiveWeight(c, pol)
	assert.InDelta(t, 0.25, bd.Health, 1e-9)

	// 下限保护：极低成功率不至于权重归零（区别于熔断硬排除）
	c.SuccEwma = 0.001
	w, _ := EffectiveWeight(c, pol)
	assert.Greater(t, w, 0.0)

	// 延迟惩罚：超过 latRef 按比例衰减，最多减半
	c = healthyChannel(1, TierPrimary, 10)
	c.LatEwmaMs = 6000 // latRef=3000 → penalty 0.5
	_, bd = EffectiveWeight(c, pol)
	assert.InDelta(t, 0.5, bd.Health, 1e-9)

	c.LatEwmaMs = 100000 // penalty 钳制在 0.5
	_, bd = EffectiveWeight(c, pol)
	assert.InDelta(t, 0.5, bd.Health, 1e-9)

	// 无健康数据（新渠道）视为满分
	c = Channel{ID: 9, Tier: TierPrimary, BaseWeight: 10, RampElapsedMs: -1}
	_, bd = EffectiveWeight(c, pol)
	assert.Equal(t, 1.0, bd.Health)
}

func TestEffectiveWeight_负载余量(t *testing.T) {
	pol := DefaultRoutingPolicy() // gamma=2

	c := healthyChannel(1, TierPrimary, 10)
	c.SoftLimit = 10
	c.Inflight = 5
	_, bd := EffectiveWeight(c, pol)
	assert.InDelta(t, 0.25, bd.Headroom, 1e-9, "余量 0.5 → 0.5^2")

	// 饱和 → 权重坍缩为 0
	c.Inflight = 10
	w, _ := EffectiveWeight(c, pol)
	assert.Zero(t, w)

	// 超卖同样为 0
	c.Inflight = 15
	w, _ = EffectiveWeight(c, pol)
	assert.Zero(t, w)

	// 无容量信息恒为 1
	c.SoftLimit = 0
	c.Inflight = 999
	_, bd = EffectiveWeight(c, pol)
	assert.Equal(t, 1.0, bd.Headroom)
}

func TestEffectiveWeight_成本因子(t *testing.T) {
	pol := DefaultRoutingPolicy() // beta=0.5, clamp [0.5, 2.0]

	c := healthyChannel(1, TierPrimary, 10)
	c.CostRatio = 0.8 // 八折渠道 → (1/0.8)^0.5 ≈ 1.118
	_, bd := EffectiveWeight(c, pol)
	assert.InDelta(t, 1.118, bd.Cost, 0.001)

	c.CostRatio = 100 // 极贵渠道钳制在下限
	_, bd = EffectiveWeight(c, pol)
	assert.Equal(t, 0.5, bd.Cost)

	c.CostRatio = 0.0001 // 极便宜钳制在上限
	_, bd = EffectiveWeight(c, pol)
	assert.Equal(t, 2.0, bd.Cost)

	c.CostRatio = 0 // 未配置视为等价
	_, bd = EffectiveWeight(c, pol)
	assert.Equal(t, 1.0, bd.Cost)
}

func TestEffectiveWeight_爬坡因子(t *testing.T) {
	pol := DefaultRoutingPolicy() // window=120s, floor=0.05

	c := healthyChannel(1, TierPrimary, 10)

	c.RampElapsedMs = 0
	_, bd := EffectiveWeight(c, pol)
	assert.Equal(t, 0.05, bd.Ramp, "刚恢复从 floor 开始")

	c.RampElapsedMs = 60_000
	_, bd = EffectiveWeight(c, pol)
	assert.InDelta(t, 0.5, bd.Ramp, 1e-9)

	c.RampElapsedMs = 120_000
	_, bd = EffectiveWeight(c, pol)
	assert.Equal(t, 1.0, bd.Ramp)

	c.RampElapsedMs = -1
	_, bd = EffectiveWeight(c, pol)
	assert.Equal(t, 1.0, bd.Ramp, "非爬坡期恒为 1")
}

func TestEffectiveWeight_基础权重非正(t *testing.T) {
	pol := DefaultRoutingPolicy()
	c := healthyChannel(1, TierPrimary, 0)
	w, _ := EffectiveWeight(c, pol)
	assert.Zero(t, w)

	c.BaseWeight = -5
	w, _ = EffectiveWeight(c, pol)
	assert.Zero(t, w)
}
