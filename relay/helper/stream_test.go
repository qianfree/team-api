package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSetEventStreamHeaders_Idempotency 测试 SSE 头幂等性保护
func TestSetEventStreamHeaders_Idempotency(t *testing.T) {
	tests := []struct {
		name           string
		useSafeWriter  bool
		callCount      int
		validateHeader func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:          "SafeWriter 多次调用幂等性保护",
			useSafeWriter: true,
			callCount:     3,
			validateHeader: func(t *testing.T, w *httptest.ResponseRecorder) {
				if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
					t.Errorf("Content-Type = %v, want text/event-stream", ct)
				}
				if te := w.Header().Get("Transfer-Encoding"); te != "chunked" {
					t.Errorf("Transfer-Encoding = %v, want chunked", te)
				}
				if xab := w.Header().Get("X-Accel-Buffering"); xab != "no" {
					t.Errorf("X-Accel-Buffering = %v, want no", xab)
				}
			},
		},
		{
			name:          "原生 ResponseWriter 单次调用正常工作",
			useSafeWriter: false,
			callCount:     1,
			validateHeader: func(t *testing.T, w *httptest.ResponseRecorder) {
				if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
					t.Errorf("Content-Type = %v, want text/event-stream", ct)
				}
				if te := w.Header().Get("Transfer-Encoding"); te != "chunked" {
					t.Errorf("Transfer-Encoding = %v, want chunked", te)
				}
			},
		},
		{
			name:          "原生 ResponseWriter 多次调用（httptest.ResponseRecorder 自带保护）",
			useSafeWriter: false,
			callCount:     2,
			validateHeader: func(t *testing.T, w *httptest.ResponseRecorder) {
				// httptest.ResponseRecorder 本身有幂等性保护，验证头仍然正确
				if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
					t.Errorf("Content-Type = %v, want text/event-stream", ct)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			var writer http.ResponseWriter = w

			if tt.useSafeWriter {
				writer = NewSafeWriter(w)
			}

			// 多次调用 SetEventStreamHeaders
			for i := 0; i < tt.callCount; i++ {
				SetEventStreamHeaders(writer)
			}

			tt.validateHeader(t, w)
		})
	}
}

// TestSetEventStreamHeaders_AllHeaders 测试所有必要的 SSE 头都被正确设置
func TestSetEventStreamHeaders_AllHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	sw := NewSafeWriter(w)

	SetEventStreamHeaders(sw)

	expectedHeaders := map[string]string{
		"Content-Type":      "text/event-stream",
		"Cache-Control":     "no-cache",
		"Connection":        "keep-alive",
		"Transfer-Encoding": "chunked",
		"X-Accel-Buffering": "no",
	}

	for key, expectedValue := range expectedHeaders {
		actualValue := w.Header().Get(key)
		if actualValue != expectedValue {
			t.Errorf("Header %s = %v, want %v", key, actualValue, expectedValue)
		}
	}

	// 验证状态码
	if w.Code != http.StatusOK {
		t.Errorf("StatusCode = %v, want %v", w.Code, http.StatusOK)
	}
}

// TestSafeWriter_ConcurrentSafety 测试 SafeWriter 的并发安全性
func TestSafeWriter_ConcurrentSafety(t *testing.T) {
	w := httptest.NewRecorder()
	sw := NewSafeWriter(w)

	SetEventStreamHeaders(sw)

	// 并发写入 SSE 数据
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				_ = WriteSSEData(sw, "test data")
			}
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证没有 panic 且数据写入成功
	body := w.Body.String()
	if !strings.Contains(body, "data: test data") {
		t.Error("Expected SSE data not found in response body")
	}
}

// TestWriteSSEData 测试 SSE 数据写入
func TestWriteSSEData(t *testing.T) {
	w := httptest.NewRecorder()
	sw := NewSafeWriter(w)

	SetEventStreamHeaders(sw)

	testData := "Hello, SSE!"
	err := WriteSSEData(sw, testData)
	if err != nil {
		t.Fatalf("WriteSSEData failed: %v", err)
	}

	body := w.Body.String()
	expected := "data: " + testData + "\n\n"
	if body != expected {
		t.Errorf("Body = %q, want %q", body, expected)
	}
}

// TestWriteSSEEvent 测试带事件名的 SSE 写入
func TestWriteSSEEvent(t *testing.T) {
	w := httptest.NewRecorder()
	sw := NewSafeWriter(w)

	SetEventStreamHeaders(sw)

	err := WriteSSEEvent(sw, "message", `{"content":"test"}`)
	if err != nil {
		t.Fatalf("WriteSSEEvent failed: %v", err)
	}

	body := w.Body.String()
	expected := "event: message\ndata: {\"content\":\"test\"}\n\n"
	if body != expected {
		t.Errorf("Body = %q, want %q", body, expected)
	}
}

// TestWriteSSEPing 测试 SSE 保活注释
func TestWriteSSEPing(t *testing.T) {
	w := httptest.NewRecorder()
	sw := NewSafeWriter(w)

	SetEventStreamHeaders(sw)

	err := WriteSSEPing(sw)
	if err != nil {
		t.Fatalf("WriteSSEPing failed: %v", err)
	}

	body := w.Body.String()
	expected := ": PING\n\n"
	if body != expected {
		t.Errorf("Body = %q, want %q", body, expected)
	}
}

// TestExtractSSEData 测试 SSE 数据提取
func TestExtractSSEData(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantData string
		wantOk   bool
	}{
		{
			name:     "标准格式（带空格）",
			line:     "data: Hello World",
			wantData: "Hello World",
			wantOk:   true,
		},
		{
			name:     "标准格式（不带空格）",
			line:     "data:HelloWorld",
			wantData: "HelloWorld",
			wantOk:   true,
		},
		{
			name:     "JSON 数据",
			line:     `data: {"message":"test"}`,
			wantData: `{"message":"test"}`,
			wantOk:   true,
		},
		{
			name:     "非 data 行",
			line:     "event: message",
			wantData: "",
			wantOk:   false,
		},
		{
			name:     "注释行",
			line:     ": PING",
			wantData: "",
			wantOk:   false,
		},
		{
			name:     "空 data",
			line:     "data: ",
			wantData: "",
			wantOk:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, ok := ExtractSSEData(tt.line)
			if ok != tt.wantOk {
				t.Errorf("ExtractSSEData() ok = %v, want %v", ok, tt.wantOk)
			}
			if data != tt.wantData {
				t.Errorf("ExtractSSEData() data = %q, want %q", data, tt.wantData)
			}
		})
	}
}
