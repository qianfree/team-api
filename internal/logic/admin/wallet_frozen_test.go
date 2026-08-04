package admin

import (
	"testing"
	"time"
)

// TestFrozenItemGuard 覆盖手动释放冻结的护栏矩阵：
// 任务关联（非终态/终态未结算/终态已结算）×冻结时长（保护期内/外）。
func TestFrozenItemGuard(t *testing.T) {
	t.Parallel()

	oldAge := frozenReleaseMinAge + time.Minute
	youngAge := frozenReleaseMinAge - time.Minute

	cases := []struct {
		name           string
		task           *frozenTaskInfo
		age            time.Duration
		wantReleasable bool
		wantNeedForce  bool
		wantBlocked    bool
	}{
		{
			name: "无任务关联+超过保护期→直接可释放",
			task: nil, age: oldAge,
			wantReleasable: true, wantNeedForce: false,
		},
		{
			name: "无任务关联+保护期内→需强制",
			task: nil, age: youngAge,
			wantReleasable: true, wantNeedForce: true,
		},
		{
			name: "任务进行中→拦截（force 也不放行）",
			task: &frozenTaskInfo{Status: "IN_PROGRESS"}, age: oldAge,
			wantReleasable: false, wantBlocked: true,
		},
		{
			name: "任务已提交→拦截",
			task: &frozenTaskInfo{Status: "SUBMITTED"}, age: youngAge,
			wantReleasable: false, wantBlocked: true,
		},
		{
			name: "任务终态但未结算→拦截（轮询器将自动重试）",
			task: &frozenTaskInfo{Status: "FAILURE", BillingSettled: false}, age: oldAge,
			wantReleasable: false, wantBlocked: true,
		},
		{
			name: "任务终态且已结算+超过保护期→真孤儿，可释放",
			task: &frozenTaskInfo{Status: "SUCCESS", BillingSettled: true}, age: oldAge,
			wantReleasable: true, wantNeedForce: false,
		},
		{
			name: "任务终态且已结算+保护期内→需强制",
			task: &frozenTaskInfo{Status: "SUCCESS", BillingSettled: true}, age: youngAge,
			wantReleasable: true, wantNeedForce: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			releasable, needForce, blockReason := frozenItemGuard(tc.task, tc.age)
			if releasable != tc.wantReleasable {
				t.Errorf("releasable = %v, want %v", releasable, tc.wantReleasable)
			}
			if needForce != tc.wantNeedForce {
				t.Errorf("needForce = %v, want %v", needForce, tc.wantNeedForce)
			}
			if tc.wantBlocked && blockReason == "" {
				t.Errorf("blockReason 为空，预期给出拦截原因")
			}
			if !tc.wantBlocked && blockReason != "" {
				t.Errorf("blockReason = %q，预期为空", blockReason)
			}
		})
	}
}

// TestIsTaskTerminal 任务终态判断
func TestIsTaskTerminal(t *testing.T) {
	t.Parallel()

	terminal := []string{"SUCCESS", "FAILURE"}
	nonTerminal := []string{"NOT_START", "SUBMITTED", "IN_PROGRESS", "QUEUED", ""}

	for _, s := range terminal {
		if !isTaskTerminal(s) {
			t.Errorf("isTaskTerminal(%q) = false, want true", s)
		}
	}
	for _, s := range nonTerminal {
		if isTaskTerminal(s) {
			t.Errorf("isTaskTerminal(%q) = true, want false", s)
		}
	}
}
