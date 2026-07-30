package dispatch

import (
	"math"
)

// EffectiveWeight 权重合成函数 W(c)（基线方案 §4，纯函数）。
//
//	W(c) = baseWeight × tierFactor × healthFactor × headroom^γ × costFactor × rampFactor
//
// 返回合成权重与分解明细（供决策日志 / ForwardingTrace，修订 R5）。
func EffectiveWeight(c Channel, pol *RoutingPolicy) (float64, WeightBreakdown) {
	bd := WeightBreakdown{
		Base:     c.BaseWeight,
		Tier:     tierFactor(c.Tier, pol),
		Health:   healthFactor(c, pol),
		Headroom: headroomFactor(c, pol),
		Cost:     costFactor(c, pol),
		Ramp:     rampFactor(c, pol),
	}
	if bd.Base <= 0 {
		bd.Base = 0
	}
	bd.Effective = bd.Base * bd.Tier * bd.Health * bd.Headroom * bd.Cost * bd.Ramp
	return bd.Effective, bd
}

// tierFactor 层级偏置（基线方案 §4.1）。未配置的层级视为 0（不参与，由扩组兜底救回）。
func tierFactor(t Tier, pol *RoutingPolicy) float64 {
	f, ok := pol.TierFactors[t]
	if !ok {
		return 0
	}
	return math.Max(f, 0)
}

// healthFactor 健康因子（基线方案 §4.2）：
//
//	clamp(succEwma, 0.01, 1)^α × latencyPenalty
//	latencyPenalty = clamp(latRef / max(latEwma, latRef), 0.5, 1.0)
func healthFactor(c Channel, pol *RoutingPolicy) float64 {
	succ := c.SuccEwma
	if succ <= 0 && c.LatEwmaMs == 0 {
		// 无任何健康数据（新渠道）视为满分
		succ = 1
	}
	succ = clamp(succ, 0.01, 1.0)
	f := math.Pow(succ, pol.Health.Alpha)

	latRef := pol.Health.LatRefMs
	if latRef > 0 && c.LatEwmaMs > latRef {
		f *= clamp(latRef/c.LatEwmaMs, 0.5, 1.0)
	}
	return f
}

// headroomFactor 负载余量因子（基线方案 §4.3）：(1 - inflight/softLimit)^γ。
// 无容量信息（SoftLimit<=0）时恒为 1。
func headroomFactor(c Channel, pol *RoutingPolicy) float64 {
	if c.SoftLimit <= 0 {
		return 1
	}
	headroom := 1 - float64(c.Inflight)/float64(c.SoftLimit)
	if headroom <= 0 {
		return 0
	}
	return math.Pow(headroom, pol.Load.Gamma)
}

// costFactor 成本因子（基线方案 §4.4）：clamp((1/costRatio)^β, min, max)。
// CostRatio 是无量纲比例（非金额，float64 不受 decimal 强约束），<=0 视为 1.0。
func costFactor(c Channel, pol *RoutingPolicy) float64 {
	ratio := c.CostRatio
	if ratio <= 0 {
		ratio = 1.0
	}
	return clamp(math.Pow(1.0/ratio, pol.Cost.Beta), pol.Cost.Min, pol.Cost.Max)
}

// rampFactor 爬坡因子（基线方案 §4.5）：新渠道 / 熔断恢复后从小流量爬坡。
func rampFactor(c Channel, pol *RoutingPolicy) float64 {
	if c.RampElapsedMs < 0 {
		return 1
	}
	window := pol.Ramp.WindowSeconds * 1000
	if window <= 0 {
		return 1
	}
	return clamp(float64(c.RampElapsedMs)/float64(window), pol.Ramp.Floor, 1.0)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
