package aireqanalyze

import (
	"testing"
	"time"
)

func TestCronJobsUseDefaultExpr(t *testing.T) {
	// CronJobs 必须是纯函数（注册期调用，不触库）：声明默认表达式，实际计划由 Init 校正
	p := &aiReqAnalyzePlugin{}
	for _, j := range p.CronJobs() {
		if j.Name == "ai-req-analyze_patrol" && j.CronExpr != defaultPatrolCron {
			t.Fatalf("巡查任务应声明默认表达式（Init 时按配置 Reschedule）: %q", j.CronExpr)
		}
	}
}

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

func TestCronJobsDeclaration(t *testing.T) {
	p := &aiReqAnalyzePlugin{}
	jobs := p.CronJobs()
	if len(jobs) != 2 {
		t.Fatalf("应声明巡查与清理两个任务: %d", len(jobs))
	}
	names := map[string]bool{}
	for _, j := range jobs {
		names[j.Name] = true
		if j.CronExpr == "" || j.Handler == nil {
			t.Fatalf("任务 %s 声明不完整", j.Name)
		}
	}
	if !names["ai-req-analyze_patrol"] || !names["ai-req-analyze_retention_cleanup"] {
		t.Fatalf("任务名声明错误: %v", names)
	}
}
