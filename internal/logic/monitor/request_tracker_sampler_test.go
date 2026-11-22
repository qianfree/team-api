package monitor

import (
	"context"
	"testing"
	"time"
)

// TestSamplerLoopUsesCapturedStop 采样循环体必须捕获 Init 时创建的局部 stop，
// 不能在每次执行时读全局 trackerStop。
//
// 回归背景：StopRequestTracker() 会 close(trackerStop) 并把它置 nil；而
// safego.RunLoop 在 fn panic 后会「重新执行同一个循环体」。若循环体读全局，
// 重启时拿到的是 nil channel，runSampler 的 select 中 <-stop 永不就绪，
// 采样 goroutine 永久泄漏，也不再响应后续停机。
func TestSamplerLoopUsesCapturedStop(t *testing.T) {
	// 让全局处于 Stop() 之后的状态（幂等；同包测试串行，不干扰其它用例）
	StopRequestTracker()
	if trackerStop != nil {
		t.Fatal("StopRequestTracker 后全局 trackerStop 应为 nil")
	}

	tr := &requestTracker{
		activeRequests: make(map[string]*TrackedRequest),
		concHistory:    newSnapshotRing[ConcurrencySnapshot](historyCapacity),
		bwHistory:      newSnapshotRing[BandwidthSnapshot](historyCapacity),
	}
	stop := make(chan struct{})
	loop := tr.samplerLoop(stop)
	close(stop)

	// 模拟 RunLoop 重启：多次执行同一个循环体，每次都应在已关闭的 stop 上立即退出。
	// 修复前循环体读全局 nil channel，会卡到 5s ticker 之后再也不返回。
	for i := range 3 {
		done := make(chan struct{})
		go func() {
			loop(context.Background())
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("第 %d 次执行采样循环未在 stop 关闭后退出：循环体可能读了全局 trackerStop(nil)", i+1)
		}
	}
}
