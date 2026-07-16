package task

import (
	"testing"
	"time"

	"github.com/qianfree/team-api/relay/common"
)

// TestBuildTimeoutFailure_UsesOriginalStatusAsCASPredicate 锁定超时兜底网的回归 bug：
// UpdateTaskCAS 的 oldStatus 必须是覆盖前的真实状态，而不是已改写的 "FAILURE"。
func TestBuildTimeoutFailure_UsesOriginalStatusAsCASPredicate(t *testing.T) {
	for _, orig := range []string{"IN_PROGRESS", "QUEUED", "SUBMITTED", "NOT_START"} {
		task := &common.AsyncTask{PublicTaskID: "task_x", Status: orig}
		now := time.Now()

		oldStatus := buildTimeoutFailure(task, now)

		if oldStatus != orig {
			t.Fatalf("oldStatus = %q, want %q (must be the pre-mutation status, never \"FAILURE\")", oldStatus, orig)
		}
		if oldStatus == "FAILURE" {
			t.Fatal("regression: oldStatus must not be the mutated FAILURE value")
		}
		if task.Status != "FAILURE" {
			t.Fatalf("task.Status = %q, want FAILURE", task.Status)
		}
		if task.FailReason != "task timed out" {
			t.Fatalf("task.FailReason = %q, want 'task timed out'", task.FailReason)
		}
		if task.FinishTime == nil || !task.FinishTime.Equal(now) {
			t.Fatal("task.FinishTime must be set to now")
		}
	}
}
