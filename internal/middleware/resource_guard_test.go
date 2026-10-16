package middleware

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/gclient"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/test/gtest"
)

// resetResourceGuardCaches 重置包级采样缓存与告警节流时间戳，
// 避免用例间的 TTL 残留互相干扰（缓存值/负缓存状态都会跨用例存活）
func resetResourceGuardCaches() {
	memPercentCache.mu.Lock()
	memPercentCache.val, memPercentCache.ok, memPercentCache.at = 0, false, time.Time{}
	memPercentCache.mu.Unlock()
	diskPercentCache.mu.Lock()
	diskPercentCache.val, diskPercentCache.ok, diskPercentCache.at = 0, false, time.Time{}
	diskPercentCache.mu.Unlock()
	lastGuardLogUnix = 0
}

// injectResourceGuardDeps 用替身替换三个读取函数变量并重置缓存，
// 用例结束自动恢复生产实现（仿 body_limit_test.go 的注入模式）
func injectResourceGuardDeps(t *gtest.T,
	limits func(context.Context) (int, int),
	memRead func() (float64, error),
	diskRead func() (float64, error),
) {
	origMem, origDisk, origLimits := readMemPercent, readDiskPercent, getGuardLimits
	readMemPercent, readDiskPercent, getGuardLimits = memRead, diskRead, limits
	resetResourceGuardCaches()
	t.Cleanup(func() {
		readMemPercent, readDiskPercent, getGuardLimits = origMem, origDisk, origLimits
		resetResourceGuardCaches()
	})
}

// startResourceGuardServer 起一个只挂 ResourceGuard 中间件的测试服务，
// 真实生效路径（阈值判定 + 三协议原生格式错误响应）与生产完全一致
func startResourceGuardServer(t *gtest.T, name string) (*ghttp.Server, *gclient.Client) {
	s := g.Server(name)
	s.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(ResourceGuard)
		group.POST("/echo", func(r *ghttp.Request) {
			r.Response.Write("ok")
		})
		group.POST("/v1/messages", func(r *ghttp.Request) {
			r.Response.Write("ok")
		})
		group.POST("/v1beta/echo", func(r *ghttp.Request) {
			r.Response.Write("ok")
		})
	})
	s.SetAddr("127.0.0.1:0")
	s.SetDumpRouterMap(false)
	s.Start()

	client := g.Client()
	client.SetPrefix(fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort()))
	return s, client
}

// TestResourceGuard_DisabledPassThrough 双阈值为 0（显式关闭）时必须零拦截放行
func TestResourceGuard_DisabledPassThrough(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		injectResourceGuardDeps(t,
			func(context.Context) (int, int) { return 0, 0 },
			func() (float64, error) { return 99.9, nil },
			func() (float64, error) { return 99.9, nil })
		s, client := startResourceGuardServer(t, "resource-guard-disabled")
		defer s.Shutdown()

		resp, err := client.Post(context.Background(), "/echo", "{}")
		t.AssertNil(err)
		defer resp.Close()

		t.Assert(resp.StatusCode, 200)
		t.Assert(resp.ReadAllString(), "ok")
	})
}

// TestResourceGuard_MemoryRejectOpenAIFormat 内存超限 → 503 + Retry-After + OpenAI 格式
func TestResourceGuard_MemoryRejectOpenAIFormat(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		injectResourceGuardDeps(t,
			func(context.Context) (int, int) { return 90, 0 },
			func() (float64, error) { return 95.5, nil },
			func() (float64, error) { return 0, errors.New("should not read disk") })
		s, client := startResourceGuardServer(t, "resource-guard-openai")
		defer s.Shutdown()

		resp, err := client.Post(context.Background(), "/echo", "{}")
		t.AssertNil(err)
		defer resp.Close()

		t.Assert(resp.StatusCode, 503)
		t.Assert(resp.Header.Get("Retry-After"), "10")
		t.Assert(resp.Header.Get("x-should-retry"), "")
		body := resp.ReadAllString()
		t.Assert(strings.Contains(body, `"server_error"`), true)
		t.Assert(strings.Contains(body, "服务器内存占用过高"), true)
	})
}

// TestResourceGuard_MemoryRejectClaudeFormat /v1/messages 内存超限 → Claude 格式
// （overloaded_error + x-should-retry）
func TestResourceGuard_MemoryRejectClaudeFormat(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		injectResourceGuardDeps(t,
			func(context.Context) (int, int) { return 90, 0 },
			func() (float64, error) { return 95.5, nil },
			func() (float64, error) { return 0, errors.New("should not read disk") })
		s, client := startResourceGuardServer(t, "resource-guard-claude")
		defer s.Shutdown()

		resp, err := client.Post(context.Background(), "/v1/messages", "{}")
		t.AssertNil(err)
		defer resp.Close()

		t.Assert(resp.StatusCode, 503)
		t.Assert(resp.Header.Get("x-should-retry"), "true")
		body := resp.ReadAllString()
		t.Assert(strings.Contains(body, `"overloaded_error"`), true)
		t.Assert(strings.Contains(body, `"type":"error"`), true)
	})
}

// TestResourceGuard_DiskRejectGeminiFormat /v1beta 磁盘超限 → Gemini 格式
// （503 + UNAVAILABLE）
func TestResourceGuard_DiskRejectGeminiFormat(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		injectResourceGuardDeps(t,
			func(context.Context) (int, int) { return 0, 90 },
			func() (float64, error) { return 0, errors.New("should not read mem") },
			func() (float64, error) { return 96.0, nil })
		s, client := startResourceGuardServer(t, "resource-guard-gemini")
		defer s.Shutdown()

		resp, err := client.Post(context.Background(), "/v1beta/echo", "{}")
		t.AssertNil(err)
		defer resp.Close()

		t.Assert(resp.StatusCode, 503)
		body := resp.ReadAllString()
		t.Assert(strings.Contains(body, `"UNAVAILABLE"`), true)
		t.Assert(strings.Contains(body, `"code":503`), true)
		t.Assert(strings.Contains(body, "服务器磁盘占用过高"), true)
	})
}

// TestResourceGuard_BoundaryEqualPasses 采样值恰好等于阈值时放行（严格大于才拒绝）
func TestResourceGuard_BoundaryEqualPasses(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		injectResourceGuardDeps(t,
			func(context.Context) (int, int) { return 90, 0 },
			func() (float64, error) { return 90, nil },
			func() (float64, error) { return 0, errors.New("should not read disk") })
		s, client := startResourceGuardServer(t, "resource-guard-boundary")
		defer s.Shutdown()

		resp, err := client.Post(context.Background(), "/echo", "{}")
		t.AssertNil(err)
		defer resp.Close()

		t.Assert(resp.StatusCode, 200)
	})
}

// TestResourceGuard_ReadErrorFailOpen 指标读取失败必须放行（fail-open），
// 不能因 /proc 等故障源把网关打入拒绝态
func TestResourceGuard_ReadErrorFailOpen(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		injectResourceGuardDeps(t,
			func(context.Context) (int, int) { return 90, 90 },
			func() (float64, error) { return 0, errors.New("mem read failed") },
			func() (float64, error) { return 0, errors.New("disk read failed") })
		s, client := startResourceGuardServer(t, "resource-guard-failopen")
		defer s.Shutdown()

		resp, err := client.Post(context.Background(), "/echo", "{}")
		t.AssertNil(err)
		defer resp.Close()

		t.Assert(resp.StatusCode, 200)
	})
}

// TestMetricCache_TTLAndConcurrentRefresh TTL 窗口内并发 lookup 只触发一次真实
// 读取（mutex 内联刷新即 single-flight）；过期后再查触发第二次
func TestMetricCache_TTLAndConcurrentRefresh(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		var calls int
		read := func() (float64, error) {
			calls++
			return 88.8, nil
		}
		ttl := time.Minute
		var c metricCache

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				val, ok := c.lookup(ttl, read)
				if !ok || val != 88.8 {
					t.Errorf("lookup 返回异常: val=%v ok=%v", val, ok)
				}
			}()
		}
		wg.Wait()
		t.Assert(calls, 1)

		// 模拟过期：把采样时间拨回 TTL 之前
		c.mu.Lock()
		c.at = time.Now().Add(-2 * ttl)
		c.mu.Unlock()

		_, ok := c.lookup(ttl, read)
		t.Assert(ok, true)
		t.Assert(calls, 2)
	})
}

// TestMetricCache_NegativeCache 读失败写负缓存：TTL 窗口内后续 lookup 不再
// 轰炸故障源，且返回不可用（ok=false）
func TestMetricCache_NegativeCache(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		var calls int
		read := func() (float64, error) {
			calls++
			return 0, errors.New("read failed")
		}
		var c metricCache

		_, ok := c.lookup(time.Minute, read)
		t.Assert(ok, false)
		t.Assert(calls, 1)

		_, ok = c.lookup(time.Minute, read)
		t.Assert(ok, false)
		t.Assert(calls, 1) // 负缓存生效，未再次调用 read
	})
}
