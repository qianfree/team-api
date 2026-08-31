package common

import (
	"errors"
	"testing"
)

// TestIsPartialStreamEnd_ReasonCoverage 流式结束原因的中断判定覆盖。
// handler_stop（写客户端失败/上游 error 主动终止）必须算部分流中断：
// 客户端断开时写失败常先于 ctx 取消被观察到（SetEndReason 为 first-writer-wins），
// 漏判会跳过中断计费兜底，按 0 token 成功结算。
func TestIsPartialStreamEnd_ReasonCoverage(t *testing.T) {
	partialReasons := []StreamEndReason{
		StreamEndReasonClientGone,
		StreamEndReasonScannerErr,
		StreamEndReasonTimeout,
		StreamEndReasonPingFail,
		StreamEndReasonHandlerStop,
	}
	for _, reason := range partialReasons {
		s := NewStreamStatus()
		s.SetEndReason(reason, errors.New("boom"))
		if !s.IsPartialStreamEnd() {
			t.Errorf("IsPartialStreamEnd(%s) = false, want true", reason)
		}
		if s.IsNormalEnd() {
			t.Errorf("IsNormalEnd(%s) = true, want false", reason)
		}
	}

	// 正常结束不算中断
	s := NewStreamStatus()
	s.SetEndReason(StreamEndReasonDone, nil)
	if s.IsPartialStreamEnd() {
		t.Error("IsPartialStreamEnd(done) = true, want false")
	}
	if !s.IsNormalEnd() {
		t.Error("IsNormalEnd(done) = false, want true")
	}
}
