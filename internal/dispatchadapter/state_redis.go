// Package dispatchadapter 是渠道调度核心库（relaykit/dispatch）的适配层：
// 全部 I/O 在此实现——渠道目录快照（PostgreSQL + 内存缓存）、运行时状态
// （Redis Lua 原子操作）、Redis 故障降级、路由策略加载与热更新。
//
// 依赖方向：relay/handler → relaykit/dispatch ← internal/logic/dispatchadapter。
package dispatchadapter

import (
	"context"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relaykit/dispatch"
)

// BreakerOpenHook 熔断打开事件回调（指标计数）。由 cmd 启动时注入
// monitor.TrackDispatchBreakerOpen，避免 dispatchadapter → monitor 的导入环
// （monitor → middleware → logic/relay → dispatchadapter）。
var BreakerOpenHook func()

// SetBreakerOpenHook 注入熔断打开事件回调。
func SetBreakerOpenHook(hook func()) { BreakerOpenHook = hook }

// Redis key 统一前缀（基线方案 §10.1 + 开发计划 §4.2）。
const (
	keyPrefix     = "dispatch:v1:"
	keyBind       = keyPrefix + "bind:"     // + sessionKey → channelID（STRING）
	keyBindRev    = keyPrefix + "bindrev:"  // + channelID → 绑定 key 集合（SET，失效用）
	keyHealth     = keyPrefix + "health:"   // + <ch>:<model> → succ_ewma/lat_ewma（HASH）
	keyBreaker    = keyPrefix + "breaker:"  // + <ch> 或 <ch>:<model> → 熔断快照（HASH）
	keyInflight   = keyPrefix + "inflight:" // + <ch> → 租约（ZSET，member=requestID score=过期时间戳）
	keyCredCD     = keyPrefix + "credcd:"   // + <keyID> → 冷却标记（STRING，修订 R1）
	keyLimit429   = keyPrefix + "limit429:" // + <ch> → 429 起始水位 EWMA（HASH，softLimit 自动估计）
	stateKeyTTLMs = int64(24 * time.Hour / time.Millisecond)
)

// healthDecayFor 修订 R6：健康 EWMA 衰减系数按错误类别分档。
// CLIENT / CREDENTIAL 类不上报健康（协调器已过滤），此处只处理会到达的类别。
func healthDecayFor(class dispatch.ErrorClass) float64 {
	switch class {
	case dispatch.ErrClassRateLimit:
		return 0.97 // 429 主要喂 softLimit 估计器，基本不伤健康
	case dispatch.ErrClassChannelFatal:
		return 0.70 // 重罚，加速权重坍缩
	default: // TRANSIENT / TIMEOUT
		return 0.93 // 轻罚，瞬时抖动不重伤
	}
}

// RedisState 实现 dispatch.StatePort。所有多步操作单条 Lua 原子执行（消除 H1）；
// Redis 故障时逐方法降级到 local（基线方案 §13 + 修订 R4）。
type RedisState struct {
	local   *localState
	outcome chan dispatch.Outcome
	policy  func() *dispatch.RoutingPolicy // 取当前策略（熔断参数、副本数等）
	// strictLookup 返回渠道是否严格容量及其手动并发上限（修订 R4，由目录提供）
	strictLookup func(channelID int64) (strict bool, maxConcurrency int)
	stop         chan struct{}
}

// NewRedisState 构造运行时状态适配器。policy 与 strictLookup 由 wire 注入。
func NewRedisState(policy func() *dispatch.RoutingPolicy, strictLookup func(int64) (bool, int)) *RedisState {
	s := &RedisState{
		local:        newLocalState(),
		outcome:      make(chan dispatch.Outcome, 4096),
		policy:       policy,
		strictLookup: strictLookup,
		stop:         make(chan struct{}),
	}
	return s
}

// Start 启动健康上报后台 worker（fire-and-forget 批量消费，消除 H2）。
func (s *RedisState) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case o := <-s.outcome:
				s.processOutcome(ctx, o)
			case <-s.stop:
				return
			}
		}
	}()
}

// Stop 停止后台 worker。
func (s *RedisState) Stop() { close(s.stop) }

// ---------------------------------------------------------------------------
// 绑定（会话粘性）
// ---------------------------------------------------------------------------

// GetBinding 读取会话绑定。Redis 故障 → 视为无绑定（退化为纯 HRW，粘性大体保留）。
func (s *RedisState) GetBinding(ctx context.Context, sessionKey string) (int64, bool) {
	v, err := g.Redis().Do(ctx, "GET", keyBind+sessionKey)
	if err != nil {
		return 0, false
	}
	if v.IsNil() {
		return 0, false
	}
	id, perr := strconv.ParseInt(v.String(), 10, 64)
	return id, perr == nil && id > 0
}

// luaBindSet 绑定 CAS 写入 + 反向索引维护（基线方案 §10.3）。
const luaBindSet = `
local old = redis.call('GET', KEYS[1])
if old and old ~= ARGV[1] then
  redis.call('SREM', ARGV[3] .. old, KEYS[1])
end
redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[2])
redis.call('SADD', ARGV[3] .. ARGV[1], KEYS[1])
redis.call('EXPIRE', ARGV[3] .. ARGV[1], tonumber(ARGV[2]) + 60)
return 1`

// SetBinding 写入/更新会话绑定（CAS + 反向索引）。
func (s *RedisState) SetBinding(ctx context.Context, sessionKey string, channelID int64, ttl time.Duration) {
	_, err := g.Redis().Do(ctx, "EVAL", luaBindSet, 1,
		keyBind+sessionKey, channelID, int64(ttl.Seconds()), keyBindRev)
	if err != nil {
		g.Log().Warningf(ctx, "[Dispatch] 绑定写入失败: session=%s channel=%d err=%v", sessionKey, channelID, err)
	}
}

// luaBindTouch 绑定滑动续期（连带反向索引）。
const luaBindTouch = `
local v = redis.call('GET', KEYS[1])
if not v then return 0 end
redis.call('EXPIRE', KEYS[1], ARGV[1])
redis.call('EXPIRE', ARGV[2] .. v, tonumber(ARGV[1]) + 60)
return 1`

// TouchBinding 成功请求后滑动续期绑定。
func (s *RedisState) TouchBinding(ctx context.Context, sessionKey string, ttl time.Duration) {
	_, _ = g.Redis().Do(ctx, "EVAL", luaBindTouch, 1,
		keyBind+sessionKey, int64(ttl.Seconds()), keyBindRev)
}

// luaBindInvalidate 渠道禁用/长期熔断时批量清理其全部绑定。
const luaBindInvalidate = `
local keys = redis.call('SMEMBERS', KEYS[1])
for _, key in ipairs(keys) do redis.call('DEL', key) end
redis.call('DEL', KEYS[1])
return #keys`

// InvalidateChannelBindings 清理指向某渠道的全部绑定（渠道禁用/删除时调用）。
func (s *RedisState) InvalidateChannelBindings(ctx context.Context, channelID int64) {
	if _, err := g.Redis().Do(ctx, "EVAL", luaBindInvalidate, 1,
		keyBindRev+strconv.FormatInt(channelID, 10)); err != nil {
		g.Log().Warningf(ctx, "[Dispatch] 清理渠道绑定失败: channel=%d err=%v", channelID, err)
	}
}

// ---------------------------------------------------------------------------
// 健康上报（fire-and-forget + 后台批量）
// ---------------------------------------------------------------------------

// ReportOutcome 投递结果事件到有界队列，满则丢弃并告警（不阻塞热路径）。
func (s *RedisState) ReportOutcome(o dispatch.Outcome) {
	select {
	case s.outcome <- o:
	default:
		g.Log().Warning(context.Background(), "[Dispatch] 健康上报队列已满，丢弃事件")
	}
}

// luaHealthObserve 健康 EWMA 原子更新（基线方案 §10.2 + 修订 R6 分档衰减）。
// ARGV: success(0/1), latencyMs, decay, nowMs, keyTtlMs
const luaHealthObserve = `
local succ = tonumber(redis.call('HGET', KEYS[1], 'succ_ewma') or '1.0')
local lat  = tonumber(redis.call('HGET', KEYS[1], 'lat_ewma') or '0')
if ARGV[1] == '1' then
  succ = succ * 0.9 + 0.1
  local l = tonumber(ARGV[2])
  if l > 0 then
    if lat == 0 then lat = l else lat = lat * 0.7 + l * 0.3 end
  end
else
  succ = succ * tonumber(ARGV[3])
end
redis.call('HSET', KEYS[1], 'succ_ewma', succ, 'lat_ewma', lat, 'updated_ms', ARGV[4])
redis.call('PEXPIRE', KEYS[1], ARGV[5])
return 1`

// luaBreakerFail 熔断失败转移（与 dispatch/breaker.go 纯函数规则保持一致，改动需同步）。
// first_opened_ms 记录本轮故障期的起点（探测失败重置 opened_ms 但不重置它），供自动禁用判定。
// ARGV: nowMs, windowMs, failThreshold, fatal(0/1), cooldownBaseMs, cooldownMaxMs, keyTtlMs
const luaBreakerFail = `
local h = KEYS[1]
local state  = tonumber(redis.call('HGET', h, 'state') or '0')
local opened = tonumber(redis.call('HGET', h, 'opened_ms') or '0')
local cd     = tonumber(redis.call('HGET', h, 'cooldown_ms') or '0')
local now    = tonumber(ARGV[1])
if state == 1 then
  if now - opened >= cd then
    -- 有效态 HALF_OPEN：探测失败 → 回 OPEN，冷却翻倍封顶
    cd = cd * 2
    if cd <= 0 then cd = tonumber(ARGV[5]) end
    if cd > tonumber(ARGV[6]) then cd = tonumber(ARGV[6]) end
    redis.call('HSET', h, 'state', 1, 'opened_ms', now, 'cooldown_ms', cd, 'fail_count', 0)
  end
  redis.call('PEXPIRE', h, ARGV[7])
  return -1
end
-- CLOSED：滑动窗口计数
local ws = tonumber(redis.call('HGET', h, 'window_start_ms') or '0')
local fc = tonumber(redis.call('HGET', h, 'fail_count') or '0')
if now - ws > tonumber(ARGV[2]) then ws = now; fc = 0 end
fc = fc + 1
if ARGV[4] == '1' or fc >= tonumber(ARGV[3]) then
  redis.call('HSET', h, 'state', 1, 'opened_ms', now, 'cooldown_ms', ARGV[5], 'fail_count', 0, 'window_start_ms', ws)
  if redis.call('HEXISTS', h, 'first_opened_ms') == 0 then
    redis.call('HSET', h, 'first_opened_ms', now)
  end
else
  redis.call('HSET', h, 'fail_count', fc, 'window_start_ms', ws)
end
redis.call('PEXPIRE', h, ARGV[7])
return fc`

// luaBreakerSuccess 熔断成功转移：HALF_OPEN 探测成功 → CLOSED（记录 recovered_ms 供爬坡，
// 清除 first_opened_ms 结束本轮故障期）。
// ARGV: nowMs, keyTtlMs
const luaBreakerSuccess = `
local h = KEYS[1]
local state  = tonumber(redis.call('HGET', h, 'state') or '0')
local opened = tonumber(redis.call('HGET', h, 'opened_ms') or '0')
local cd     = tonumber(redis.call('HGET', h, 'cooldown_ms') or '0')
local now    = tonumber(ARGV[1])
if state == 1 and now - opened >= cd then
  redis.call('HSET', h, 'state', 0, 'opened_ms', 0, 'cooldown_ms', 0, 'fail_count', 0, 'recovered_ms', now)
  redis.call('HDEL', h, 'first_opened_ms')
elseif state == 0 then
  redis.call('HSET', h, 'fail_count', 0)
  redis.call('HDEL', h, 'first_opened_ms')
end
redis.call('PEXPIRE', h, ARGV[2])
return 1`

// luaLimit429Observe softLimit 自动估计（基线方案 §8.2）：记录收到 429 时的 inflight 水位，
// onset_ewma = onset×0.8 + 当前水位×0.2（首次直接取当前水位）。
// KEYS[1]=limit429 key, KEYS[2]=inflight key; ARGV: nowMs, keyTtlMs
const luaLimit429Observe = `
redis.call('ZREMRANGEBYSCORE', KEYS[2], '-inf', ARGV[1])
local inflight = redis.call('ZCARD', KEYS[2])
local onset = tonumber(redis.call('HGET', KEYS[1], 'onset_ewma') or '0')
if onset <= 0 then
  onset = inflight
else
  onset = onset * 0.8 + inflight * 0.2
end
redis.call('HSET', KEYS[1], 'onset_ewma', onset, 'updated_ms', ARGV[1])
redis.call('PEXPIRE', KEYS[1], ARGV[2])
return 1`

// processOutcome 消费一条结果事件：健康 EWMA + 渠道级/模型级熔断转移。
// 限流类不喂熔断窗口（429 属容量信号而非故障信号），改喂 softLimit 估计器。
func (s *RedisState) processOutcome(ctx context.Context, o dispatch.Outcome) {
	pol := s.policy()
	now := time.Now().UnixMilli()
	chStr := strconv.FormatInt(o.ChannelID, 10)
	healthKey := keyHealth + chStr + ":" + o.Model

	success := "0"
	if o.Success {
		success = "1"
	}
	if _, err := g.Redis().Do(ctx, "EVAL", luaHealthObserve, 1, healthKey,
		success, o.LatencyMs, healthDecayFor(o.Class), now, stateKeyTTLMs); err != nil {
		s.local.observe(o) // 降级：实例本地健康镜像
		return
	}

	// 429 → softLimit 自动估计器（基线方案 §8.2）
	if o.Class == dispatch.ErrClassRateLimit {
		_, _ = g.Redis().Do(ctx, "EVAL", luaLimit429Observe, 2,
			keyLimit429+chStr, keyInflight+chStr, now, stateKeyTTLMs)
	}

	breakerKeys := []string{keyBreaker + chStr}
	// 模型级熔断只喂模型相关的致命错误（如 404 模型不存在），避免误伤其它模型
	if o.Class == dispatch.ErrClassChannelFatal || o.Success {
		breakerKeys = append(breakerKeys, keyBreaker+chStr+":"+o.Model)
	}
	for _, bk := range breakerKeys {
		if o.Success {
			_, _ = g.Redis().Do(ctx, "EVAL", luaBreakerSuccess, 1, bk, now, stateKeyTTLMs)
			continue
		}
		if o.Class == dispatch.ErrClassRateLimit {
			continue
		}
		fatal := "0"
		if o.Class == dispatch.ErrClassChannelFatal {
			fatal = "1"
		}
		b := pol.Breaker
		ret, err := g.Redis().Do(ctx, "EVAL", luaBreakerFail, 1, bk,
			now, int64(b.WindowSeconds)*1000, b.FailThreshold, fatal,
			int64(b.CooldownSeconds)*1000, int64(b.CooldownMaxSeconds)*1000, stateKeyTTLMs)
		// CLOSED→OPEN 转移（返回值为窗口失败数且达到阈值/致命直达）计入熔断打开指标
		if err == nil && ret.Int() >= 0 && (fatal == "1" || ret.Int() >= b.FailThreshold) && BreakerOpenHook != nil {
			BreakerOpenHook()
		}
	}
}

// luaBreakerManualReset 手动恢复：熔断复位为 CLOSED 并记录 recovered_ms（触发爬坡因子）。
// ARGV: nowMs, keyTtlMs
const luaBreakerManualReset = `
redis.call('HSET', KEYS[1], 'state', 0, 'opened_ms', 0, 'cooldown_ms', 0, 'fail_count', 0, 'recovered_ms', ARGV[1])
redis.call('HDEL', KEYS[1], 'first_opened_ms')
redis.call('PEXPIRE', KEYS[1], ARGV[2])
return 1`

// MarkChannelRecovered 渠道被运营手动启用/恢复时调用：复位熔断并开启爬坡窗口，
// 避免恢复瞬间被 HRW 一次性灌满流量（基线方案 §4.5 rampFactor）。
func (s *RedisState) MarkChannelRecovered(ctx context.Context, channelID int64) {
	if _, err := g.Redis().Do(ctx, "EVAL", luaBreakerManualReset, 1,
		keyBreaker+strconv.FormatInt(channelID, 10), time.Now().UnixMilli(), stateKeyTTLMs); err != nil {
		g.Log().Warningf(ctx, "[Dispatch] 手动恢复标记失败: channel=%d err=%v", channelID, err)
	}
}

// ---------------------------------------------------------------------------
// 健康度重置
// ---------------------------------------------------------------------------

// resetHealthSuccEwma 重置健康度时写入的成功率 EWMA：对应健康分≈80（0.9²×100=81）。
const resetHealthSuccEwma = 0.9

// luaHealthReset 健康 EWMA 重置（管理后台"重置健康度"）：成功率恢复为健康值、延迟清零。
// ARGV: succ, nowMs, keyTtlMs
const luaHealthReset = `
local succ = tonumber(ARGV[1])
if succ <= 0 then succ = 1 end
redis.call('HSET', KEYS[1], 'succ_ewma', succ, 'lat_ewma', 0, 'updated_ms', ARGV[2])
redis.call('PEXPIRE', KEYS[1], ARGV[3])
return 1`

// ResetHealth 重置渠道健康（管理后台"重置健康度"）：渠道级 + 各模型级熔断复位为 CLOSED
// 并开启爬坡窗口（recovered_ms），各模型成功率 EWMA 恢复为健康值，渠道立即恢复被调度选择的能力。
// 失败仅记 Debug 日志（管理后台手动操作，重试成本低）。
func (s *RedisState) ResetHealth(ctx context.Context, channelID int64, models []string) {
	chStr := strconv.FormatInt(channelID, 10)
	now := time.Now().UnixMilli()

	s.resetBreakerState(ctx, keyBreaker+chStr, now)
	for _, model := range models {
		s.resetBreakerState(ctx, keyBreaker+chStr+":"+model, now)
		if _, err := g.Redis().Do(ctx, "EVAL", luaHealthReset, 1,
			keyHealth+chStr+":"+model, resetHealthSuccEwma, now, stateKeyTTLMs); err != nil {
			g.Log().Debugf(ctx, "[Dispatch] 健康重置失败: channel=%d model=%s err=%v", channelID, model, err)
		}
	}
}

// resetBreakerState 熔断复位为 CLOSED 并记录 recovered_ms（开启爬坡窗口）。
func (s *RedisState) resetBreakerState(ctx context.Context, key string, nowMs int64) {
	_, _ = g.Redis().Do(ctx, "EVAL", luaBreakerManualReset, 1, key, nowMs, stateKeyTTLMs)
}

// ---------------------------------------------------------------------------
// 熔断探测令牌
// ---------------------------------------------------------------------------

// luaProbeToken HALF_OPEN 探测令牌：多副本下每窗口全局只放行 1 个真实请求（基线方案 §10.4）。
// ARGV: nowMs, probeWindowMs
const luaProbeToken = `
local h = KEYS[1]
local state  = tonumber(redis.call('HGET', h, 'state') or '0')
local opened = tonumber(redis.call('HGET', h, 'opened_ms') or '0')
local cd     = tonumber(redis.call('HGET', h, 'cooldown_ms') or '0')
local now    = tonumber(ARGV[1])
if not (state == 1 and now - opened >= cd) then return 0 end
local ps = tonumber(redis.call('HGET', h, 'probe_ms') or '0')
if now - ps < tonumber(ARGV[2]) then return 0 end
redis.call('HSET', h, 'probe_ms', now)
return 1`

// TryProbeToken 尝试获取 HALF_OPEN 探测令牌。Redis 故障 → 实例本地限流兜底。
func (s *RedisState) TryProbeToken(ctx context.Context, channelID int64) bool {
	pol := s.policy()
	v, err := g.Redis().Do(ctx, "EVAL", luaProbeToken, 1,
		keyBreaker+strconv.FormatInt(channelID, 10),
		time.Now().UnixMilli(), int64(pol.Breaker.ProbeWindowSeconds)*1000)
	if err != nil {
		return s.local.tryProbe(channelID, int64(pol.Breaker.ProbeWindowSeconds)*1000)
	}
	return v.Int() == 1
}

// ---------------------------------------------------------------------------
// 容量租约
// ---------------------------------------------------------------------------

// luaLeaseAcquire 租约获取（基线方案 §10.5）。softLimit<=0 表示无上限，仅记录 inflight。
// ARGV: nowMs, expireAtMs, softLimit, requestID, keyTtlMs
const luaLeaseAcquire = `
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
local limit = tonumber(ARGV[3])
if limit > 0 and redis.call('ZCARD', KEYS[1]) >= limit then return 0 end
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[4])
redis.call('PEXPIRE', KEYS[1], ARGV[5])
return 1`

// AcquireLease 获取容量租约。Redis 故障：严格容量渠道 fail-closed（实例级保守限额，
// 修订 R4），其余 fail-open。
func (s *RedisState) AcquireLease(ctx context.Context, channelID int64, softLimit int, requestID string) bool {
	pol := s.policy()
	leaseMs := int64(pol.Load.LeaseSeconds) * 1000
	now := time.Now().UnixMilli()
	v, err := g.Redis().Do(ctx, "EVAL", luaLeaseAcquire, 1,
		keyInflight+strconv.FormatInt(channelID, 10),
		now, now+leaseMs, softLimit, requestID, leaseMs+60_000)
	if err != nil {
		strict, maxConc := false, 0
		if s.strictLookup != nil {
			strict, maxConc = s.strictLookup(channelID)
		}
		if !strict {
			return true // fail-open
		}
		limit := localFallbackLimit(maxConc, pol.Degrade.MaxReplicas)
		return s.local.acquireLease(channelID, requestID, limit, leaseMs)
	}
	return v.Int() == 1
}

// luaLeaseRefresh 续租：仅在租约仍存在时更新过期时间（不能复活已释放的租约）。
// ARGV: requestID, expireAtMs, keyTtlMs
const luaLeaseRefresh = `
if redis.call('ZSCORE', KEYS[1], ARGV[1]) then
  redis.call('ZADD', KEYS[1], ARGV[2], ARGV[1])
  redis.call('PEXPIRE', KEYS[1], ARGV[3])
  return 1
end
return 0`

// RefreshLease 长流式请求续租。
func (s *RedisState) RefreshLease(ctx context.Context, channelID int64, requestID string) {
	pol := s.policy()
	leaseMs := int64(pol.Load.LeaseSeconds) * 1000
	_, err := g.Redis().Do(ctx, "EVAL", luaLeaseRefresh, 1,
		keyInflight+strconv.FormatInt(channelID, 10),
		requestID, time.Now().UnixMilli()+leaseMs, leaseMs+60_000)
	if err != nil {
		s.local.refreshLease(channelID, requestID, leaseMs)
	}
}

// ReleaseLease 释放租约。
func (s *RedisState) ReleaseLease(ctx context.Context, channelID int64, requestID string) {
	_, err := g.Redis().Do(ctx, "ZREM", keyInflight+strconv.FormatInt(channelID, 10), requestID)
	if err != nil {
		s.local.releaseLease(channelID, requestID)
	}
}

// localFallbackLimit 修订 R4：实例级保守限额 = floor(全局硬上限 / 最大副本数)，至少 1。
// 未配置 max_concurrency 的严格渠道降级期间直接拒绝（返回 0 → 排除）。
func localFallbackLimit(maxConcurrency, maxReplicas int) int {
	if maxConcurrency <= 0 {
		return 0
	}
	if maxReplicas <= 0 {
		maxReplicas = 1
	}
	return max(maxConcurrency/maxReplicas, 1)
}

// ---------------------------------------------------------------------------
// 凭证冷却（修订 R1）
// ---------------------------------------------------------------------------

// IsCredentialCooled 查询渠道 Key 是否在冷却中。Redis 故障 → 查实例本地镜像。
func (s *RedisState) IsCredentialCooled(ctx context.Context, keyID int64) bool {
	v, err := g.Redis().Do(ctx, "EXISTS", keyCredCD+strconv.FormatInt(keyID, 10))
	if err != nil {
		return s.local.isCredCooled(keyID)
	}
	return v.Int() == 1
}

// CoolCredential 冷却渠道 Key（401/403/Key 额度耗尽时调用）。
func (s *RedisState) CoolCredential(ctx context.Context, keyID int64, ttl time.Duration) {
	s.local.coolCred(keyID, ttl) // 本地镜像同步写，Redis 故障期间仍生效
	_, err := g.Redis().Do(ctx, "SET", keyCredCD+strconv.FormatInt(keyID, 10),
		time.Now().UnixMilli(), "EX", int64(ttl.Seconds()))
	if err != nil {
		g.Log().Warningf(ctx, "[Dispatch] 凭证冷却写入失败: key=%d err=%v", keyID, err)
	}
}

// ---------------------------------------------------------------------------
// 运行时读值（供目录快照合并健康/熔断/负载）
// ---------------------------------------------------------------------------

// RuntimeReadout 单个渠道×模型的运行时读值。
type RuntimeReadout struct {
	SuccEwma      float64
	LatEwmaMs     float64
	Inflight      int
	Breaker       dispatch.BreakerState
	ModelBreaker  dispatch.BreakerState
	RecoveredMs   int64   // 渠道级熔断最近恢复时间（0 = 无记录）
	FirstOpenedMs int64   // 渠道级本轮故障期起点（0 = 非故障期），供自动禁用判定
	Onset429Ewma  float64 // 429 起始水位 EWMA（0 = 无 429 历史），供 softLimit 自动估计
}

// ReadRuntime 读取渠道×模型的运行时状态（目录刷新循环调用，非请求热路径）。
// Redis 故障 → 返回乐观默认值（健康满分 + CLOSED），由 last-known 快照语义兜底。
func (s *RedisState) ReadRuntime(ctx context.Context, channelID int64, model string) RuntimeReadout {
	out := RuntimeReadout{SuccEwma: 1}
	chStr := strconv.FormatInt(channelID, 10)
	now := time.Now().UnixMilli()

	if v, err := g.Redis().Do(ctx, "HMGET", keyHealth+chStr+":"+model, "succ_ewma", "lat_ewma"); err == nil {
		vals := v.Vars()
		if len(vals) == 2 {
			if !vals[0].IsNil() {
				out.SuccEwma = vals[0].Float64()
			}
			if !vals[1].IsNil() {
				out.LatEwmaMs = vals[1].Float64()
			}
		}
	}
	out.Breaker, out.RecoveredMs, out.FirstOpenedMs = s.readBreaker(ctx, keyBreaker+chStr, now)
	out.ModelBreaker, _, _ = s.readBreaker(ctx, keyBreaker+chStr+":"+model, now)
	if v, err := g.Redis().Do(ctx, "ZCOUNT", keyInflight+chStr, now, "+inf"); err == nil {
		out.Inflight = v.Int()
	}
	if v, err := g.Redis().Do(ctx, "HGET", keyLimit429+chStr, "onset_ewma"); err == nil && !v.IsNil() {
		out.Onset429Ewma = v.Float64()
	}
	return out
}

// readBreaker 读取熔断快照并做惰性 OPEN→HALF_OPEN 判定。
func (s *RedisState) readBreaker(ctx context.Context, key string, nowMs int64) (dispatch.BreakerState, int64, int64) {
	v, err := g.Redis().Do(ctx, "HMGET", key, "state", "opened_ms", "cooldown_ms", "recovered_ms", "first_opened_ms")
	if err != nil {
		return dispatch.BreakerClosed, 0, 0
	}
	vals := v.Vars()
	if len(vals) != 5 || vals[0].IsNil() {
		return dispatch.BreakerClosed, 0, 0
	}
	snap := dispatch.BreakerSnapshot{
		State:      dispatch.BreakerState(vals[0].Int()),
		OpenedAtMs: vals[1].Int64(),
		CooldownMs: vals[2].Int64(),
	}
	return dispatch.EffectiveBreakerState(snap, nowMs), vals[3].Int64(), vals[4].Int64()
}
