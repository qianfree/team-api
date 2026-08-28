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
		p.Breaker.FailThreshold = 1000      // 阈值调高，只验证计数不触发熔断
		p.Breaker.ModelFailThreshold = 1000 // 模型级阈值同步调高，避免打开后渠道级停止计数
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
		{5, dispatch.ErrClassModelFatal, 0.70},
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

// Test探测失败不衰减健康EWMA 探测类（Probe）失败只喂熔断窗口，健康分不变——
// 防止每 5 分钟的自动探测持续失败把无流量渠道健康分指数拖垮（0.93^N → 个位数）；
// 探测成功照常回升健康并记录延迟。
func Test探测失败不衰减健康EWMA(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, func(p *dispatch.RoutingPolicy) {
		p.Breaker.FailThreshold = 1000 // 阈值调高，只验证窗口计数不触发熔断
		p.Breaker.ModelFailThreshold = 1000
	})

	// 连续探测失败：健康 EWMA 保持满分，熔断窗口照常计数
	for range 5 {
		s.processOutcome(ctx, dispatch.Outcome{
			ChannelID: 9, Model: "m", Success: false,
			Class: dispatch.ErrClassTransient, Probe: true,
		})
	}
	rt := s.ReadRuntime(ctx, 9, "m")
	assert.InEpsilon(t, 1.0, rt.SuccEwma, 1e-9, "探测失败不得衰减健康分")

	v, err := g.Redis().Do(ctx, "HGET", keyBreaker+"9", "fail_count")
	require.NoError(t, err)
	assert.Equal(t, 5, v.Int(), "探测失败仍喂熔断窗口")

	// 真实流量失败拉低健康分后，探测成功照常回升
	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 9, Model: "m", Success: false, Class: dispatch.ErrClassTransient})
	rt = s.ReadRuntime(ctx, 9, "m")
	require.InEpsilon(t, 0.93, rt.SuccEwma, 1e-9)

	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 9, Model: "m", Success: true, LatencyMs: 500, Probe: true})
	rt = s.ReadRuntime(ctx, 9, "m")
	assert.InEpsilon(t, 0.93*0.9+0.1, rt.SuccEwma, 1e-9, "探测成功照常回升健康分")
	assert.InEpsilon(t, 500, rt.LatEwmaMs, 1e-9)
}

// Test缺失健康键读默认值 gf 驱动对 HMGET 缺失字段返回空字符串而非 nil（IsNil()==false）。
// ReadRuntime 必须按无数据处理保留乐观默认（succ=1）：无流量模型或 TTL（24h）过期键
// 若读成 succ=0，健康快照取平均会把渠道健康度拖到个位数。
func Test缺失健康键读默认值(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	rt := s.ReadRuntime(ctx, 999, "no-traffic-model")
	assert.InEpsilon(t, 1.0, rt.SuccEwma, 1e-9, "无健康数据必须默认满分")
	assert.Zero(t, rt.LatEwmaMs)
	assert.Equal(t, dispatch.BreakerClosed, rt.Breaker)
	assert.Equal(t, dispatch.BreakerClosed, rt.ModelBreaker)
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
	s := newTestState(t, func(p *dispatch.RoutingPolicy) {
		p.Breaker.ModelFailThreshold = 100 // 调高模型级阈值，聚焦验证渠道级 8 次语义
	})

	// 8 次瞬时失败 → 渠道级熔断 OPEN
	for range 8 {
		s.processOutcome(ctx, dispatch.Outcome{ChannelID: 7, Model: "m", Success: false, Class: dispatch.ErrClassTransient})
	}
	rt := s.ReadRuntime(ctx, 7, "m")
	assert.Equal(t, dispatch.BreakerOpen, rt.Breaker)

	// 冷却期内探测令牌不放行
	assert.False(t, s.TryProbeToken(ctx, 7, ""))

	// 手动把 opened_ms 拨回 31s 前（冷却 30s）→ 有效态 HALF_OPEN
	_, err := g.Redis().Do(ctx, "HSET", keyBreaker+"7", "opened_ms", time.Now().UnixMilli()-31_000)
	require.NoError(t, err)
	rt = s.ReadRuntime(ctx, 7, "m")
	assert.Equal(t, dispatch.BreakerHalfOpen, rt.Breaker)

	// 探测令牌：每窗口全局只放行一个
	assert.True(t, s.TryProbeToken(ctx, 7, ""))
	assert.False(t, s.TryProbeToken(ctx, 7, ""), "同窗口第二个探测必须拒绝")

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

// ---------------------------------------------------------------------------
// 模型级隔离（错误作用域分流 + 去重守卫）
// ---------------------------------------------------------------------------

// Test模型致命一次直达模型级 404/模型不存在：模型级 fatal 直达 OPEN，
// 渠道级仅窗口计 1 票；模型级已 OPEN 后同模型后续失败不再污染渠道级。
func Test模型致命一次直达模型级(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 62, Model: "gone", Success: false, Class: dispatch.ErrClassModelFatal})
	rt := s.ReadRuntime(ctx, 62, "gone")
	assert.Equal(t, dispatch.BreakerOpen, rt.ModelBreaker, "模型级 fatal 一次直达")
	assert.Equal(t, dispatch.BreakerClosed, rt.Breaker, "渠道级不被拉黑")

	v, err := g.Redis().Do(ctx, "HGET", keyBreaker+"62", "fail_count")
	require.NoError(t, err)
	assert.Equal(t, 1, v.Int(), "渠道级仅窗口计 1 票")

	// 去重守卫：模型级 OPEN（含有效 HALF_OPEN）后，本模型失败不再喂渠道级
	for range 5 {
		s.processOutcome(ctx, dispatch.Outcome{ChannelID: 62, Model: "gone", Success: false, Class: dispatch.ErrClassModelFatal})
	}
	v, _ = g.Redis().Do(ctx, "HGET", keyBreaker+"62", "fail_count")
	assert.Equal(t, 1, v.Int(), "模型 OPEN 后渠道级不再计数")
}

// Test模型级窗口阈值与渠道级隔离 单模型慢性故障（瞬时类）：模型级阈值 4 先打开，
// 渠道级贡献封顶在 4 票（< 8），渠道级永不熔断——单模型故障不误伤其它模型。
func Test模型级窗口阈值与渠道级隔离(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil) // failThreshold=8, modelFailThreshold=4

	for range 3 {
		s.processOutcome(ctx, dispatch.Outcome{ChannelID: 60, Model: "m", Success: false, Class: dispatch.ErrClassTransient})
	}
	v, err := g.Redis().Do(ctx, "HGET", keyBreaker+"60", "fail_count")
	require.NoError(t, err)
	assert.Equal(t, 3, v.Int(), "渠道级窗口计数 3")
	v, _ = g.Redis().Do(ctx, "HGET", keyBreaker+"60:m", "fail_count")
	assert.Equal(t, 3, v.Int(), "模型级窗口计数 3")

	// 第 4 次触发模型级打开（先渠道计数后模型转移，本事件贡献第 4 票），此后不再计数
	for range 10 {
		s.processOutcome(ctx, dispatch.Outcome{ChannelID: 60, Model: "m", Success: false, Class: dispatch.ErrClassTransient})
	}
	rt := s.ReadRuntime(ctx, 60, "m")
	assert.Equal(t, dispatch.BreakerOpen, rt.ModelBreaker, "模型级达阈值 4 打开")
	assert.Equal(t, dispatch.BreakerClosed, rt.Breaker, "渠道级停在 4 票不熔断")
	v, _ = g.Redis().Do(ctx, "HGET", keyBreaker+"60", "fail_count")
	assert.Equal(t, 4, v.Int(), "单模型对渠道级贡献封顶在 modelFailThreshold")
}

// Test跨模型共识渠道级熔断 两个模型各恶化 4 次（4+4=8）→ 渠道级 OPEN：
// 渠道级熔断保留给真正的多模型同时恶化（整渠道故障）。
func Test跨模型共识渠道级熔断(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	for _, m := range []string{"m1", "m2"} {
		for range 4 {
			s.processOutcome(ctx, dispatch.Outcome{ChannelID: 61, Model: m, Success: false, Class: dispatch.ErrClassTransient})
		}
	}
	rt := s.ReadRuntime(ctx, 61, "m1")
	assert.Equal(t, dispatch.BreakerOpen, rt.Breaker, "跨模型 4+4=8 达成共识")
	assert.Equal(t, dispatch.BreakerOpen, rt.ModelBreaker, "两模型各自也达阈值 4")
}

// Test渠道致命不喂模型级 402/Key 耗尽等渠道级信号：fatal 只打渠道级，模型级 key 不被触碰
// （渠道恢复后模型无残留熔断）。
func Test渠道致命不喂模型级(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 63, Model: "m", Success: false, Class: dispatch.ErrClassChannelFatal})
	rt := s.ReadRuntime(ctx, 63, "m")
	assert.Equal(t, dispatch.BreakerOpen, rt.Breaker)

	v, err := g.Redis().Do(ctx, "EXISTS", keyBreaker+"63:m")
	require.NoError(t, err)
	assert.Equal(t, 0, v.Int(), "渠道级致命不创建/触碰模型级 key")
}

// Test模型级探测成功复位 模型级 OPEN 冷却期满 → HALF_OPEN → 成功 → CLOSED + recovered_ms
// （模型级熔断可靠流量探测恢复，不再死锁到 24h TTL）。
func Test模型级探测成功复位(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 64, Model: "m", Success: false, Class: dispatch.ErrClassModelFatal})
	_, _ = g.Redis().Do(ctx, "HSET", keyBreaker+"64:m", "opened_ms", time.Now().UnixMilli()-31_000)

	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 64, Model: "m", Success: true, LatencyMs: 100})
	rt := s.ReadRuntime(ctx, 64, "m")
	assert.Equal(t, dispatch.BreakerClosed, rt.ModelBreaker)
	v, _ := g.Redis().Do(ctx, "HGET", keyBreaker+"64:m", "recovered_ms")
	assert.Greater(t, v.Int64(), int64(0), "模型级恢复时间必须记录")
}

// Test模型级探测令牌与探测失败翻倍 模型级令牌按渠道×模型 key 独立限流；
// 冷却期满后的失败走 HALF_OPEN 探测失败分支（模型级回 OPEN 冷却翻倍，渠道级不计数）。
func Test模型级探测令牌与探测失败翻倍(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 65, Model: "m", Success: false, Class: dispatch.ErrClassModelFatal})
	_, _ = g.Redis().Do(ctx, "HSET", keyBreaker+"65:m", "opened_ms", time.Now().UnixMilli()-31_000)

	// 模型级令牌：每窗口放行一个；渠道级 CLOSED 时渠道令牌不放行（不再误拒探测）
	assert.True(t, s.TryProbeToken(ctx, 65, "m"))
	assert.False(t, s.TryProbeToken(ctx, 65, "m"), "同窗口第二个模型级探测必须拒绝")
	assert.False(t, s.TryProbeToken(ctx, 65, ""), "渠道级 CLOSED 不放行渠道令牌")

	// 探测失败：模型级回 OPEN 冷却翻倍，渠道级不新增计数（保持首次 ModelFatal 贡献的 1 票）
	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 65, Model: "m", Success: false, Class: dispatch.ErrClassTransient})
	v, err := g.Redis().Do(ctx, "HMGET", keyBreaker+"65:m", "state", "cooldown_ms")
	require.NoError(t, err)
	vals := v.Vars()
	assert.Equal(t, 1, vals[0].Int(), "模型级探测失败回 OPEN")
	assert.Equal(t, int64(60_000), vals[1].Int64(), "冷却翻倍 30s→60s")
	v, _ = g.Redis().Do(ctx, "HGET", keyBreaker+"65", "fail_count")
	assert.Equal(t, 1, v.Int(), "渠道级不因探测失败新增计数")
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

// Test凭证冷却可手动解除 覆盖「更换 Key / 重置健康度」的解除路径：冷却按 keyID 打标，
// 而换 Key 是原地改同一行（keyID 不变），不解除的话新 Key 会被整段跳过直到 TTL 到期。
func Test凭证冷却可手动解除(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	s.CoolCredential(ctx, 202, 5*time.Minute)
	require.True(t, s.IsCredentialCooled(ctx, 202))
	require.Positive(t, CredentialCooldownRemaining(ctx, 202), "冷却中必须能读到剩余秒数")

	s.ClearCredentialCooldown(ctx, 202)
	assert.False(t, s.IsCredentialCooled(ctx, 202), "解除后必须立即可用，不等 TTL")
	assert.Zero(t, CredentialCooldownRemaining(ctx, 202))
	assert.False(t, s.local.isCredCooled(202), "本地镜像必须一并清除，否则 Redis 降级时仍会跳过该 Key")

	// 解除未冷却的 Key 是幂等空操作
	s.ClearCredentialCooldown(ctx, 999)
	assert.False(t, s.IsCredentialCooled(ctx, 999))
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

// TestReadRuntime无数据回落默认值 从未被访问过的渠道×模型（Redis key 不存在）必须回落
// 默认满分 1.0。HMGET 缺失字段经 gredis 包装成非 nil 空 gvar（IsNil() 恒为 false、
// Float64() 静默返回 0），旧代码用 IsNil() 判空失效，把全部冷模型读成 0 分，
// 拖垮渠道聚合健康分与调度 healthFactor。
func TestReadRuntime无数据回落默认值(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	rt := s.ReadRuntime(ctx, 99, "never-touched")
	assert.Equal(t, 1.0, rt.SuccEwma, "key 不存在必须回落默认满分，不能是 0")
	assert.Equal(t, 0.0, rt.LatEwmaMs, "无延迟数据为 0")
	assert.Equal(t, dispatch.BreakerClosed, rt.Breaker)

	// 只有 lat_ewma、无 succ_ewma 字段时，succ 同样回落默认值
	_, err := g.Redis().Do(ctx, "HSET", keyHealth+"99:partial", "lat_ewma", "100")
	require.NoError(t, err)
	rt = s.ReadRuntime(ctx, 99, "partial")
	assert.Equal(t, 1.0, rt.SuccEwma, "字段缺失必须回落默认满分")
	assert.Equal(t, 100.0, rt.LatEwmaMs)
}

// TestReadRuntime异常值防御 succ_ewma 为空串/非法值/精确 0 时视为无数据（保持默认 1.0）。
func TestReadRuntime异常值防御(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	for _, bad := range []string{"", "  ", "abc", "0"} {
		_, err := g.Redis().Do(ctx, "HSET", keyHealth+"30:dirty", "succ_ewma", bad, "lat_ewma", "100")
		require.NoError(t, err)
		rt := s.ReadRuntime(ctx, 30, "dirty")
		assert.Equal(t, 1.0, rt.SuccEwma, "succ_ewma=%q 应视为无数据", bad)
	}

	// 真实低分（正数）不受影响
	_, err := g.Redis().Do(ctx, "HSET", keyHealth+"30:low", "succ_ewma", "0.0008", "lat_ewma", "100")
	require.NoError(t, err)
	rt := s.ReadRuntime(ctx, 30, "low")
	assert.Equal(t, 0.0008, rt.SuccEwma, "合法低分必须如实读取")

	// lat_ewma 同样防御：脏值回落 0
	_, err = g.Redis().Do(ctx, "HSET", keyHealth+"30:dirtylat", "succ_ewma", "0.5", "lat_ewma", "xyz")
	require.NoError(t, err)
	rt = s.ReadRuntime(ctx, 30, "dirtylat")
	assert.Equal(t, 0.5, rt.SuccEwma)
	assert.Equal(t, 0.0, rt.LatEwmaMs)
}

// TestReadRuntime区分无数据与读失败 「没读到」与「读失败」必须可区分：前者是真实的
// 「该模型没有流量」，后者是 Redis 不可用。二者都会回落成默认满分，若调用方无法区分，
// Redis 抖动期间维护快照会把全渠道健康分刷成 100，事故现场被销毁。
func TestReadRuntime区分无数据与读失败(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	// 从未上报：读值是乐观默认满分，但必须标记为「无真实数据」
	rt := s.ReadRuntime(ctx, 77, "cold")
	assert.Equal(t, 1.0, rt.SuccEwma)
	assert.False(t, rt.HasHealth, "无上报的默认满分不得冒充实测值")
	assert.False(t, rt.Degraded)

	// 有真实上报
	_, err := g.Redis().Do(ctx, "HSET", keyHealth+"77:hot", "succ_ewma", "0.42", "lat_ewma", "900")
	require.NoError(t, err)
	rt = s.ReadRuntime(ctx, 77, "hot")
	assert.Equal(t, 0.42, rt.SuccEwma)
	assert.True(t, rt.HasHealth)
	assert.False(t, rt.Degraded)

	// Redis 故障：同样回落满分，但标记 Degraded
	mr.SetError("LOADING Redis is loading the dataset in memory")
	defer mr.SetError("")
	rt = s.ReadRuntime(ctx, 77, "hot")
	assert.True(t, rt.Degraded, "读失败必须显式标记")
	assert.False(t, rt.HasHealth, "读失败不是「有数据」")
	assert.Equal(t, 1.0, rt.SuccEwma)
}

// TestReadRuntime脏值不算有效数据 精确 0 / 非法值视为无数据，HasHealth 保持 false，
// 避免脏数据以「实测值」身份参与渠道聚合（2026-08-27 全渠道健康分坍缩事故的读侧防线）。
func TestReadRuntime脏值不算有效数据(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	for _, bad := range []string{"", "abc", "0"} {
		_, err := g.Redis().Do(ctx, "HSET", keyHealth+"78:dirty", "succ_ewma", bad, "lat_ewma", "100")
		require.NoError(t, err)
		rt := s.ReadRuntime(ctx, 78, "dirty")
		assert.False(t, rt.HasHealth, "succ_ewma=%q 不得算作有效实测数据", bad)
		assert.Equal(t, 1.0, rt.SuccEwma)
	}
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
	assert.True(t, l.tryProbe(1, "", 10_000))
	assert.False(t, l.tryProbe(1, "", 10_000), "同窗口本实例只放行一个")
	assert.True(t, l.tryProbe(2, "", 10_000), "不同渠道互不影响")
	assert.True(t, l.tryProbe(1, "gpt-4o", 10_000), "同渠道的模型级令牌与渠道级互不影响")
}

func TestLocal凭证冷却过期(t *testing.T) {
	l := newLocalState()
	l.coolCred(9, 50*time.Millisecond)
	assert.True(t, l.isCredCooled(9))
	time.Sleep(80 * time.Millisecond)
	assert.False(t, l.isCredCooled(9))
}

func TestLocal凭证冷却解除(t *testing.T) {
	l := newLocalState()
	l.coolCred(9, time.Hour)
	require.True(t, l.isCredCooled(9))
	l.clearCred(9)
	assert.False(t, l.isCredCooled(9), "解除后不等 TTL 立即可用")
	l.clearCred(9) // 幂等
}

func Test429估计器水位收敛(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	// 预置 30 个在途租约（未过期）
	for i := range 30 {
		require.True(t, s.AcquireLease(ctx, 40, 0, fmt.Sprintf("req-%d", i)))
	}

	// 首次 429：onset 直接取当前水位
	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 40, Model: "m", Success: false, Class: dispatch.ErrClassRateLimit})
	rt := s.ReadRuntime(ctx, 40, "m")
	assert.InEpsilon(t, 30.0, rt.Onset429Ewma, 1e-9)

	// 释放到 10 个在途后再收 429：onset = 30×0.8 + 10×0.2 = 26
	for i := 10; i < 30; i++ {
		s.ReleaseLease(ctx, 40, fmt.Sprintf("req-%d", i))
	}
	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 40, Model: "m", Success: false, Class: dispatch.ErrClassRateLimit})
	rt = s.ReadRuntime(ctx, 40, "m")
	assert.InEpsilon(t, 26.0, rt.Onset429Ewma, 1e-9)
}

func TestEffectiveSoftLimit(t *testing.T) {
	assert.Equal(t, 50, effectiveSoftLimit(50, 30), "手动上限优先")
	assert.Equal(t, 0, effectiveSoftLimit(0, 0), "无信息视为无限容量")
	assert.Equal(t, 27, effectiveSoftLimit(0, 30), "floor(30×0.9)")
	assert.Equal(t, 4, effectiveSoftLimit(0, 2), "下限保护 max(4, ...)")
}

func Test手动恢复复位熔断并开启爬坡(t *testing.T) {
	ctx := context.Background()
	s := newTestState(t, nil)

	// 制造熔断 OPEN
	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 50, Model: "m", Success: false, Class: dispatch.ErrClassChannelFatal})
	rt := s.ReadRuntime(ctx, 50, "m")
	require.Equal(t, dispatch.BreakerOpen, rt.Breaker)
	require.Greater(t, rt.FirstOpenedMs, int64(0))

	// 手动恢复：CLOSED + recovered_ms（爬坡起点）+ 清故障期
	s.MarkChannelRecovered(ctx, 50)
	rt = s.ReadRuntime(ctx, 50, "m")
	assert.Equal(t, dispatch.BreakerClosed, rt.Breaker)
	assert.Greater(t, rt.RecoveredMs, int64(0))
	assert.Zero(t, rt.FirstOpenedMs)

	// 模型级熔断不受 MarkChannelRecovered 影响（只验证了 test_model，不能证明其它模型恢复；
	// 模型级靠流量探测自行恢复，决策见 MarkChannelRecovered 注释）
	s.processOutcome(ctx, dispatch.Outcome{ChannelID: 50, Model: "m2", Success: false, Class: dispatch.ErrClassModelFatal})
	rt = s.ReadRuntime(ctx, 50, "m2")
	assert.Equal(t, dispatch.BreakerOpen, rt.ModelBreaker, "模型级熔断必须保留")
	assert.Equal(t, dispatch.BreakerClosed, rt.Breaker, "渠道级保持复位后状态")
}
