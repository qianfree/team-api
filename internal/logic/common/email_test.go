package common

import (
	"context"
	"errors"
	"testing"
	"time"
)

// sleepCtx 是重试退避可被取消的关键：调用方（如邮件配置连通性测试）设了超时后，
// 不能再被 2s/4s 的固定退避拖满。
func TestSleepCtx_WaitsFullDuration(t *testing.T) {
	start := time.Now()
	if err := sleepCtx(context.Background(), 30*time.Millisecond); err != nil {
		t.Fatalf("sleepCtx() = %v, want nil", err)
	}
	if elapsed := time.Since(start); elapsed < 30*time.Millisecond {
		t.Fatalf("sleepCtx 提前返回：耗时 %v，期望至少 30ms", elapsed)
	}
}

func TestSleepCtx_ReturnsImmediatelyOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := sleepCtx(ctx, 5*time.Second)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("sleepCtx() = %v, want context.Canceled", err)
	}
	if elapsed > time.Second {
		t.Fatalf("sleepCtx 未被取消打断：耗时 %v", elapsed)
	}
}

func TestSleepCtx_InterruptedByDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := sleepCtx(ctx, 5*time.Second)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("sleepCtx() = %v, want context.DeadlineExceeded", err)
	}
	if elapsed > time.Second {
		t.Fatalf("sleepCtx 未在超时后返回：耗时 %v", elapsed)
	}
}

func TestIsValidEmail(t *testing.T) {
	cases := map[string]bool{
		"system@mail.aifree.com": true,
		"a@b.co":                 true,
		"no-at-sign.com":         false,
		"no-dot@localhost":       false,
		"":                       false,
	}
	for input, want := range cases {
		if got := IsValidEmail(input); got != want {
			t.Errorf("IsValidEmail(%q) = %v, want %v", input, got, want)
		}
	}
}
