package helper

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
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

// sseEvent 一个被客户端派发出去的 SSE 事件
type sseEvent struct {
	event string
	data  string
}

// decodeSSE 按 SSE 规范模拟客户端解码：`:` 开头为注释忽略，空行即派发当前事件。
// 用于断言网关吐出的字节在客户端眼里是什么，而不是只比对原始字符串。
func decodeSSE(raw string) []sseEvent {
	var events []sseEvent
	cur := sseEvent{}
	var dataLines []string
	dispatch := func() {
		if cur.event == "" && len(dataLines) == 0 {
			return
		}
		cur.data = strings.Join(dataLines, "\n")
		events = append(events, cur)
		cur = sseEvent{}
		dataLines = nil
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSuffix(line, "\r")
		switch {
		case line == "":
			dispatch()
		case strings.HasPrefix(line, ":"):
			// 注释（保活 ping），忽略
		case strings.HasPrefix(line, "event:"):
			cur.event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	dispatch()
	return events
}

// assertNoEmptyDataEvent 断言没有「有事件名但 data 为空」的事件 —— 这正是客户端
// 对空串做 JSON.parse 报 "JSON Parse error: Unexpected EOF" 并中止请求的成因。
func assertNoEmptyDataEvent(t *testing.T, raw string) {
	t.Helper()
	for i, ev := range decodeSSE(raw) {
		if ev.event != "" && ev.data == "" {
			t.Errorf("事件 #%d (event=%q) 的 data 为空，客户端 JSON.parse 会失败；原始输出:\n%s",
				i, ev.event, raw)
		}
	}
}

// TestSafeWriter_LineByLineWriteIsSplitByPing 对照用例：逐行写 SSE 时，
// SafeWriter 挡不住保活 ping 把自带空行插进帧中间 —— 半帧被提前派发，
// 客户端拿到 data 为空的事件。这是 SSEFrameWriter 存在的理由，也证明
// assertNoEmptyDataEvent 确实能抓到该缺陷。
func TestSafeWriter_LineByLineWriteIsSplitByPing(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := NewSafeWriter(rec)

	_, _ = sw.Write([]byte("event: content_block_delta\n"))
	// 主循环阻塞在读上游下一行时，ping goroutine 抢到了写入
	_ = WriteSSEPing(sw)
	_, _ = sw.Write([]byte(`data: {"type":"content_block_delta"}` + "\n"))
	_, _ = sw.Write([]byte("\n"))

	events := decodeSSE(rec.Body.String())
	if len(events) == 0 || events[0].data != "" {
		t.Fatalf("对照用例失效：期望逐行写会派发出空 data 事件，实际 events=%+v", events)
	}
}

// TestSSEFrameWriter_PingCannotSplitFrame 回归：改用 SSEFrameWriter 后，
// 帧攒齐才出网，ping 只能落在帧边界，客户端不会再收到空 data 事件。
func TestSSEFrameWriter_PingCannotSplitFrame(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := NewSafeWriter(rec)
	frame := NewSSEFrameWriter(sw)

	if err := frame.WriteLine("event: content_block_delta\n"); err != nil {
		t.Fatalf("WriteLine failed: %v", err)
	}
	// 半帧必须还留在缓冲里，一个字节都不能出网
	if rec.Body.Len() != 0 {
		t.Errorf("半帧提前出网: %q", rec.Body.String())
	}
	if frame.Buffered() == 0 {
		t.Error("Buffered() = 0, 期望半帧仍在缓冲中")
	}

	// ping goroutine 此刻抢到写入
	if err := WriteSSEPing(sw); err != nil {
		t.Fatalf("WriteSSEPing failed: %v", err)
	}

	if err := frame.WriteLine(`data: {"type":"content_block_delta"}` + "\n"); err != nil {
		t.Fatalf("WriteLine failed: %v", err)
	}
	if err := frame.WriteLine("\n"); err != nil {
		t.Fatalf("WriteLine failed: %v", err)
	}

	body := rec.Body.String()
	assertNoEmptyDataEvent(t, body)

	events := decodeSSE(body)
	if len(events) != 1 {
		t.Fatalf("派发事件数 = %d, want 1; body=%q", len(events), body)
	}
	if events[0].event != "content_block_delta" || events[0].data != `{"type":"content_block_delta"}` {
		t.Errorf("事件 = %+v, 与写入内容不符", events[0])
	}
	// ping 必须整体落在帧之前，不能夹在帧内部
	if !strings.HasPrefix(body, ": PING\n\n") {
		t.Errorf("ping 未落在帧边界: %q", body)
	}
}

// TestSSEFrameWriter_ConcurrentPingDuringFrames 并发压力下的帧完整性：
// 主循环持续逐行写帧，ping goroutine 同时高频写入，客户端解码后不得出现空 data 事件。
func TestSSEFrameWriter_ConcurrentPingDuringFrames(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := NewSafeWriter(rec)
	frame := NewSSEFrameWriter(sw)

	stopPing := make(chan struct{})
	pingDone := make(chan struct{})
	go func() {
		defer close(pingDone)
		for {
			select {
			case <-stopPing:
				return
			default:
				_ = WriteSSEPing(sw)
			}
		}
	}()

	for i := 0; i < 500; i++ {
		if err := frame.WriteLine("event: content_block_delta\n"); err != nil {
			t.Fatalf("WriteLine failed: %v", err)
		}
		if err := frame.WriteLine(`data: {"i":` + strconv.Itoa(i) + "}\n"); err != nil {
			t.Fatalf("WriteLine failed: %v", err)
		}
		if err := frame.WriteLine("\n"); err != nil {
			t.Fatalf("WriteLine failed: %v", err)
		}
	}

	close(stopPing)
	<-pingDone

	body := rec.Body.String()
	assertNoEmptyDataEvent(t, body)

	events := decodeSSE(body)
	if len(events) != 500 {
		t.Fatalf("派发事件数 = %d, want 500", len(events))
	}
	for i, ev := range events {
		if want := `{"i":` + strconv.Itoa(i) + "}"; ev.data != want {
			t.Fatalf("事件 #%d data = %q, want %q（帧内容错位）", i, ev.data, want)
		}
	}
}

// TestSSEFrameWriter_CRLFBoundary 上游用 CRLF 行尾时同样按帧冲刷。
// 旧实现只认 len(line)==1 && line[0]=='\n'，CRLF 上游会导致整条流从不 Flush。
func TestSSEFrameWriter_CRLFBoundary(t *testing.T) {
	rec := httptest.NewRecorder()
	frame := NewSSEFrameWriter(NewSafeWriter(rec))

	_ = frame.WriteLine("event: ping\r\n")
	if rec.Body.Len() != 0 {
		t.Errorf("半帧提前出网: %q", rec.Body.String())
	}
	_ = frame.WriteLine("data: {}\r\n")
	_ = frame.WriteLine("\r\n")

	if frame.Buffered() != 0 {
		t.Errorf("CRLF 空行未被识别为帧边界，仍有 %d 字节滞留", frame.Buffered())
	}
	if got := rec.Body.String(); got != "event: ping\r\ndata: {}\r\n\r\n" {
		t.Errorf("Body = %q", got)
	}
}

// TestSSEFrameWriter_FlushPartial 上游未以空行结尾时，残留必须冲刷，不能丢帧。
func TestSSEFrameWriter_FlushPartial(t *testing.T) {
	rec := httptest.NewRecorder()
	frame := NewSSEFrameWriter(NewSafeWriter(rec))

	_ = frame.WriteLine("data: {\"last\":true}\n")
	if rec.Body.Len() != 0 {
		t.Fatalf("无空行结尾的帧不应提前出网: %q", rec.Body.String())
	}
	if err := frame.FlushPartial(); err != nil {
		t.Fatalf("FlushPartial failed: %v", err)
	}
	if got := rec.Body.String(); got != "data: {\"last\":true}\n" {
		t.Errorf("Body = %q, 最后一帧丢失", got)
	}
	// 空缓冲重复调用应为 no-op
	if err := frame.FlushPartial(); err != nil {
		t.Errorf("空缓冲 FlushPartial 应为 no-op, got %v", err)
	}
}

// TestSSEFrameWriter_Discard 中断时丢弃半帧，不把不完整的帧写给客户端。
func TestSSEFrameWriter_Discard(t *testing.T) {
	rec := httptest.NewRecorder()
	frame := NewSSEFrameWriter(NewSafeWriter(rec))

	_ = frame.WriteLine("event: content_block_delta\n")
	frame.Discard()
	if frame.Buffered() != 0 {
		t.Errorf("Discard 后 Buffered() = %d, want 0", frame.Buffered())
	}
	if err := frame.FlushPartial(); err != nil {
		t.Fatalf("FlushPartial failed: %v", err)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("被丢弃的半帧仍然出网: %q", rec.Body.String())
	}
}

// TestSSEFrameWriter_MaxBufDegrades 上游异常（始终不发空行）时按上限降级冲刷，
// 不把整条流攒进内存。
func TestSSEFrameWriter_MaxBufDegrades(t *testing.T) {
	rec := httptest.NewRecorder()
	frame := NewSSEFrameWriter(NewSafeWriter(rec))

	line := "data: " + strings.Repeat("x", 64*1024) + "\n"
	for i := 0; i < sseFrameMaxBuf/len(line)+2; i++ {
		if err := frame.WriteLine(line); err != nil {
			t.Fatalf("WriteLine failed: %v", err)
		}
	}
	if frame.Buffered() >= sseFrameMaxBuf {
		t.Errorf("Buffered() = %d, 超过上限仍未降级冲刷", frame.Buffered())
	}
	if rec.Body.Len() == 0 {
		t.Error("超过上限后仍未向客户端写出任何数据")
	}
}

// TestSSEFrameWriter_WriteErrorPropagates 写客户端失败时错误必须上报，
// 供调用方走流中断结算（而非静默丢数据按成功结算）。
func TestSSEFrameWriter_WriteErrorPropagates(t *testing.T) {
	fw := &failingWriter{err: errors.New("connection reset by peer")}
	frame := NewSSEFrameWriter(fw)

	if err := frame.WriteLine("event: x\n"); err != nil {
		t.Fatalf("半帧不触发写入，不应报错: %v", err)
	}
	err := frame.WriteLine("\n")
	if err == nil || !strings.Contains(err.Error(), "connection reset") {
		t.Fatalf("WriteLine 帧结束时错误未上报: %v", err)
	}
	// 失败后缓冲已清空，不会重复写出
	if frame.Buffered() != 0 {
		t.Errorf("写失败后 Buffered() = %d, want 0", frame.Buffered())
	}
}

// failingWriter 写入必定失败的 ResponseWriter，用于验证错误传播
type failingWriter struct {
	err error
}

func (f *failingWriter) Header() http.Header       { return http.Header{} }
func (f *failingWriter) WriteHeader(int)           {}
func (f *failingWriter) Write([]byte) (int, error) { return 0, f.err }

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
