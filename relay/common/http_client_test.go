package common

import (
	"net/http"
	"testing"
	"time"
)

// 验证 NewPooledClient 的传输层选择：
//   - timeout > defaultResponseHeaderTimeout（180s）→ longRun 传输层（ResponseHeaderTimeout=0），
//     避免图片/音频等长耗时同步请求被 180s 头超时误杀。
//   - timeout ≤ 180s → 普通传输层（ResponseHeaderTimeout=180s），保留对短超时请求的假死保护。
// 非代理路径直接断言底层 *http.Transport 的字段，是最稳定的判定方式。

func TestNewPooledClient_LongRunDisablesResponseHeaderTimeout(t *testing.T) {
	c := NewPooledClient(600, false) // 图片生成的典型超时（GetTimeoutSeconds 下限）
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", c.Transport)
	}
	if tr.ResponseHeaderTimeout != 0 {
		t.Errorf("long-run transport: ResponseHeaderTimeout=%v, want 0 (disabled)", tr.ResponseHeaderTimeout)
	}
	if c.Timeout != 600*time.Second {
		t.Errorf("Client.Timeout=%v, want 600s", c.Timeout)
	}
}

func TestNewPooledClient_LongRunAtBoundary(t *testing.T) {
	// 恰好 181s 触发 longRun；180s 仍走普通传输层
	if c := NewPooledClient(181, false); c.Transport != longRunSharedTransport {
		t.Error("timeout=181s should select longRunSharedTransport")
	}
	if c := NewPooledClient(180, false); c.Transport != sharedTransport {
		t.Error("timeout=180s should select sharedTransport")
	}
}

func TestNewPooledClient_NormalKeepsResponseHeaderTimeout(t *testing.T) {
	c := NewPooledClient(60, false) // 普通对话默认超时
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", c.Transport)
	}
	if tr.ResponseHeaderTimeout != defaultResponseHeaderTimeout {
		t.Errorf("normal transport: ResponseHeaderTimeout=%v, want %v", tr.ResponseHeaderTimeout, defaultResponseHeaderTimeout)
	}
	if c.Timeout != 60*time.Second {
		t.Errorf("Client.Timeout=%v, want 60s", c.Timeout)
	}
}

func TestNewPooledClient_StreamIgnoresLongRunSelection(t *testing.T) {
	// 流式请求无论 timeout 多大都走 streamClient（无 Client.Timeout，由 StreamScanner 管）
	c := NewPooledClient(600, false, true)
	if c != streamClient {
		t.Error("stream request should return streamClient regardless of timeout")
	}
}
