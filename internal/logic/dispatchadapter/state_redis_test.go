package dispatchadapter

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/gogf/gf/contrib/nosql/redis/v2"

	"github.com/qianfree/team-api/relaykit/dispatch"
)

var mr *miniredis.Miniredis

func TestMain(m *testing.M) {
	var err error
	mr, err = miniredis.Run()
	if err != nil {
		panic(err)
	}
	gredis.SetConfig(&gredis.Config{Address: mr.Addr()})
	code := m.Run()
	mr.Close()
	os.Exit(code)
}

func newTestState(t *testing.T, mutate func(*dispatch.RoutingPolicy)) *RedisState {
	t.Helper()
	mr.FlushAll()
	pol := dispatch.DefaultRoutingPolicy()
	if mutate != nil {
		mutate(pol)
	}
	return NewRedisState(func() *dispatch.RoutingPolicy { return pol }, nil)
}

// Test并发健康EWMA原子性 H1 修复验证：100 并发失败经 Lua 原子读-算-写，
// EWMA 精确等于 0.93^100，熔断窗口计数精确 =100（读改写竞态会导致丢失更新）。
func Test并发健康EWMA原子性(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, func(p *dispatch.RoutingPolicy) {
		p.Breaker.FailThreshold = 1000 // 阈值调高，只验证计数不触发熔断
	})

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.processOutcome(ctx, dispatch.Outcome{
				ChannelID: 1, Model: "gpt-4o", Success: false,
				Class: dispatch.ErrClassTransient, LatencyMs: 50,
			})
		}()
	}
	wg.Wait()

	rt := s.ReadRuntime(ctx, 1, "gpt-4o")
	expected := math.Pow(0.93, 100)
	assert.InEpsilon(t, expected, rt.SuccEwma, 1e-9, "EWMA 必须精确无丢失更新")

	v, err := g.Redis().Do(ctx, "HGET", keyBreaker+"1", "fail_count")
	require.NoError(t, err)
	assert.Equal(t, 100, v.Int(), "熔断窗口失败计数必须精确 =100")
}

func Test健康衰减分档(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	cases := []struct {
		channel int64
		class   dispatch.ErrorClass
		decay   float64
	}{
		{1, dispatch.ErrClassTransient, 0.93},
		{2, dispatch.ErrClassTimeout, 0.93},
		{3, dispatch.ErrClassRateLimit, 0.97},
		{4, dispatch.ErrClassChannelFatal, 0.70},
	}
	for _, c := range cases {
		s.processOutcome(ctx, dispatch.Outcome{ChannelID: c.channel, Model: "m", Success: false, Class: c.class})
		rt := s.ReadRuntime(ctx, c.channel, "m")
		assert.InEpsilon(t, c.decay, rt.SuccEwma, 1e-9, "class=%s", c.class)
	}

	// 成功恢复：succ = succ×0.9 + 0.1，延迟 EWMA 记录
	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 1, Model: "m", Success: true, LatencyMs: 800})
	rt := s.ReadRuntime(ctx, 1, "m")
	assert.InEpsilon(t, 0.93*0.9+0.1, rt.SuccEwma, 1e-9)
	assert.InEpsilon(t, 800, rt.LatEwmaMs, 1e-9)
}

func Test限流不喂熔断窗口(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	for range 20 {
		s.processOutcome(ctx, dispatch.Outcome{ChannelID: 5, Model: "m", Success: false, Class: dispatch.ErrClassRateLimit})
	}
	v, err := g.Redis().Do(ctx, "HGET", keyBreaker+"5", "fail_count")
	require.NoError(t, err)
	assert.True(t, v.IsNil() || v.Int() == 0, "429 不得计入熔断失败窗口")

	rt := s.ReadRuntime(ctx, 5, "m")
	assert.Equal(t, dispatch.BreakerClosed, rt.Breaker)
}

func Test熔断转移与探测令牌(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil) // failThreshold=8

	// 8 次瞬时失败 → 渠道级熔断 OPEN
	for range 8 {
		s.processOutcome(ctx, dispatch.Outcome{ChannelID: 7, Model: "m", Success: false, Class: dispatch.ErrClassTransient})
	}
	rt := s.ReadRuntime(ctx, 7, "m")
	assert.Equal(t, dispatch.BreakerOpen, rt.Breaker)

	// 冷却期内探测令牌不放行
	assert.False(t, s.TryProbeToken(ctx, 7))

	// 手动把 opened_ms 拨回 31s 前（冷却 30s）→ 有效态 HALF_OPEN
	_, err := g.Redis().Do(ctx, "HSET", keyBreaker+"7", "opened_ms", time.Now().UnixMilli()-31_000)
	require.NoError(t, err)
	rt = s.ReadRuntime(ctx, 7, "m")
	assert.Equal(t, dispatch.BreakerHalfOpen, rt.Breaker)

	// 探测令牌：每窗口全局只放行一个
	assert.True(t, s.TryProbeToken(ctx, 7))
	assert.False(t, s.TryProbeToken(ctx, 7), "同窗口第二个探测必须拒绝")

	// 探测成功 → CLOSED + recovered_ms（供爬坡因子）
	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 7, Model: "m", Success: true, LatencyMs: 100})
	rt = s.ReadRuntime(ctx, 7, "m")
	assert.Equal(t, dispatch.BreakerClosed, rt.Breaker)
	assert.Greater(t, rt.RecoveredMs, int64(0), "恢复时间必须记录")
}

func Test熔断探测失败冷却翻倍(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	// 致命错误一次直达熔断
	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 8, Model: "m", Success: false, Class: dispatch.ErrClassChannelFatal})
	rt := s.ReadRuntime(ctx, 8, "m")
	assert.Equal(t, dispatch.BreakerOpen, rt.Breaker)

	// 拨回冷却期满 → 探测失败 → 冷却翻倍
	_, _ = g.Redis().Do(ctx, "HSET", keyBreaker+"8", "opened_ms", time.Now().UnixMilli()-31_000)
	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 8, Model: "m", Success: false, Class: dispatch.ErrClassTransient})

	v, err := g.Redis().Do(ctx, "HMGET", keyBreaker+"8", "state", "cooldown_ms")
	require.NoError(t, err)
	vals := v.Vars()
	assert.Equal(t, 1, vals[0].Int(), "探测失败回 OPEN")
	assert.Equal(t, int64(60_000), vals[1].Int64(), "冷却翻倍 30s→60s")
}

func Test绑定CAS与反向索引(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	s.SetBinding(ctx, "sk:test:aaa", 1, time.Hour)
	id, ok := s.GetBinding(ctx, "sk:test:aaa")
	require.True(t, ok)
	assert.Equal(t, int64(1), id)

	// 改绑：旧反向索引清除、新反向索引写入
	s.SetBinding(ctx, "sk:test:aaa", 2, time.Hour)
	v, _ := g.Redis().Do(ctx, "SISMEMBER", keyBindRev+"1", keyBind+"sk:test:aaa")
	assert.Equal(t, 0, v.Int(), "旧渠道反向索引必须清除")
	v, _ = g.Redis().Do(ctx, "SISMEMBER", keyBindRev+"2", keyBind+"sk:test:aaa")
	assert.Equal(t, 1, v.Int())

	// 并发改绑竞争：最终绑定值与反向索引一致收敛
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.SetBinding(ctx, "sk:test:race", int64(n%5+1), time.Hour)
		}(i)
	}
	wg.Wait()
	final, ok := s.GetBinding(ctx, "sk:test:race")
	require.True(t, ok)
	for ch := int64(1); ch <= 5; ch++ {
		v, _ := g.Redis().Do(ctx, "SISMEMBER", keyBindRev+strconv.FormatInt(ch, 10), keyBind+"sk:test:race")
		if ch == final {
			assert.Equal(t, 1, v.Int(), "最终渠道 %d 的反向索引必须包含绑定", ch)
		} else {
			assert.Equal(t, 0, v.Int(), "非最终渠道 %d 的反向索引必须不含绑定", ch)
		}
	}

	// 批量失效：渠道下线清理全部绑定
	s.InvalidateChannelBindings(ctx, final)
	_, ok = s.GetBinding(ctx, "sk:test:race")
	assert.False(t, ok)
}

func Test绑定续期(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	s.SetBinding(ctx, "sk:test:touch", 1, 10*time.Second)
	s.TouchBinding(ctx, "sk:test:touch", time.Hour)
	ttl := mr.TTL(keyBind + "sk:test:touch")
	assert.Greater(t, ttl, 30*time.Minute, "续期后 TTL 必须延长")

	// 不存在的绑定续期为 no-op
	s.TouchBinding(ctx, "sk:test:none", time.Hour)
}

func Test容量租约(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	// 硬上限 1：并发争抢只允许一个成功
	okCount := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if s.AcquireLease(ctx, 10, 1, fmt.Sprintf("req-%d", n)) {
				mu.Lock()
				okCount++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	assert.Equal(t, 1, okCount, "上限 1 时并发争抢只能一个成功")

	// 释放后可重新获取
	// 找出持有者（10 个中成功的那个）——直接清空重测简单可靠
	mr.FlushAll()
	require.True(t, s.AcquireLease(ctx, 10, 1, "req-a"))
	require.False(t, s.AcquireLease(ctx, 10, 1, "req-b"))
	s.ReleaseLease(ctx, 10, "req-a")
	require.True(t, s.AcquireLease(ctx, 10, 1, "req-b"))

	// 无上限（softLimit=0）：全部放行，仅记录 inflight
	mr.FlushAll()
	for i := range 20 {
		require.True(t, s.AcquireLease(ctx, 11, 0, fmt.Sprintf("r-%d", i)))
	}
	rt := s.ReadRuntime(ctx, 11, "m")
	assert.Equal(t, 20, rt.Inflight)
}

func Test租约过期自愈(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	// 预置一个已过期的租约（score 在过去）——实例崩溃场景
	_, err := g.Redis().Do(ctx, "ZADD", keyInflight+"12", time.Now().UnixMilli()-1000, "dead-request")
	require.NoError(t, err)

	// 获取时原子清理过期租约，容量恢复
	assert.True(t, s.AcquireLease(ctx, 12, 1, "req-new"), "过期租约必须被清理，容量自愈")

	// 续租不能复活已释放的租约
	s.ReleaseLease(ctx, 12, "req-new")
	s.RefreshLease(ctx, 12, "req-new")
	v, _ := g.Redis().Do(ctx, "ZSCORE", keyInflight+"12", "req-new")
	assert.True(t, v.IsNil(), "已释放的租约不得被续租复活")
}

func Test凭证冷却(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	assert.False(t, s.IsCredentialCooled(ctx, 101))
	s.CoolCredential(ctx, 101, 2*time.Second)
	assert.True(t, s.IsCredentialCooled(ctx, 101))

	// TTL 到期自动恢复
	mr.FastForward(3 * time.Second)
	assert.False(t, s.IsCredentialCooled(ctx, 101), "冷却到期后必须恢复可用")
}

func TestReportOutcome异步消费(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)
	s.Start(ctx)
	defer s.Stop()

	s.ReportOutcome(dispatch.Outcome{ChannelID: 20, Model: "m", Success: false, Class: dispatch.ErrClassTransient})

	require.Eventually(t, func() bool {
		rt := s.ReadRuntime(ctx, 20, "m")
		return rt.SuccEwma < 1.0
	}, 3*time.Second, 20*time.Millisecond, "后台 worker 必须消费上报事件")
}

// ---------------------------------------------------------------------------
// 本地降级（修订 R4 + 基线方案 §13）
// ---------------------------------------------------------------------------

func TestLocalFallbackLimit(t *testing.T) {
	assert.Equal(t, 5, localFallbackLimit(10, 2))
	assert.Equal(t, 1, localFallbackLimit(3, 8), "至少为 1")
	assert.Equal(t, 0, localFallbackLimit(0, 2), "未配置上限的严格渠道降级期间拒绝")
	assert.Equal(t, 10, localFallbackLimit(10, 0), "副本数缺省按 1")
}

func TestLocal严格容量租约(t *testing.T) {
	l := newLocalState()
	require.True(t, l.acquireLease(1, "a", 2, 60_000))
	require.True(t, l.acquireLease(1, "b", 2, 60_000))
	require.False(t, l.acquireLease(1, "c", 2, 60_000), "实例级保守限额必须生效")

	l.releaseLease(1, "a")
	require.True(t, l.acquireLease(1, "c", 2, 60_000))

	// limit 0（未配置上限的严格渠道）直接拒绝
	require.False(t, l.acquireLease(2, "x", 0, 60_000))
}

func TestLocal探测限流(t *testing.T) {
	l := newLocalState()
	assert.True(t, l.tryProbe(1, 10_000))
	assert.False(t, l.tryProbe(1, 10_000), "同窗口本实例只放行一个")
	assert.True(t, l.tryProbe(2, 10_000), "不同渠道互不影响")
}

func TestLocal凭证冷却过期(t *testing.T) {
	l := newLocalState()
	l.coolCred(9, 50*time.Millisecond)
	assert.True(t, l.isCredCooled(9))
	time.Sleep(80 * time.Millisecond)
	assert.False(t, l.isCredCooled(9))
}
