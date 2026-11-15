//go:build e2e

// 缓存失效 e2e 回归（需真实 Redis；默认 go test 不编译本文件）。
// 跑法：TEAMAPI_TEST_REDIS_ADDR=host:port go test -tags e2e -run 'TestDeleteByPattern|TestCacheInvalidatePattern' ./internal/logic/common/
//
// 注意：本文件 blank import redis adapter 会改变同二进制内其他依赖「adapter 未注册即 skip」
// 测试的行为（如 register_rate_limit/security_2fa_fail 从 skip 变为真实执行），故必须用
// e2e 标签隔离，不进常规测试二进制。

package common

import (
	"context"
	"os"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
)

// DeleteByPattern 跨 L1/L2 双层失效回归测试（需真实 Redis：设置 TEAMAPI_TEST_REDIS_ADDR 后运行）。
// 背景：GetJSON/Get 命中 L2 会以全新 TTL 回填 L1，使其过期晚于 L2；进程重启、Redis 写失败
// 也会产生「只在 L1、不在 L2」的键。DeleteByPattern 此前只按 SCAN 到的 L2 键清 L1，
// 这类键漏清——租户调级/模型改价后旧折扣（旧价格）会在 L1 TTL 内继续生效。
func TestDeleteByPatternCoversL1OnlyKeys(t *testing.T) {
	addr := os.Getenv("TEAMAPI_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("需要真实 Redis：设置 TEAMAPI_TEST_REDIS_ADDR（如 192.168.50.22:16380）后运行")
	}
	gredis.SetConfig(&gredis.Config{Address: addr, Db: 0}, "default")

	ctx := context.Background()
	c := NewCache("repro_l1", time.Minute)
	isolated := NewCache("repro_isolated", time.Minute)

	// 预清理，保证断言幂等可重放
	for _, k := range []string{"repro_l1:9:both", "repro_l1:9:l1only", "repro_l1:7:gpt-4", "repro_isolated:9:l1only"} {
		gcache.Remove(ctx, k)
		_, _ = g.Redis().Do(ctx, "DEL", k)
	}

	// 场景1：L1+L2 双层键 → 双层清除（既有行为不回归）
	c.Set(ctx, "9:both", "stale", time.Minute)
	// 场景2：L1-only 键（模拟 L2 先过期/丢失）→ L1 必须清除，本修复的核心断言
	c.Set(ctx, "9:l1only", "stale-0.85", time.Minute)
	if _, err := g.Redis().Do(ctx, "DEL", "repro_l1:9:l1only"); err != nil {
		t.Fatalf("模拟 L2 丢失失败: %v", err)
	}
	// 场景3：其他 prefix 的同名键，验证前缀隔离不被误删
	isolated.Set(ctx, "9:l1only", "keep", time.Minute)

	c.DeleteByPattern(ctx, "9:*")

	if v, _ := gcache.Get(ctx, "repro_l1:9:both"); v != nil {
		t.Errorf("L1+L2 键的 L1 未清除: %v", v)
	}
	if rv, err := g.Redis().Do(ctx, "GET", "repro_l1:9:both"); err == nil && rv != nil && !rv.IsNil() {
		t.Errorf("L1+L2 键的 L2 未清除: %v", rv)
	}
	if v, _ := gcache.Get(ctx, "repro_l1:9:l1only"); v != nil {
		t.Errorf("L1-only 键未清除（核心场景）：调级/改价失效后旧折扣会在 L1 TTL 内继续生效: %v", v)
	}
	if v, _ := gcache.Get(ctx, "repro_isolated:9:l1only"); v == nil {
		t.Error("前缀隔离失败：其他 prefix 的同名键被误删")
	}

	// 场景4：后缀通配（ClearModelPriceCache 的 *:{modelName} 形态）对 L1-only 键同样生效
	c.Set(ctx, "7:gpt-4", "stale", time.Minute)
	_, _ = g.Redis().Do(ctx, "DEL", "repro_l1:7:gpt-4")
	c.DeleteByPattern(ctx, "*:gpt-4")
	if v, _ := gcache.Get(ctx, "repro_l1:7:gpt-4"); v != nil {
		t.Errorf("后缀通配模式的 L1-only 键未清除: %v", v)
	}
}

// TestCacheInvalidatePatternChannelClearsLocalL1 模式频道跨实例失效测试：
// 其他实例发起 DeleteByPattern 时，本实例的 L1-only 键（对发布方 SCAN 不可见）
// 依赖 cache:invalidate:pattern 频道广播由订阅方本地全量匹配清除。
func TestCacheInvalidatePatternChannelClearsLocalL1(t *testing.T) {
	addr := os.Getenv("TEAMAPI_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("需要真实 Redis：设置 TEAMAPI_TEST_REDIS_ADDR（如 192.168.50.22:16380）后运行")
	}
	gredis.SetConfig(&gredis.Config{Address: addr, Db: 0}, "default")

	ctx := context.Background()

	// 直接构造 L1-only 键（模拟经 L2 回填续命后 L2 先过期，发布方 SCAN 不可见）
	gcache.Set(ctx, "repro_l1:5:remote", "stale", time.Minute)

	StartCacheInvalidationSubscriber(ctx)

	// 订阅建立与消息投递有延迟，周期性重发广播直至生效或超时
	deadline := time.Now().Add(5 * time.Second)
	for {
		if v, _ := gcache.Get(ctx, "repro_l1:5:remote"); v == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("模式频道广播后 5s 内本地 L1 键未被清除")
		}
		// 模拟另一实例发起的模式失效广播
		_, _ = g.Redis().Do(ctx, "PUBLISH", "cache:invalidate:pattern", "repro_l1:5:*")
		time.Sleep(100 * time.Millisecond)
	}
}
