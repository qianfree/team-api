package common

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunHandlerSafely_PanicBecomesError robfig/cron v3 的 cron.New() 不带 Recover 链，
// startJob 是裸的 `go j.Run()`——任务 panic 会崩掉整个进程。此处必须兜住并转成
// 普通 error，才能既保住进程、又让失败落进 sys_cron_jobs 供管理后台查看。
func TestRunHandlerSafely_PanicBecomesError(t *testing.T) {
	job := &CronJob{
		Name: "panicking_job",
		Handler: func(_ context.Context) error {
			panic("boom")
		},
	}

	var err error
	assert.NotPanics(t, func() {
		err = runHandlerSafely(context.Background(), job)
	}, "panic 必须被兜住，否则整个进程会崩")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom", "错误信息应保留 panic 内容供排查")
}

func TestRunHandlerSafely_PassesThroughReturnValue(t *testing.T) {
	ctx := context.Background()

	okJob := &CronJob{Name: "ok", Handler: func(_ context.Context) error { return nil }}
	assert.NoError(t, runHandlerSafely(ctx, okJob))

	failJob := &CronJob{Name: "fail", Handler: func(_ context.Context) error {
		return assert.AnError
	}}
	assert.ErrorIs(t, runHandlerSafely(ctx, failJob), assert.AnError, "普通错误必须原样返回")
}

// TestReschedule_OnlyUpdatesRecordWhenNotStarted StartBackground 之前没有 cron 条目，
// 此时重排只改 job.Schedule，供随后的 StartBackground 按新表达式注册。
func TestReschedule_OnlyUpdatesRecordWhenNotStarted(t *testing.T) {
	cs := NewCronScheduler()
	cs.Register("job_a", "任务A", "*/5 * * * *", func(_ context.Context) error { return nil })

	require.NoError(t, cs.Reschedule(context.Background(), "job_a", "@every 30m"))

	jobs := cs.ListJobs()
	require.Len(t, jobs, 1)
	assert.Equal(t, "@every 30m", jobs[0].Schedule)
}

func TestReschedule_UnregisteredTaskErrors(t *testing.T) {
	cs := NewCronScheduler()
	assert.Error(t, cs.Reschedule(context.Background(), "nonexistent", "@every 5m"))
}

// TestReschedule_InvalidExprKeepsExistingSchedule 一次错误配置不能把任务从调度器里彻底摘掉，
// 因此实现是「先加新条目、成功后再摘旧条目」。
func TestReschedule_InvalidExprKeepsExistingSchedule(t *testing.T) {
	ctx := context.Background()
	cs := NewCronScheduler()
	cs.Register("job_b", "任务B", "@every 5m", func(_ context.Context) error { return nil })
	cs.StartBackground(ctx)
	defer cs.cron.Stop()

	before := cs.ListJobs()[0].Schedule
	require.Error(t, cs.Reschedule(ctx, "job_b", "不是合法表达式"))
	assert.Equal(t, before, cs.ListJobs()[0].Schedule, "非法表达式不得改动原调度")

	// 条目仍在调度器中，未被误摘
	assert.Len(t, cs.cron.Entries(), 1)
}

// TestReschedule_ValidExprReplacesEntry 同时验证 @every 语法确实被 cron.New() 的
// 默认解析器接受——探测间隔配置正是靠它支持任意分钟数（*/N 在 N>=60 时会静默失真）。
func TestReschedule_ValidExprReplacesEntry(t *testing.T) {
	ctx := context.Background()
	cs := NewCronScheduler()
	cs.Register("job_c", "任务C", "@every 5m", func(_ context.Context) error { return nil })
	cs.StartBackground(ctx)
	defer cs.cron.Stop()

	require.Len(t, cs.cron.Entries(), 1)
	oldID := cs.entryIDs["job_c"]

	require.NoError(t, cs.Reschedule(ctx, "job_c", "@every 90m"))

	assert.Equal(t, "@every 90m", cs.ListJobs()[0].Schedule)
	assert.Len(t, cs.cron.Entries(), 1, "旧条目必须被摘除，否则会双倍触发")
	assert.NotEqual(t, oldID, cs.entryIDs["job_c"], "条目 ID 应已更新")
}

// TestScheduleInterval 停摆判定靠它把「固定 1 小时」换成「按任务自身周期」，
// 各类表达式的间隔必须算准——尤其日任务（24h），算错就会退回误报。
func TestScheduleInterval(t *testing.T) {
	cases := []struct {
		schedule string
		want     time.Duration
	}{
		{"* * * * *", time.Minute},         // 每分钟
		{"*/5 * * * *", 5 * time.Minute},   // 每 5 分钟
		{"*/10 * * * *", 10 * time.Minute}, // 每 10 分钟
		{"0 */6 * * *", 6 * time.Hour},     // 每 6 小时（update_check）
		{"0 3 * * *", 24 * time.Hour},      // 每日 3 点（各类日清理）
		{"20 5 * * *", 24 * time.Hour},     // 每日 5:20（计费日对账）
		{"@every 5m", 5 * time.Minute},     // @every 描述符（渠道自动探测默认）
		{"@every 90m", 90 * time.Minute},   // @every 大间隔（*/N 在 N>=60 时会失真，故用 @every）
	}
	for _, c := range cases {
		got, err := ScheduleInterval(c.schedule)
		require.NoError(t, err, "schedule %q 应可解析", c.schedule)
		assert.Equal(t, c.want, got, "schedule %q 的正常间隔不符", c.schedule)
	}

	_, err := ScheduleInterval("不是合法表达式")
	assert.Error(t, err, "非法表达式应返回错误，由调用方兜底")
}

// TestOnSettingsChanged_CallbackPanicDoesNotAffectOthers 回调在订阅 goroutine 内串行执行，
// 一个回调 panic 不能带崩订阅循环或阻断后续回调。
func TestOnSettingsChanged_CallbackPanicDoesNotAffectOthers(t *testing.T) {
	settingsHookMu.Lock()
	saved := settingsHooks
	settingsHooks = nil
	settingsHookMu.Unlock()
	defer func() {
		settingsHookMu.Lock()
		settingsHooks = saved
		settingsHookMu.Unlock()
	}()

	var firstCalled, thirdCalled bool
	OnSettingsChanged(func(_ context.Context, _ string) { firstCalled = true })
	OnSettingsChanged(func(_ context.Context, _ string) { panic("hook boom") })
	OnSettingsChanged(func(_ context.Context, _ string) { thirdCalled = true })

	assert.NotPanics(t, func() {
		fireSettingsChanged(context.Background(), "some_key")
	})
	assert.True(t, firstCalled)
	assert.True(t, thirdCalled, "前一个回调 panic 不得阻断后续回调")
}
