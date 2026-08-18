package common

import (
	"context"
	"testing"
)

// TestResponseRouteKey_Format 路由 key 按租户隔离、包含 response_id。
func TestResponseRouteKey_Format(t *testing.T) {
	got := responseRouteKey(42, "resp_abc")
	want := "relay:resp:route:42:resp_abc"
	if got != want {
		t.Errorf("responseRouteKey = %q, want %q", got, want)
	}
}

// TestResponseRouteStore_NilRedisNoop Redis 未配置（单测环境）时全程降级 no-op：
// Record/Delete 不 panic，Lookup 恒 miss。
func TestResponseRouteStore_NilRedisNoop(t *testing.T) {
	store := DefaultResponseRouteStore
	ctx := context.Background()

	// 不应 panic
	store.Record(ctx, 1, "resp_x", ResponseRoute{ChannelID: 1, ModelName: "gpt-4o"})
	store.Delete(ctx, 1, "resp_x")

	// Lookup 在无 Redis（或 Redis 不可达）时返回 miss，不报错
	if _, ok := store.Lookup(ctx, 1, "resp_missing"); ok {
		t.Error("Lookup should miss when redis is unavailable")
	}
}
