package common

import (
	"sync"
)

// StreamEndReason 流式响应结束原因
type StreamEndReason string

const (
	StreamEndReasonDone        StreamEndReason = "done"
	StreamEndReasonTimeout     StreamEndReason = "timeout"
	StreamEndReasonClientGone  StreamEndReason = "client_gone"
	StreamEndReasonError       StreamEndReason = "error"
	StreamEndReasonPanic       StreamEndReason = "panic"
	StreamEndReasonHandlerStop StreamEndReason = "handler_stop" // dataHandler 调用了 Stop()
	StreamEndReasonPingFail    StreamEndReason = "ping_fail"    // ping 写入失败
	StreamEndReasonScannerErr  StreamEndReason = "scanner_err"  // bufio.Scanner 错误
	StreamEndReasonEOF         StreamEndReason = "eof"          // scanner EOF 未收到 [DONE]
)

const maxStreamErrors = 20

// StreamStatus 流式响应状态追踪
type StreamStatus struct {
	mu       sync.RWMutex
	endOnce  sync.Once
	reason   StreamEndReason
	err      error
	errors   []string
	errCount int
}

// NewStreamStatus 创建新的流式状态追踪器
func NewStreamStatus() *StreamStatus {
	return &StreamStatus{}
}

// SetEndReason 设置流结束原因（first-writer-wins，仅第一次调用生效）
func (s *StreamStatus) SetEndReason(reason StreamEndReason, err error) {
	s.endOnce.Do(func() {
		s.mu.Lock()
		s.reason = reason
		s.err = err
		s.mu.Unlock()
	})
}

// GetEndReason 获取流结束原因
func (s *StreamStatus) GetEndReason() StreamEndReason {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reason
}

// RecordError 记录软错误（流继续处理）
func (s *StreamStatus) RecordError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errCount++
	if len(s.errors) < maxStreamErrors {
		s.errors = append(s.errors, msg)
	}
}

// ErrorCount 获取错误计数
func (s *StreamStatus) ErrorCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.errCount
}

// IsPartialStreamEnd 流已开始传输但未正常结束，应按已消费 usage 部分结算。
func (s *StreamStatus) IsPartialStreamEnd() bool {
	r := s.GetEndReason()
	return r == StreamEndReasonClientGone || r == StreamEndReasonScannerErr ||
		r == StreamEndReasonTimeout || r == StreamEndReasonPingFail
}

// IsNormalEnd 是否正常结束
func (s *StreamStatus) IsNormalEnd() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reason == StreamEndReasonDone || s.reason == StreamEndReasonHandlerStop || s.reason == ""
}

// HasErrors 是否有错误
func (s *StreamStatus) HasErrors() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reason == StreamEndReasonError || s.reason == StreamEndReasonPanic || s.errCount > 0
}

// Error 获取错误信息
func (s *StreamStatus) Error() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

// Summary 返回状态摘要
func (s *StreamStatus) Summary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.err != nil {
		return string(s.reason) + ": " + s.err.Error()
	}
	return string(s.reason)
}
