package safego

import (
	"testing"
	"time"
)

// TestNextLoopBackoffRespectsCap 退避必须严格封顶 loopRestartBackoffMax。
//
// 回归背景：初版写成 `if backoff < max { backoff *= 2 }`，序列会是
// 5→10→20→40→80→80…：40s 因仍小于 60s 而翻倍到 80s，突破注释声明的
// 「封顶 60s」上限。正确做法是翻倍后再 clamp。
func TestNextLoopBackoffRespectsCap(t *testing.T) {
	cases := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{"首次翻倍", loopRestartBackoffInitial, 10 * time.Second},
		{"10s→20s", 10 * time.Second, 20 * time.Second},
		{"20s→40s", 20 * time.Second, 40 * time.Second},
		{"40s 封顶到 60s（回归关键）", 40 * time.Second, loopRestartBackoffMax},
		{"已达上限保持不变", loopRestartBackoffMax, loopRestartBackoffMax},
		{"异常超出上限也收敛回上限", 80 * time.Second, loopRestartBackoffMax},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := nextLoopBackoff(c.in); got != c.want {
				t.Fatalf("nextLoopBackoff(%s) = %s, want %s", c.in, got, c.want)
			}
		})
	}
}

// TestNextLoopBackoffSequence 从初始值连续迭代，验证序列单调不减、
// 永不突破上限，且最终稳定在上限（不出现 80s）。
func TestNextLoopBackoffSequence(t *testing.T) {
	want := []time.Duration{5, 10, 20, 40, 60, 60, 60, 60, 60, 60, 60}
	got := []time.Duration{loopRestartBackoffInitial}
	cur := loopRestartBackoffInitial
	for range len(want) - 1 {
		cur = nextLoopBackoff(cur)
		if cur > loopRestartBackoffMax {
			t.Fatalf("第 %d 次退避突破上限: %s > %s", len(got), cur, loopRestartBackoffMax)
		}
		if cur < got[len(got)-1] {
			t.Fatalf("退避序列非单调不减: %s < %s", cur, got[len(got)-1])
		}
		got = append(got, cur)
	}
	for i := range want {
		if got[i] != want[i]*time.Second {
			t.Fatalf("退避序列不符: got %v, want %v (秒)", got, want)
		}
	}
}
