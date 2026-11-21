package tenant

import (
	"context"
	"testing"
	"time"
)

// TestWebhookDispatcherShutdownWaitGroupPaired 停机时 wg.Add/wg.Done 必须恰好配对。
//
// 回归背景：worker 曾同时在外层 safego.Run 闭包和 worker() 内各 defer 一次
// wg.Done()，而 startWorkers 只 Add(1) 一次 → 计数被减成负数，触发
// "sync: negative WaitGroup counter" panic（被 safego 吞进日志），且 wg.Wait()
// 会在尚有 worker 未收尾时提前返回 / 永久阻塞，违背「等待在途投递完成」语义。
//
// 本测试不投递任何事件，避免触发 deliverEventByID 查库；只验证停机收敛与计数归零。
func TestWebhookDispatcherShutdownWaitGroupPaired(t *testing.T) {
	d := &WebhookDispatcher{
		eventCh: make(chan int64, dispatcherChannelSize),
		ctx:     context.Background(),
		stopCh:  make(chan struct{}),
	}
	d.startWorkers(context.Background())

	done := make(chan struct{})
	go func() {
		d.Shutdown() // close(stopCh) + wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown 未在超时内返回：wg.Add/wg.Done 未配对（重复 Done 使计数变负，Wait 阻塞）")
	}

	// 停机后 WaitGroup 计数必须恰好为 0。若存在多余 Done，counter 为负，
	// 下面的 Add(1)/Done() 会命中 sync.WaitGroup 的 v<0 检查并 panic。
	panicked := make(chan bool, 1)
	go func() {
		defer func() { panicked <- recover() != nil }()
		d.wg.Add(1)
		d.wg.Done()
	}()
	select {
	case p := <-panicked:
		if p {
			t.Fatal("停机后 WaitGroup 计数为负：worker 存在重复的 wg.Done()")
		}
	case <-time.After(time.Second):
		t.Fatal("检测 WaitGroup 状态超时")
	}
}
