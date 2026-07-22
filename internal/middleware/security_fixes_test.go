package middleware

import (
	"testing"
	"time"
)

// TestBuildIdempotencyKey 验证幂等键按认证主体隔离（对应安全修复 #7）。
// 核心不变量：不同租户/用户即便复用同一客户端 Idempotency-Key，也必须得到不同的存储键，
// 否则会命中彼此缓存的响应体（跨租户数据泄漏）或互相误触 409。
func TestBuildIdempotencyKey(t *testing.T) {
	const clientKey = "client-supplied-uuid"

	t.Run("相同主体+相同客户端key -> 相同存储键（幂等生效）", func(t *testing.T) {
		a := buildIdempotencyKey("tenant", 1, 10, clientKey)
		b := buildIdempotencyKey("tenant", 1, 10, clientKey)
		if a != b {
			t.Fatalf("same principal must yield same key: %q != %q", a, b)
		}
	})

	t.Run("不同租户复用同一客户端key -> 不同存储键（防跨租户泄漏）", func(t *testing.T) {
		tenantA := buildIdempotencyKey("tenant", 1, 10, clientKey)
		tenantB := buildIdempotencyKey("tenant", 2, 10, clientKey)
		if tenantA == tenantB {
			t.Fatalf("different tenants must not collide on the same client key: %q", tenantA)
		}
	})

	t.Run("同租户不同用户 -> 不同存储键", func(t *testing.T) {
		userA := buildIdempotencyKey("tenant", 1, 10, clientKey)
		userB := buildIdempotencyKey("tenant", 1, 11, clientKey)
		if userA == userB {
			t.Fatalf("different users must not collide: %q", userA)
		}
	})

	t.Run("不同用户类型 -> 不同存储键（admin 与 tenant 不混）", func(t *testing.T) {
		admin := buildIdempotencyKey("admin", 1, 10, clientKey)
		tenant := buildIdempotencyKey("tenant", 1, 10, clientKey)
		if admin == tenant {
			t.Fatalf("different user types must not collide: %q", admin)
		}
	})

	t.Run("不同客户端key -> 不同存储键", func(t *testing.T) {
		k1 := buildIdempotencyKey("tenant", 1, 10, "key-1")
		k2 := buildIdempotencyKey("tenant", 1, 10, "key-2")
		if k1 == k2 {
			t.Fatalf("different client keys must differ: %q", k1)
		}
	})
}

// TestOpenNonceKey 验证 nonce 去重键按应用隔离（对应安全修复 #4）。
func TestOpenNonceKey(t *testing.T) {
	t.Run("相同 app+nonce -> 相同键（确定性）", func(t *testing.T) {
		k1 := openNonceKey(7, "abc")
		k2 := openNonceKey(7, "abc")
		if k1 != k2 {
			t.Fatal("nonce key must be deterministic")
		}
	})

	t.Run("不同 app 相同 nonce -> 不同键（按应用隔离）", func(t *testing.T) {
		if openNonceKey(7, "abc") == openNonceKey(8, "abc") {
			t.Fatal("same nonce under different apps must not collide")
		}
	})

	t.Run("相同 app 不同 nonce -> 不同键", func(t *testing.T) {
		if openNonceKey(7, "abc") == openNonceKey(7, "xyz") {
			t.Fatal("different nonces must differ")
		}
	})
}

// TestOpenNonceTTL 验证 nonce 记录存活时间足以覆盖整个可重放窗口（对应安全修复 #4）。
// 请求在 [ts-skew, ts+skew] 内均可能通过时间戳校验，因此 nonce 至少要记住 2×skew，
// 才能保证同一 nonce 在其所有可被接受的时刻都已存在于 Redis。
func TestOpenNonceTTL(t *testing.T) {
	ttl := openNonceTTLSeconds()

	minRequired := int(2 * openPlatformMaxSkew / time.Second)
	if ttl < minRequired {
		t.Fatalf("nonce TTL %ds must cover the full replay window (>= %ds)", ttl, minRequired)
	}
	// 当前配置：skew=5min → TTL=600s
	if ttl != 600 {
		t.Fatalf("expected TTL 600s for a 5min skew window, got %ds", ttl)
	}
}
