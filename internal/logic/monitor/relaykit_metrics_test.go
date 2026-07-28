package monitor

import (
	"errors"
	"testing"
	"time"
)

// resetTracker 在每个测试前重置单例，保证测试隔离（InitRelaykitTracker 会新建空 map）。
func resetTracker() {
	InitRelaykitTracker()
}

func TestTrackConverterCall_Success(t *testing.T) {
	resetTracker()

	for i := 0; i < 3; i++ {
		TrackConverterCall("openai_to_claude", "openai", "claude", 2*time.Millisecond, nil)
	}

	got := GetRelaykitConverterMetrics()
	if len(got) != 1 {
		t.Fatalf("expected 1 converter, got %d", len(got))
	}
	m := got[0]
	if m.ConverterID != "openai_to_claude" || m.From != "openai" || m.To != "claude" {
		t.Errorf("unexpected identity: %+v", m)
	}
	if m.Success != 3 || m.Failed != 0 {
		t.Errorf("expected success=3 failed=0, got success=%d failed=%d", m.Success, m.Failed)
	}
	if m.TotalMs != 6 {
		t.Errorf("expected total_ms=6, got %d", m.TotalMs)
	}
	if m.ErrorRate != 0 {
		t.Errorf("expected error_rate=0, got %v", m.ErrorRate)
	}
	if m.AvgDurationMs != 2 {
		t.Errorf("expected avg_duration_ms=2, got %v", m.AvgDurationMs)
	}
	if m.LastError != "" {
		t.Errorf("expected empty last_error, got %q", m.LastError)
	}
}

func TestTrackConverterCall_Failure(t *testing.T) {
	resetTracker()

	TrackConverterCall("claude_to_openai", "claude", "openai", 1*time.Millisecond, nil)
	TrackConverterCall("claude_to_openai", "claude", "openai", 3*time.Millisecond, errors.New("boom"))

	got := GetRelaykitConverterMetrics()
	if len(got) != 1 {
		t.Fatalf("expected 1 converter, got %d", len(got))
	}
	m := got[0]
	if m.Success != 1 || m.Failed != 1 {
		t.Errorf("expected success=1 failed=1, got success=%d failed=%d", m.Success, m.Failed)
	}
	if m.TotalMs != 4 {
		t.Errorf("expected total_ms=4, got %d", m.TotalMs)
	}
	// error_rate = failed/(success+failed) = 1/2 = 0.5
	if m.ErrorRate != 0.5 {
		t.Errorf("expected error_rate=0.5, got %v", m.ErrorRate)
	}
	// avg_duration_ms = total_ms/(success+failed) = 4/2 = 2
	if m.AvgDurationMs != 2 {
		t.Errorf("expected avg_duration_ms=2, got %v", m.AvgDurationMs)
	}
	if m.LastError != "boom" {
		t.Errorf("expected last_error='boom', got %q", m.LastError)
	}
}

func TestTrackConverterCall_MultipleConvertersSorted(t *testing.T) {
	resetTracker()

	// 故意乱序注册
	TrackConverterCall("gemini_to_openai", "gemini", "openai", 1*time.Millisecond, nil)
	TrackConverterCall("openai_to_claude", "openai", "claude", 1*time.Millisecond, nil)
	TrackConverterCall("claude_to_openai", "claude", "openai", 1*time.Millisecond, nil)

	got := GetRelaykitConverterMetrics()
	if len(got) != 3 {
		t.Fatalf("expected 3 converters, got %d", len(got))
	}
	// 结果按 converterID 字典序稳定排列
	wantOrder := []string{"claude_to_openai", "gemini_to_openai", "openai_to_claude"}
	for i, w := range wantOrder {
		if got[i].ConverterID != w {
			t.Errorf("position %d: expected %q, got %q", i, w, got[i].ConverterID)
		}
	}
}

func TestGetRelaykitConverterMetrics_EmptyReturnsNil(t *testing.T) {
	resetTracker()
	if got := GetRelaykitConverterMetrics(); got != nil {
		t.Errorf("expected nil when no activity, got %v", got)
	}
}

func TestTrackConverterCall_NilTrackerNoPanic(t *testing.T) {
	// 未初始化时调用不得 panic（安全默认）
	relaykitT = nil
	defer resetTracker()

	TrackConverterCall("openai_to_claude", "openai", "claude", 1*time.Millisecond, nil)
	if got := GetRelaykitConverterMetrics(); got != nil {
		t.Errorf("expected nil when tracker uninitialized, got %v", got)
	}
}

func TestFlushRelaykitMetrics_NoActivitySkips(t *testing.T) {
	// 无活动时应早退，不触碰 metricsWriter（测试环境未初始化 DB writer）
	resetTracker()
	flushRelaykitMetrics(time.Now()) // 不应 panic
}
