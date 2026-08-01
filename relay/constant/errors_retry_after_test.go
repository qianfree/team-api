package constant

import (
	"net/http"
	"testing"
	"time"
)

func TestRetryAfterFromHeader(t *testing.T) {
	mk := func(v string) http.Header {
		h := http.Header{}
		if v != "" {
			h.Set("Retry-After", v)
		}
		return h
	}

	tests := []struct {
		name string
		val  string
		want time.Duration
	}{
		{"缺失", "", 0},
		{"秒数", "30", 30 * time.Second},
		{"秒数带空格", "  5 ", 5 * time.Second},
		{"零", "0", 0},
		{"负数", "-3", 0},
		{"非法", "soon", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RetryAfterFromHeader(mk(tt.val)); got != tt.want {
				t.Errorf("RetryAfterFromHeader(%q) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}

	// HTTP-date：未来时间 → 正时长；过去时间 → 0
	future := time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)
	if got := RetryAfterFromHeader(mk(future)); got < 80*time.Second || got > 90*time.Second {
		t.Errorf("HTTP-date 未来时间解析 = %v, want ~90s", got)
	}
	past := time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat)
	if got := RetryAfterFromHeader(mk(past)); got != 0 {
		t.Errorf("HTTP-date 过去时间解析 = %v, want 0", got)
	}
}

func TestWithRetryAfter(t *testing.T) {
	e := NewUpstreamError(429, "rate limited", nil).WithRetryAfter(2 * time.Second)
	if e.RetryAfter != 2*time.Second {
		t.Errorf("RetryAfter = %v, want 2s", e.RetryAfter)
	}
	if e.StatusCode != 429 {
		t.Errorf("链式调用不得改变其它字段")
	}

	// d<=0 不生效
	e2 := NewUpstreamError(429, "x", nil).WithRetryAfter(0)
	if e2.RetryAfter != 0 {
		t.Errorf("d=0 不应写入")
	}

	// nil 接收者安全
	var nilErr *RelayError
	if nilErr.WithRetryAfter(time.Second) != nil {
		t.Errorf("nil 接收者应原样返回 nil")
	}
}
