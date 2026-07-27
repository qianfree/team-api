package payment

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ─── LockOrder / UnlockOrder basic ──────────────────────────────────

func TestLockUnlock_Basic(t *testing.T) {
	orderNo := "test-order-basic"
	LockOrder(orderNo)
	UnlockOrder(orderNo)
}

func TestLockUnlock_DifferentOrders(t *testing.T) {
	done := make(chan struct{})
	go func() {
		LockOrder("order-a")
		LockOrder("order-b")
		UnlockOrder("order-b")
		UnlockOrder("order-a")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("different orders should not block each other")
	}
}

// 注：sharded mutex 使用裸 sync.Mutex，UnlockOrder 必须与 LockOrder 严格配对
// （生产代码均通过 defer 保证）。Unlock 一个未加锁的 mutex 会 panic，这是更严格的
// 契约，能暴露调用错误，故不再保留「UnlockOrder 不存在订单」的容错测试。

// ─── Mutual exclusion ───────────────────────────────────────────────

func TestLockOrder_MutualExclusion(t *testing.T) {
	orderNo := "test-order-mutex"
	var counter int64

	LockOrder(orderNo)

	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		LockOrder(orderNo)
		atomic.AddInt64(&counter, 1)
		UnlockOrder(orderNo)
		close(done)
	}()

	<-started
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt64(&counter) != 0 {
		t.Fatal("goroutine should be blocked by the lock")
	}

	UnlockOrder(orderNo)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine should have completed after unlock")
	}

	if atomic.LoadInt64(&counter) != 1 {
		t.Fatal("goroutine should have incremented counter")
	}
}

// ─── Multiple waiters serialize through the same shard ─────────────

// TestLockOrder_SerializesMultipleWaiters 验证同一订单的多个等待者在持有者释放后
// 能依次拿到锁。sharded mutex 不再有引用计数/清理语义（分片静态存在），故不再断言
// map 清理，仅断言所有等待者最终都完成。
func TestLockOrder_SerializesMultipleWaiters(t *testing.T) {
	orderNo := "test-order-refcount"
	var sequence []int
	var mu sync.Mutex

	LockOrder(orderNo)

	var wg sync.WaitGroup
	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			LockOrder(orderNo)
			mu.Lock()
			sequence = append(sequence, id)
			mu.Unlock()
			UnlockOrder(orderNo)
		}(i)
	}

	time.Sleep(50 * time.Millisecond)
	UnlockOrder(orderNo)

	wg.Wait()

	if len(sequence) != 3 {
		t.Fatalf("expected 3 goroutines to complete, got %d", len(sequence))
	}
}

// ─── Shard determinism ─────────────────────────────────────────────

// TestOrderLockShardIndex_Deterministic 验证同一 orderNo 始终映射到同一分片
// （这是 sharded mutex 对同一订单互斥正确性的前提）。
func TestOrderLockShardIndex_Deterministic(t *testing.T) {
	for _, orderNo := range []string{"ORD-2024-0001", "abc", "x", "169.254.169.254"} {
		first := orderLockShardIndex(orderNo)
		for i := 0; i < 10; i++ {
			if got := orderLockShardIndex(orderNo); got != first {
				t.Fatalf("orderLockShardIndex(%q) not deterministic: %d vs %d", orderNo, got, first)
			}
			if first < 0 || first >= orderLockShardCount {
				t.Fatalf("orderLockShardIndex(%q) = %d out of range [0,%d)", orderNo, first, orderLockShardCount)
			}
		}
	}
}

// ─── Concurrent lock/unlock stress ──────────────────────────────────

func TestLockOrder_ConcurrentStress(t *testing.T) {
	orderNo := "test-order-stress"
	var counter int64
	const goroutines = 50

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			LockOrder(orderNo)
			val := atomic.LoadInt64(&counter)
			atomic.StoreInt64(&counter, val+1)
			UnlockOrder(orderNo)
		}()
	}

	wg.Wait()

	if counter != goroutines {
		t.Fatalf("expected counter=%d, got %d (race condition detected)", goroutines, counter)
	}
}

// ─── Re-lock after full release ─────────────────────────────────────

// TestLockOrder_ReuseAfterRelease 验证释放后可再次加锁。sharded mutex 分片静态存在，
// 无需（也不再有）清理动作，只需确保 unlock 后同订单可重新 lock 而不死锁。
func TestLockOrder_ReuseAfterRelease(t *testing.T) {
	orderNo := "test-order-reuse"

	LockOrder(orderNo)
	UnlockOrder(orderNo)

	LockOrder(orderNo)
	UnlockOrder(orderNo)
}
