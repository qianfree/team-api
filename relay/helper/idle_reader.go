package helper

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// ErrStreamIdleTimeout 上游流两帧之间的空闲间隔超过 streaming_timeout_seconds。
// 经 bufio.Scanner 的 scanner.Err() 原样透出（转换器用 %w 包装），宿主以 errors.Is 识别后
// 记 StreamEndReasonTimeout 并向客户端补发协议原生的终止事件。
var ErrStreamIdleTimeout = errors.New("stream idle timeout")

// IdleTimeoutReader 给上游流式响应体加「两帧之间的空闲超时」。
//
// StreamScannerHandler 的空闲超时只覆盖 OpenAI 同格式透传路径；relaykit 桥接与 Responses
// 直连透传直接 scanner.Scan() 读 resp.Body，而流式 http.Client 没有 Timeout、请求 ctx 也没有
// deadline——上游假死（连接保持、不再发数据）时客户端会被 15s ping 维持着无限挂起。
//
// 阻塞在 Read 上的 goroutine 无法被打断，唯一解锁手段是关闭 body：定时器超时即 Close，
// 当前 Read 立刻以错误返回，由本类型替换为 ErrStreamIdleTimeout。每次成功 Read 重置定时器。
type IdleTimeoutReader struct {
	rc       io.ReadCloser
	timeout  time.Duration
	timer    *time.Timer
	timedOut atomic.Bool
	stopOnce sync.Once
}

// NewIdleTimeoutReader 包装 rc；timeout<=0 时不启用超时（纯透传）。
// 调用方须在流结束后调用 Stop 释放定时器（defer 即可）。
func NewIdleTimeoutReader(rc io.ReadCloser, timeout time.Duration) *IdleTimeoutReader {
	r := &IdleTimeoutReader{rc: rc, timeout: timeout}
	if timeout > 0 {
		r.timer = time.AfterFunc(timeout, r.onTimeout)
	}
	return r
}

func (r *IdleTimeoutReader) onTimeout() {
	r.timedOut.Store(true)
	// 关闭 body 解锁阻塞中的 Read；http 响应体允许并发 Close
	_ = r.rc.Close()
}

// Read 实现 io.Reader。超时后无论底层返回什么都以 ErrStreamIdleTimeout 结束（底层此时
// 多半是 "read on closed body"，对调用方无诊断价值）；已读到的字节仍如实返回，不丢数据。
func (r *IdleTimeoutReader) Read(p []byte) (int, error) {
	n, err := r.rc.Read(p)
	if r.timedOut.Load() {
		return n, fmt.Errorf("%w: no data received for %v", ErrStreamIdleTimeout, r.timeout)
	}
	if err == nil && r.timer != nil {
		r.timer.Reset(r.timeout)
	}
	return n, err
}

// Stop 停止空闲定时器。流正常结束后必须调用，否则定时器稍后触发会关闭一个已归还
// 连接池的 body（虽无害但多余）。
func (r *IdleTimeoutReader) Stop() {
	r.stopOnce.Do(func() {
		if r.timer != nil {
			r.timer.Stop()
		}
	})
}

// TimedOut 是否已因空闲超时关闭上游。
func (r *IdleTimeoutReader) TimedOut() bool {
	return r.timedOut.Load()
}
