package jsplugin

import (
	"context"
	"testing"
)

// TestShouldRebuildInvalidate 消息来源判定：仅本实例 ID 跳过，其余（其他实例 ID、
// 手工运维的 "*"、空载荷）一律触发重建。
func TestShouldRebuildInvalidate(t *testing.T) {
	if shouldRebuildInvalidate(invalidateNodeID) {
		t.Fatal("来源为本实例的消息应跳过（管理操作内已同步重建）")
	}
	for _, payload := range []string{"other-node", "*", ""} {
		if !shouldRebuildInvalidate(payload) {
			t.Fatalf("载荷 %q 应触发重建", payload)
		}
	}
}

// TestPublishInvalidate_NoRedisNoPanic 单测环境无 Redis 配置：探测返回 nil，
// 广播静默降级，不 panic（g.Redis() 未配置时直接 panic，见 goframe-conventions）。
func TestPublishInvalidate_NoRedisNoPanic(t *testing.T) {
	DefaultRegistry().PublishInvalidate(context.Background())
	// 能走到这里即未 panic；无 Redis 时不发布任何消息
}

// TestStartInvalidateSubscriber_NoRedisNoop 无 Redis 时不启动订阅循环（无 goroutine 泄漏面）。
func TestStartInvalidateSubscriber_NoRedisNoop(t *testing.T) {
	DefaultRegistry().StartInvalidateSubscriber() // 应静默返回
	stopInvalidateSubscriber()                    // 幂等：未启动时也应安全
}

// TestStopInvalidateSubscriber_Idempotent 连续停止不 panic（close 只应发生一次）。
func TestStopInvalidateSubscriber_Idempotent(t *testing.T) {
	stopInvalidateSubscriber()
	stopInvalidateSubscriber()
}
