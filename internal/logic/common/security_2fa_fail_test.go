package common

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

// require2FARedis 与 register_rate_limit_test.go 相同的 skip-if-no-Redis 模式：
// 未配置适配器（panic）或 Redis 不可用时跳过，不拖垮无 Redis 环境的常规测试。
func require2FARedis(t *testing.T, ctx context.Context) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Skipf("Redis adapter is not configured: %v", r)
		}
	}()
	if _, err := g.Redis().Do(ctx, "PING"); err != nil {
		t.Skipf("Redis is not available: %v", err)
	}
}

// Test2FAFailureCounter 2FA 验证失败计数：递增、达阈值 blocked、
// 成功清零、固定窗口 TTL 存在
func Test2FAFailureCounter(t *testing.T) {
	ctx := context.Background()
	require2FARedis(t, ctx)

	// 阈值压到 3，测试确定性（默认 5 次）
	Config().cache.Set(ctx, "login_max_attempts", "3")
	t.Cleanup(func() { Config().cache.Delete(ctx, "login_max_attempts") })

	const userType = "admin"
	const userID int64 = 999901
	key := twoFAFailKey(userType, userID)
	_, _ = g.Redis().Do(ctx, "DEL", key)
	t.Cleanup(func() { _, _ = g.Redis().Do(ctx, "DEL", key) })

	// 无失败记录时不应 blocked
	if Is2FABlocked(ctx, userType, userID) {
		t.Fatal("无失败记录时不应 blocked")
	}

	// 前两次失败：未达阈值
	if Register2FAFailure(ctx, userType, userID) {
		t.Fatal("第 1 次失败不应 blocked")
	}
	if Register2FAFailure(ctx, userType, userID) {
		t.Fatal("第 2 次失败不应 blocked")
	}
	if Is2FABlocked(ctx, userType, userID) {
		t.Fatal("未达阈值时不应 blocked")
	}

	// 第 3 次失败：达到阈值
	if !Register2FAFailure(ctx, userType, userID) {
		t.Fatal("达到阈值后应返回 blocked")
	}
	if !Is2FABlocked(ctx, userType, userID) {
		t.Fatal("达到阈值后 Is2FABlocked 应为 true")
	}

	// 计数键必须带 TTL（固定 300s 窗口，自首败起算自愈）
	ttl, err := g.Redis().Do(ctx, "TTL", key)
	if err != nil || ttl.Int64() <= 0 {
		t.Fatalf("失败计数键应有 TTL: ttl=%v err=%v", ttl, err)
	}

	// 验证成功后清零，恢复尝试机会
	Clear2FAFailures(ctx, userType, userID)
	if Is2FABlocked(ctx, userType, userID) {
		t.Fatal("清零后不应 blocked")
	}
}
