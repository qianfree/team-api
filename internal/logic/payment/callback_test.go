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

func TestUnlockOrder_NonExistent(t *testing.T) {
	UnlockOrder("never-locked-order")
}

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

// ─── Ref-counting cleanup ───────────────────────────────────────────

func TestLockOrder_RefCounting(t *testing.T) {
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

	_, exists := orderLocks.Load(orderNo)
	if exists {
		t.Fatal("lock entry should be cleaned up after all refs are released")
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

func TestLockOrder_ReuseAfterRelease(t *testing.T) {
	orderNo := "test-order-reuse"

	LockOrder(orderNo)
	UnlockOrder(orderNo)

	_, exists := orderLocks.Load(orderNo)
	if exists {
		t.Fatal("lock should be cleaned up after release")
	}

	LockOrder(orderNo)
	UnlockOrder(orderNo)
}
