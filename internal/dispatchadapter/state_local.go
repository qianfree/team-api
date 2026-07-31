package dispatchadapter

import (
	"sync"
	"time"

	"github.com/qianfree/team-api/relaykit/dispatch"
)

// localState Redis 故障时的实例本地降级镜像（基线方案 §13）：
// 保护仍在、只是不再全局协同。Redis 恢复后自然弃用（无需回填）。
type localState struct {
	mu sync.Mutex

	probeMs map[int64]int64 // channelID → 最近探测放行时间（本地探测限流）

	leases map[int64]map[string]int64 // channelID → requestID → 过期时间戳 ms（严格容量渠道）

	credCD map[int64]time.Time // keyID → 冷却截止时间

	health map[int64]*localHealth // channelID → 本地健康/熔断镜像（实例级熔断兜底）
}

// localHealth 实例本地的简化健康镜像：只维持连续失败熔断兜底，不做完整 EWMA。
type localHealth struct {
	consecutiveFails int
}

func newLocalState() *localState {
	return &localState{
		probeMs: make(map[int64]int64),
		leases:  make(map[int64]map[string]int64),
		credCD:  make(map[int64]time.Time),
		health:  make(map[int64]*localHealth),
	}
}

// tryProbe 本地探测限流：每窗口本实例只放行一个（失去全局协同，仍防打爆）。
func (l *localState) tryProbe(channelID int64, windowMs int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now().UnixMilli()
	if now-l.probeMs[channelID] < windowMs {
		return false
	}
	l.probeMs[channelID] = now
	return true
}

// acquireLease 严格容量渠道的本地租约（修订 R4：实例级保守限额，fail-closed）。
func (l *localState) acquireLease(channelID int64, requestID string, limit int, leaseMs int64) bool {
	if limit <= 0 {
		return false // 未配置上限的严格渠道降级期间直接拒绝
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now().UnixMilli()
	m, ok := l.leases[channelID]
	if !ok {
		m = make(map[string]int64)
		l.leases[channelID] = m
	}
	// 清过期
	for id, exp := range m {
		if exp <= now {
			delete(m, id)
		}
	}
	if len(m) >= limit {
		return false
	}
	m[requestID] = now + leaseMs
	return true
}

func (l *localState) refreshLease(channelID int64, requestID string, leaseMs int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if m, ok := l.leases[channelID]; ok {
		if _, held := m[requestID]; held {
			m[requestID] = time.Now().UnixMilli() + leaseMs
		}
	}
}

func (l *localState) releaseLease(channelID int64, requestID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if m, ok := l.leases[channelID]; ok {
		delete(m, requestID)
	}
}

// coolCred 本地凭证冷却镜像（Redis 写失败时仍在本实例生效）。
func (l *localState) coolCred(keyID int64, ttl time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.credCD[keyID] = time.Now().Add(ttl)
}

func (l *localState) isCredCooled(keyID int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	until, ok := l.credCD[keyID]
	if !ok {
		return false
	}
	if time.Now().After(until) {
		delete(l.credCD, keyID)
		return false
	}
	return true
}

// observe Redis 不可用时的本地健康记账：只做连续失败计数，
// 供实例级熔断兜底判断（完整 EWMA 在 Redis 恢复后继续）。
func (l *localState) observe(o dispatch.Outcome) {
	l.mu.Lock()
	defer l.mu.Unlock()
	h, ok := l.health[o.ChannelID]
	if !ok {
		h = &localHealth{}
		l.health[o.ChannelID] = h
	}
	if o.Success {
		h.consecutiveFails = 0
		return
	}
	h.consecutiveFails++
}
