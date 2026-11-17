package helper

import (
	"errors"
	"io"
	"testing"
	"time"
)

// 空闲超时 Reader 是桥接 / Responses 直连路径唯一的上游假死保护：
// 超时必须能解锁阻塞中的 Read 并以可识别的哨兵错误返回，同时不丢已到达的数据。

func TestIdleTimeoutReader_UnblocksStalledRead(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	r := NewIdleTimeoutReader(pr, 50*time.Millisecond)
	defer r.Stop()

	// 超时前的数据正常送达
	go func() { _, _ = pw.Write([]byte("hello")) }()
	buf := make([]byte, 16)
	n, err := r.Read(buf)
	if err != nil || string(buf[:n]) != "hello" {
		t.Fatalf("first read = %q, %v; want hello, nil", buf[:n], err)
	}

	// 之后上游假死：Read 必须在超时后返回而不是永久阻塞
	done := make(chan error, 1)
	go func() {
		_, err := r.Read(buf)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ErrStreamIdleTimeout) {
			t.Fatalf("stalled read err = %v, want ErrStreamIdleTimeout", err)
		}
		if !r.TimedOut() {
			t.Error("TimedOut() = false after timeout")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stalled read did not return after idle timeout")
	}
}

func TestIdleTimeoutReader_ResetOnActivity(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	r := NewIdleTimeoutReader(pr, 80*time.Millisecond)
	defer r.Stop()

	// 持续以小于超时的间隔写入，累计时长远超单次超时，不得误触发
	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(30 * time.Millisecond)
			_, _ = pw.Write([]byte("x"))
		}
	}()
	buf := make([]byte, 4)
	for i := 0; i < 5; i++ {
		if _, err := r.Read(buf); err != nil {
			t.Fatalf("read %d: %v (timer should reset on each successful read)", i, err)
		}
	}
	if r.TimedOut() {
		t.Error("TimedOut() = true despite continuous activity")
	}
}

func TestIdleTimeoutReader_StopPreventsLateClose(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	r := NewIdleTimeoutReader(pr, 30*time.Millisecond)
	r.Stop()
	time.Sleep(60 * time.Millisecond)
	if r.TimedOut() {
		t.Fatal("timer fired after Stop()")
	}
	// 底层仍可用
	go func() { _, _ = pw.Write([]byte("ok")) }()
	buf := make([]byte, 4)
	if n, err := r.Read(buf); err != nil || string(buf[:n]) != "ok" {
		t.Fatalf("read after Stop = %q, %v", buf[:n], err)
	}
}

func TestIdleTimeoutReader_DisabledWhenNonPositive(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	r := NewIdleTimeoutReader(pr, 0)
	defer r.Stop()
	if r.timer != nil {
		t.Fatal("timeout<=0 should not arm a timer")
	}
	go func() { _, _ = pw.Write([]byte("ok")) }()
	buf := make([]byte, 4)
	if n, err := r.Read(buf); err != nil || string(buf[:n]) != "ok" {
		t.Fatalf("read = %q, %v", buf[:n], err)
	}
}
