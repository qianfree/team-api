package monitor

import (
	"sync"
	"testing"
	"time"
)

func TestSnapshotBandwidthConcurrent(t *testing.T) {
	tracker := &requestTracker{}
	tracker.bytesIn.Store(1024)
	tracker.bytesOut.Store(2048)

	base := time.Now()
	var workers sync.WaitGroup
	for i := range 100 {
		workers.Add(1)
		go func(offset int) {
			defer workers.Done()
			tracker.snapshotBandwidth(base.Add(time.Duration(offset) * time.Millisecond))
		}(i)
	}
	workers.Wait()

	tracker.bytesIn.Add(100)
	tracker.bytesOut.Add(200)
	snapshot := tracker.snapshotBandwidth(base.Add(time.Second))
	if snapshot.BytesInPerSec < 0 || snapshot.BytesOutPerSec < 0 {
		t.Fatalf("bandwidth rates must not be negative: %+v", snapshot)
	}
}

// 读接口不得推进采样：历史点只能来自后台采样器，
// 否则页面关闭期间不采样、旧曲线不过期，再打开时会把上次会话的并发/带宽当成"刚刚"画出来。
func TestGetRealtimeDataDoesNotSample(t *testing.T) {
	InitRequestTracker()
	defer StopRequestTracker()

	for range 3 {
		if data := GetRealtimeData(); len(data.History) != 0 || len(data.BandwidthHistory) != 0 {
			t.Fatalf("读接口不应写入历史点: conc=%d bw=%d", len(data.History), len(data.BandwidthHistory))
		}
	}

	tracker.sample(time.Now())
	data := GetRealtimeData()
	if len(data.History) != 1 || len(data.BandwidthHistory) != 1 {
		t.Fatalf("采样器应产出 1 个点: conc=%d bw=%d", len(data.History), len(data.BandwidthHistory))
	}
}

// 超时未反注册的悬挂条目必须被清扫，且计数以 map 为准重算（修正漏减漂移）。
func TestSweepStaleDropsHangingRequests(t *testing.T) {
	tr := &requestTracker{activeRequests: make(map[string]*TrackedRequest)}
	now := time.Now()

	tr.activeRequests["fresh"] = &TrackedRequest{RequestID: "fresh", StartTime: now.Add(-time.Minute), IsStream: true}
	tr.activeRequests["hung-req"] = &TrackedRequest{RequestID: "hung-req", StartTime: now.Add(-staleRequestTTL - time.Minute)}
	tr.activeRequests["hung-task"] = &TrackedRequest{RequestID: "hung-task", StartTime: now.Add(-staleTaskTTL - time.Minute), IsAsyncTask: true}
	// 未到异步任务 TTL 的长任务应保留
	tr.activeRequests["live-task"] = &TrackedRequest{RequestID: "live-task", StartTime: now.Add(-staleRequestTTL - time.Minute), IsAsyncTask: true}
	// 计数故意置成漂移值，验证重算能自愈
	tr.totalActive.Store(99)

	total, streaming := tr.sweepStale(now)
	if total != 2 || streaming != 1 {
		t.Fatalf("清扫后应剩 2 条（1 条流式），实际 total=%d streaming=%d", total, streaming)
	}
	if got := tr.totalActive.Load(); got != 2 {
		t.Fatalf("计数应按 map 重算为 2，实际 %d", got)
	}
	if _, ok := tr.activeRequests["live-task"]; !ok {
		t.Fatal("未超时的异步任务被误清扫")
	}
}

func TestFilterRecentDropsStalePoints(t *testing.T) {
	now := time.Now()
	items := []ConcurrencySnapshot{
		{Timestamp: now.Add(-2 * historyWindow), TotalActive: 7}, // 上次会话遗留
		{Timestamp: now.Add(-time.Minute), TotalActive: 1},
		{Timestamp: now, TotalActive: 2},
	}
	got := filterRecent(items, now.Add(-historyWindow), func(s ConcurrencySnapshot) time.Time { return s.Timestamp })
	if len(got) != 2 || got[0].TotalActive != 1 {
		t.Fatalf("应只保留窗口内的 2 个点，实际 %+v", got)
	}
}
