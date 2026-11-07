package task

import (
	"encoding/json"
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

// TestPollStateChanged 状态变化判定（轮询调试记录的节流依据）：
// 状态或进度任一相对任务行当前持久化值变化才算变化，其余忽略
func TestPollStateChanged(t *testing.T) {
	task := &common.AsyncTask{Status: "IN_PROGRESS", Progress: "50%"}

	// 完全一致 → 未变化
	if pollStateChanged(&common.TaskInfo{Status: "IN_PROGRESS", Progress: "50%"}, task) {
		t.Error("状态与进度均一致时不应视为变化")
	}
	// 状态变化（终态转移）
	if !pollStateChanged(&common.TaskInfo{Status: "SUCCESS", Progress: "50%"}, task) {
		t.Error("状态变化应视为变化")
	}
	// 仅进度变化
	if !pollStateChanged(&common.TaskInfo{Status: "IN_PROGRESS", Progress: "60%"}, task) {
		t.Error("进度变化应视为变化")
	}
	// 空进度 → 有进度也算变化（提交后首次轮询）
	if !pollStateChanged(&common.TaskInfo{Status: "IN_PROGRESS", Progress: "10%"},
		&common.AsyncTask{Status: "SUBMITTED", Progress: ""}) {
		t.Error("空进度到有进度应视为变化")
	}
}

// TestPollDebugLogEnabled 轮询调试开关判定：JSONB 解析 + 目标过滤 AND 组合
func TestPollDebugLogEnabled(t *testing.T) {
	task := &common.AsyncTask{TenantID: 1, UserID: 2, ApiKeyID: 3}

	// 空/非法 settings 视为未开启
	if pollDebugLogEnabled(nil, task) || pollDebugLogEnabled([]byte("not-json"), task) {
		t.Error("空或非法 settings 应视为未开启")
	}
	// 未开启
	off, _ := json.Marshal(common.ChannelSettings{DebugLogEnabled: false})
	if pollDebugLogEnabled(off, task) {
		t.Error("DebugLogEnabled=false 应不开启")
	}
	// 开启且无过滤 → 匹配
	on, _ := json.Marshal(common.ChannelSettings{DebugLogEnabled: true})
	if !pollDebugLogEnabled(on, task) {
		t.Error("开启且无过滤应匹配")
	}
	// 开启但租户不匹配 → 拒绝
	filtered, _ := json.Marshal(common.ChannelSettings{DebugLogEnabled: true, DebugLogTenantID: 99})
	if pollDebugLogEnabled(filtered, task) {
		t.Error("租户过滤不匹配应拒绝")
	}
}

// TestJsPluginInactiveFailure 「JS 插件已停用」失败终态构造（表驱动）：
// 必须是终态 FAILURE + 固定中文失败原因（走 applyTaskPollResult 失败分支退款），
// 且相对任何非终态存量状态都构成状态变化（触发调试记录提交与 CAS 转移）
func TestJsPluginInactiveFailure(t *testing.T) {
	cases := []struct {
		name       string
		taskStatus string
	}{
		{"进行中任务", "IN_PROGRESS"},
		{"排队中任务", "QUEUED"},
		{"已提交任务", "SUBMITTED"},
		{"未开始任务", "NOT_START"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info := jsPluginInactiveFailure()
			if info == nil || info.Status != common.TaskStatusFailure {
				t.Fatalf("状态必须为 FAILURE，得到: %+v", info)
			}
			if !info.Status.IsTerminal() {
				t.Fatal("必须是终态（触发退款与审计路径）")
			}
			if info.FailReason != "任务所属 JS 插件已停用，自动终止并退款" {
				t.Fatalf("失败原因不符: %q", info.FailReason)
			}
			task := &common.AsyncTask{Status: c.taskStatus}
			if !pollStateChanged(info, task) {
				t.Fatalf("相对存量状态 %s 应视为状态变化", c.taskStatus)
			}
		})
	}
}
