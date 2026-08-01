package dispatch

// 熔断状态机（基线方案 §7）。
//
// 状态转移逻辑为纯函数；计数与快照的原子存取由适配层（Redis Lua）负责。
// 适配层在 ReportOutcome 后读取窗口失败数，调用本文件的转移函数得到新快照并写回
//（写回本身用 Lua CAS 保证原子，阶段 1 实现）。
//
//	CLOSED → OPEN：滑动窗口内失败 ≥ failThreshold，或 CHANNEL_FATAL 一次直达
//	OPEN → HALF_OPEN：冷却期满（惰性判定）
//	HALF_OPEN → CLOSED：探测成功（rampFactor 开始爬坡）
//	HALF_OPEN → OPEN：探测失败，冷却翻倍（上限封顶）

// BreakerSnapshot 熔断器快照（与 Redis hash 字段一一对应）。
type BreakerSnapshot struct {
	State           BreakerState
	OpenedAtMs      int64 // 进入 OPEN 的时间戳（毫秒）
	CooldownMs      int64 // 当前冷却时长（指数递增）
	FailWindowCount int   // 滑动窗口内失败数（由 Lua 维护，此处只读）
}

// EffectiveBreakerState 惰性判定当前有效状态：OPEN 且冷却期满 → HALF_OPEN。
// 只做读侧判定，不修改快照（HALF_OPEN 的探测放行由 StatePort.TryProbeToken 原子控制）。
func EffectiveBreakerState(s BreakerSnapshot, nowMs int64) BreakerState {
	if s.State == BreakerOpen && nowMs-s.OpenedAtMs >= s.CooldownMs {
		return BreakerHalfOpen
	}
	return s.State
}

// BreakerOnFailure 失败事件转移。fatal 表示 CHANNEL_FATAL 类错误（一次直达熔断）。
func BreakerOnFailure(s BreakerSnapshot, fatal bool, nowMs int64, p BreakerPolicy) BreakerSnapshot {
	switch EffectiveBreakerState(s, nowMs) {
	case BreakerHalfOpen:
		// 探测失败：回 OPEN，冷却翻倍
		s.State = BreakerOpen
		s.OpenedAtMs = nowMs
		s.CooldownMs = nextCooldownMs(s.CooldownMs, p)
		return s

	case BreakerOpen:
		// 已在熔断中（未到冷却期），不重复转移
		return s

	default: // CLOSED
		if fatal || s.FailWindowCount >= p.FailThreshold {
			s.State = BreakerOpen
			s.OpenedAtMs = nowMs
			if s.CooldownMs <= 0 {
				s.CooldownMs = int64(p.CooldownSeconds) * 1000
			}
			return s
		}
		return s
	}
}

// BreakerOnSuccess 成功事件转移：HALF_OPEN 探测成功 → CLOSED，冷却复位。
func BreakerOnSuccess(s BreakerSnapshot, nowMs int64) BreakerSnapshot {
	if EffectiveBreakerState(s, nowMs) == BreakerHalfOpen || s.State == BreakerClosed {
		s.State = BreakerClosed
		s.OpenedAtMs = 0
		s.CooldownMs = 0
		s.FailWindowCount = 0
	}
	return s
}

// ShouldAutoDisable OPEN 持续超过 autoDisableAfter → 落库禁用 + 清绑定 + 告警（适配层执行）。
func ShouldAutoDisable(s BreakerSnapshot, nowMs int64, p BreakerPolicy) bool {
	return s.State == BreakerOpen && nowMs-s.OpenedAtMs >= int64(p.AutoDisableAfterSeconds)*1000
}

// nextCooldownMs 冷却指数递增，封顶 cooldownMax。
func nextCooldownMs(cur int64, p BreakerPolicy) int64 {
	base := int64(p.CooldownSeconds) * 1000
	maxMs := int64(p.CooldownMaxSeconds) * 1000
	if cur <= 0 {
		return base
	}
	next := cur * 2
	if next > maxMs {
		return maxMs
	}
	return next
}
