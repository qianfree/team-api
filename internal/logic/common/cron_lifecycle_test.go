package common

import (
	"context"
	"testing"
)

func noopHandler(ctx context.Context) error { return nil }

// TestRegisterReplacesDuplicate 同名重复注册按覆盖处理（插件禁用→再启用的路径），
// 不产生双条目——否则同一任务会被 cron 挂载两次，靠 running 互斥只是兜底。
func TestRegisterReplacesDuplicate(t *testing.T) {
	cs := NewCronScheduler()
	cs.Register("dup_job", "任务", "0 3 * * *", noopHandler)
	cs.Register("dup_job", "任务-新计划", "0 5 * * *", noopHandler)

	jobs := cs.ListJobs()
	count := 0
	for _, j := range jobs {
		if j.Name == "dup_job" {
			count++
			if j.Schedule != "0 5 * * *" {
				t.Fatalf("覆盖注册应更新表达式: %s", j.Schedule)
			}
		}
	}
	if count != 1 {
		t.Fatalf("同名注册应覆盖而非追加，实际 %d 条", count)
	}
}

// TestRemoveUnregisters Remove 从任务表与条目表同时摘除，且幂等。
func TestRemoveUnregisters(t *testing.T) {
	cs := NewCronScheduler()
	cs.Register("gone_job", "任务", "0 3 * * *", noopHandler)

	cs.Remove("gone_job")
	for _, j := range cs.ListJobs() {
		if j.Name == "gone_job" {
			t.Fatal("Remove 后任务不应留在列表")
		}
	}

	cs.Remove("gone_job") // 幂等：不存在时静默
	cs.Remove("never_existed")
}

// TestRegisterAfterStartMountsEntry 调度器启动后再注册（运行时启用插件），
// 条目必须即时挂载——否则任务进得了列表却永远不触发。
func TestRegisterAfterStartMountsEntry(t *testing.T) {
	cs := NewCronScheduler()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cs.StartBackground(ctx)

	cs.Register("late_job", "运行时注册", "0 3 * * *", noopHandler)

	cs.entryMu.Lock()
	_, mounted := cs.entryIDs["late_job"]
	cs.entryMu.Unlock()
	if !mounted {
		t.Fatal("调度器启动后注册的任务应即时挂载条目")
	}

	// 摘除后条目消失
	cs.Remove("late_job")
	cs.entryMu.Lock()
	_, mounted = cs.entryIDs["late_job"]
	cs.entryMu.Unlock()
	if mounted {
		t.Fatal("Remove 后条目应被卸下")
	}

	stopCtx := cs.cron.Stop()
	<-stopCtx.Done()
}
