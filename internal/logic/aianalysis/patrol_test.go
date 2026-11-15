package aianalysis

import (
	"testing"
	"time"
)

func TestPatrolWindow(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.Local)
	w := patrolWindow(now, 24)
	if got := now.Sub(w.start); got != 24*time.Hour {
		t.Fatalf("窗口长度错误: %v", got)
	}
	if !w.end.Equal(now) {
		t.Fatalf("窗口终点应为当前时刻: %v", w.end)
	}

	// 非法窗口时长兜底为 24h
	w = patrolWindow(now, 0)
	if got := now.Sub(w.start); got != 24*time.Hour {
		t.Fatalf("非法时长应兜底 24h: %v", got)
	}
}

func TestPatrolScheduleConstants(t *testing.T) {
	// 任务名是分布式锁 key 与 sys_cron_jobs 主键，必须稳定且与插件期解耦
	if PatrolJobName != "ai_analysis_patrol" || RetentionJobName != "ai_analysis_retention_cleanup" {
		t.Fatalf("任务名错误: %s / %s", PatrolJobName, RetentionJobName)
	}
	// 清理计划与巡查计划错开分钟，避免整点扎堆
	if DefaultPatrolCron == RetentionCleanupCron {
		t.Fatalf("巡查与清理计划不应同刻: %s", DefaultPatrolCron)
	}
}
