package helper

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/qianfree/team-api/relay/dto"
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

// ===== 池化帧写入器 =====

// benchClaudeDelta 构造一个典型的 Claude 流式增量事件，作为基准与парity 测试的载荷
func benchClaudeDelta() *dto.ClaudeResponse {
	text := "这是一段典型长度的流式增量文本内容"
	idx := 0
	return &dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: &idx,
		Delta: &dto.ClaudeDelta{Type: "text_delta", Text: &text},
	}
}

// TestWriteSSEEventJSON_ByteParityWithMarshalFprintf 池化路径的输出必须与
// 「json.Marshal → string → WriteSSEEvent」逐字节一致（含 HTML 转义行为），
// 否则等于悄悄改了对外协议。
func TestWriteSSEEventJSON_ByteParityWithMarshalFprintf(t *testing.T) {
	payloads := []any{
		benchClaudeDelta(),
		&dto.ClaudeResponse{Type: "message_stop"},
		// HTML 字符：json.Marshal 与 json.Encoder 默认都转义，此处锁定该行为
		map[string]any{"text": `a<b>c&d`, "n": 1.5, "null": nil},
		map[string]any{"empty": ""},
	}

	for i, p := range payloads {
		old := httptest.NewRecorder()
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("payload #%d marshal failed: %v", i, err)
		}
		if err := WriteSSEEvent(old, "content_block_delta", string(data)); err != nil {
			t.Fatalf("WriteSSEEvent failed: %v", err)
		}

		pooled := httptest.NewRecorder()
		if err := WriteSSEEventJSON(pooled, "content_block_delta", p); err != nil {
			t.Fatalf("WriteSSEEventJSON failed: %v", err)
		}

		if old.Body.String() != pooled.Body.String() {
			t.Errorf("payload #%d 输出不一致:\nold:    %q\npooled: %q", i, old.Body.String(), pooled.Body.String())
		}
	}
}

// TestWriteSSEDataJSON_ByteParity data-only 帧同样要求逐字节一致。
func TestWriteSSEDataJSON_ByteParity(t *testing.T) {
	p := benchClaudeDelta()

	old := httptest.NewRecorder()
	data, _ := json.Marshal(p)
	if err := WriteSSEData(old, string(data)); err != nil {
		t.Fatalf("WriteSSEData failed: %v", err)
	}

	pooled := httptest.NewRecorder()
	if err := WriteSSEDataJSON(pooled, p); err != nil {
		t.Fatalf("WriteSSEDataJSON failed: %v", err)
	}

	if old.Body.String() != pooled.Body.String() {
		t.Errorf("输出不一致:\nold:    %q\npooled: %q", old.Body.String(), pooled.Body.String())
	}
}

// TestWriteSSEEventJSON_MarshalFailureWritesNothing 序列化失败时必须一个字节都不出网，
// 且错误可被 errors.Is 识别 —— 半截帧或空 data 帧会让客户端整条请求失败。
func TestWriteSSEEventJSON_MarshalFailureWritesNothing(t *testing.T) {
	rec := httptest.NewRecorder()

	// chan 无法被 encoding/json 序列化
	err := WriteSSEEventJSON(rec, "content_block_delta", map[string]any{"bad": make(chan int)})
	if !errors.Is(err, ErrSSEPayloadMarshal) {
		t.Fatalf("err = %v, want ErrSSEPayloadMarshal", err)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("序列化失败仍写出了 %q", rec.Body.String())
	}

	// 失败后缓冲归池，不得污染后续事件
	rec2 := httptest.NewRecorder()
	if err := WriteSSEEventJSON(rec2, "content_block_delta", benchClaudeDelta()); err != nil {
		t.Fatalf("后续写入失败: %v", err)
	}
	if !strings.HasPrefix(rec2.Body.String(), "event: content_block_delta\ndata: {") {
		t.Errorf("池被上一次失败污染: %q", rec2.Body.String())
	}
}

// TestWriteSSEEventJSON_FrameIsSingleWrite 整帧必须单次 Write 写出 ——
// 这是保活 ping 无法插进帧中间的前提。
func TestWriteSSEEventJSON_FrameIsSingleWrite(t *testing.T) {
	w := &countingWriter{}
	if err := WriteSSEEventJSON(w, "content_block_delta", benchClaudeDelta()); err != nil {
		t.Fatalf("WriteSSEEventJSON failed: %v", err)
	}
	if w.writes != 1 {
		t.Errorf("Write 调用次数 = %d, want 1（多次写出会给 ping 留出插入窗口）", w.writes)
	}
}

// countingWriter 统计 Write 调用次数
type countingWriter struct {
	header http.Header
	writes int
}

func (c *countingWriter) Header() http.Header {
	if c.header == nil {
		c.header = http.Header{}
	}
	return c.header
}
func (c *countingWriter) WriteHeader(int) {}
func (c *countingWriter) Write(p []byte) (int, error) {
	c.writes++
	return len(p), nil
}

// discardWriter 丢弃写入内容的 ResponseWriter，让基准只测序列化与拼帧开销
type discardWriter struct{ header http.Header }

func (d *discardWriter) Header() http.Header {
	if d.header == nil {
		d.header = http.Header{}
	}
	return d.header
}
func (d *discardWriter) WriteHeader(int)             {}
func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// BenchmarkSSEEvent_MarshalFprintf 旧路径：json.Marshal → string() → fmt.Fprintf
func BenchmarkSSEEvent_MarshalFprintf(b *testing.B) {
	w := &discardWriter{}
	payload := benchClaudeDelta()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(payload)
		if err != nil {
			b.Fatal(err)
		}
		if err := WriteSSEEvent(w, "content_block_delta", string(data)); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSSEEvent_PooledJSON 新路径：池化 buffer + 绑定的 json.Encoder，整帧单次 Write
func BenchmarkSSEEvent_PooledJSON(b *testing.B) {
	w := &discardWriter{}
	payload := benchClaudeDelta()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := WriteSSEEventJSON(w, "content_block_delta", payload); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSSEEvent_MarshalFprintfParallel 并发下的旧路径（真实场景是多请求同时流式输出）
func BenchmarkSSEEvent_MarshalFprintfParallel(b *testing.B) {
	payload := benchClaudeDelta()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := &discardWriter{}
		for pb.Next() {
			data, err := json.Marshal(payload)
			if err != nil {
				b.Fatal(err)
			}
			if err := WriteSSEEvent(w, "content_block_delta", string(data)); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkSSEEvent_PooledJSONParallel 并发下的新路径，验证 sync.Pool 的 per-P 缓存不成为瓶颈
func BenchmarkSSEEvent_PooledJSONParallel(b *testing.B) {
	payload := benchClaudeDelta()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := &discardWriter{}
		for pb.Next() {
			if err := WriteSSEEventJSON(w, "content_block_delta", payload); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// concurrentProbe 并发独立性探针载荷：stream/seq 定位来源，pad 让各流的帧长度不同，
// 一旦缓冲被跨流复用或 Reset 失效，长度或字段就会对不上。
type concurrentProbe struct {
	Stream int    `json:"stream"`
	Seq    int    `json:"seq"`
	Pad    string `json:"pad"`
}

// TestWriteSSEEventJSON_ConcurrentIndependence 100 条流并发输出时，池化缓冲必须完全独占：
// 每条流的响应里只能有自己的事件，顺序、内容、长度都不得被别的流污染。
//
// sync.Pool 的 Get 会把对象从池中取走，两个并发的 Get 不可能拿到同一个对象；
// 本用例锁死这一点，防止将来有人把 eb 或 eb.buf.Bytes() 的引用泄漏到 Put 之后。
func TestWriteSSEEventJSON_ConcurrentIndependence(t *testing.T) {
	const streams = 100
	const eventsPerStream = 50

	var wg sync.WaitGroup
	bodies := make([]string, streams)
	errs := make([]error, streams)

	for s := 0; s < streams; s++ {
		wg.Add(1)
		go func(s int) {
			defer wg.Done()
			// 每条流一个独立 writer，对应真实场景里每个请求各自的 ResponseWriter
			rec := httptest.NewRecorder()
			w := NewSafeWriter(rec)
			for e := 0; e < eventsPerStream; e++ {
				payload := concurrentProbe{Stream: s, Seq: e, Pad: strings.Repeat("x", s%64)}
				if err := WriteSSEEventJSON(w, "content_block_delta", payload); err != nil {
					errs[s] = err
					return
				}
			}
			bodies[s] = rec.Body.String()
		}(s)
	}
	wg.Wait()

	for s := 0; s < streams; s++ {
		if errs[s] != nil {
			t.Fatalf("stream %d 写入失败: %v", s, errs[s])
		}
		events := decodeSSE(bodies[s])
		if len(events) != eventsPerStream {
			t.Fatalf("stream %d 事件数 = %d, want %d", s, len(events), eventsPerStream)
		}
		for e, ev := range events {
			var got concurrentProbe
			if err := json.Unmarshal([]byte(ev.data), &got); err != nil {
				t.Fatalf("stream %d 事件 #%d 不是合法 JSON（缓冲被跨流复用会产生这种拼接残片）: %v\ndata=%q",
					s, e, err, ev.data)
			}
			if got.Stream != s || got.Seq != e || len(got.Pad) != s%64 {
				t.Fatalf("stream %d 事件 #%d 内容串流: got %+v, want stream=%d seq=%d padLen=%d",
					s, e, got, s, e, s%64)
			}
		}
	}
}

// TestWriteSSEEventJSON_ConcurrentWithPing 并发输出叠加保活 ping：
// 帧在私有缓冲里拼好后单次写出，ping 只能落在帧边界，客户端不会拿到空 data 事件。
func TestWriteSSEEventJSON_ConcurrentWithPing(t *testing.T) {
	const streams = 32
	const eventsPerStream = 30

	var wg sync.WaitGroup
	bodies := make([]string, streams)

	for s := 0; s < streams; s++ {
		wg.Add(1)
		go func(s int) {
			defer wg.Done()
			rec := httptest.NewRecorder()
			w := NewSafeWriter(rec)

			stopPing := make(chan struct{})
			pingDone := make(chan struct{})
			go func() {
				defer close(pingDone)
				for {
					select {
					case <-stopPing:
						return
					default:
						_ = WriteSSEPing(w)
					}
				}
			}()

			for e := 0; e < eventsPerStream; e++ {
				_ = WriteSSEEventJSON(w, "content_block_delta", concurrentProbe{Stream: s, Seq: e})
			}

			close(stopPing)
			<-pingDone
			bodies[s] = rec.Body.String()
		}(s)
	}
	wg.Wait()

	for s := 0; s < streams; s++ {
		assertNoEmptyDataEvent(t, bodies[s])
	}
}

// TestWriteSSEEventJSON_OversizedPayload 单个事件远超池缓冲的初始容量与回收阈值时，
// 必须照常完整写出（bytes.Buffer 自动扩容，不截断、不报错）；超阈值的缓冲不回池，
// 但不影响后续事件的正确性。
func TestWriteSSEEventJSON_OversizedPayload(t *testing.T) {
	// 1 MiB 文本，远超 sseEventBufInitCap(2KiB) 与 sseEventBufKeepCap(64KiB)
	huge := strings.Repeat("你好", 512*1024/6)
	payload := concurrentProbe{Stream: 1, Seq: 2, Pad: huge}

	rec := httptest.NewRecorder()
	if err := WriteSSEEventJSON(rec, "content_block_delta", payload); err != nil {
		t.Fatalf("超大事件写出失败: %v", err)
	}

	// 与旧路径逐字节一致（即：既没截断也没串位）
	want := httptest.NewRecorder()
	data, _ := json.Marshal(payload)
	_ = WriteSSEEvent(want, "content_block_delta", string(data))
	if rec.Body.String() != want.Body.String() {
		t.Fatalf("超大事件输出与 Marshal 路径不一致（len got=%d want=%d）",
			rec.Body.Len(), want.Body.Len())
	}

	// 超阈值缓冲被丢弃后，后续事件仍然正确（下一次 Get 会新建）
	next := httptest.NewRecorder()
	if err := WriteSSEEventJSON(next, "content_block_delta", concurrentProbe{Stream: 3, Seq: 4}); err != nil {
		t.Fatalf("后续事件写出失败: %v", err)
	}
	events := decodeSSE(next.Body.String())
	if len(events) != 1 {
		t.Fatalf("后续事件数 = %d, want 1", len(events))
	}
	var got concurrentProbe
	if err := json.Unmarshal([]byte(events[0].data), &got); err != nil {
		t.Fatalf("后续事件不是合法 JSON: %v", err)
	}
	if got.Stream != 3 || got.Seq != 4 || got.Pad != "" {
		t.Errorf("后续事件被超大缓冲污染: %+v", got)
	}
}

// TestWriteSSEEventJSON_RealWorldFrameSizes 记录真实流式事件的实际帧长度，
// 用于校准 sseEventBufInitCap（初始容量应覆盖绝大多数事件，避免首次写入扩容）。
func TestWriteSSEEventJSON_RealWorldFrameSizes(t *testing.T) {
	text := "这是一段典型长度的流式增量文本内容"
	idx := 0
	cases := []struct {
		name    string
		event   string
		payload any
	}{
		{"content_block_delta(文本)", "content_block_delta", &dto.ClaudeResponse{
			Type: "content_block_delta", Index: &idx,
			Delta: &dto.ClaudeDelta{Type: "text_delta", Text: &text},
		}},
		{"message_start", "message_start", &dto.ClaudeResponse{
			Type: "message_start",
			Message: &dto.ClaudeMessageInfo{
				ID: "msg_01ABCDEFGHIJKLMNOPQRSTUV", Type: "message", Role: "assistant",
				Model: "claude-opus-4-8", Content: []dto.ClaudeContentBlock{},
				Usage: &dto.ClaudeUsage{InputTokens: 1234, OutputTokens: 0, CacheReadInputTokens: 5678},
			},
		}},
		{"message_delta(含 usage)", "message_delta", &dto.ClaudeResponse{
			Type:  "message_delta",
			Delta: &dto.ClaudeDelta{StopReason: &text},
			Usage: &dto.ClaudeUsage{InputTokens: 1234, OutputTokens: 567},
		}},
		{"message_stop", "message_stop", &dto.ClaudeResponse{Type: "message_stop"}},
	}

	for _, c := range cases {
		rec := httptest.NewRecorder()
		if err := WriteSSEEventJSON(rec, c.event, c.payload); err != nil {
			t.Fatalf("%s 写出失败: %v", c.name, err)
		}
		n := rec.Body.Len()
		t.Logf("%-28s 整帧 %4d 字节（初始容量 %d）", c.name, n, sseEventBufInitCap)
		if n > sseEventBufInitCap {
			t.Errorf("%s 帧长 %d 超过初始容量 %d，每个事件都会触发扩容，应上调 sseEventBufInitCap",
				c.name, n, sseEventBufInitCap)
		}
	}
}

// BenchmarkSSEData_PooledJSON_Pointer / _Value 验证「传指针而非结构体值」的理由：
// 结构体值传进 any 形参需要装箱，会额外堆分配一份拷贝；传指针则不会。
// 迁移调用点时据此统一传 &chunk。
func BenchmarkSSEData_PooledJSON_Pointer(b *testing.B) {
	w := &discardWriter{}
	payload := benchClaudeDelta() // 已是指针
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := WriteSSEDataJSON(w, payload); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSSEData_PooledJSON_Value(b *testing.B) {
	w := &discardWriter{}
	payload := *benchClaudeDelta() // 解引用成值
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := WriteSSEDataJSON(w, payload); err != nil {
			b.Fatal(err)
		}
	}
}

// TestSSEFrameWriter_MaxBufDegradeSuppressesPing 超大帧降级为分次写出时，写出的这批字节
// 不含帧尾 —— 此刻放行 ping 依然会把帧劈成两个事件。降级期间必须抑制 ping，直到分隔空行到达。
func TestSSEFrameWriter_MaxBufDegradeSuppressesPing(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := NewSafeWriter(rec)
	frame := NewSSEFrameWriter(sw)

	if err := frame.WriteLine("event: content_block_delta\n"); err != nil {
		t.Fatalf("WriteLine failed: %v", err)
	}
	// 帧尚未降级，ping 可以正常写（半帧还在缓冲里，没有出网风险）
	if written, err := sw.WritePing(); err != nil || !written {
		t.Fatalf("未降级时 ping 应正常写出: written=%v err=%v", written, err)
	}

	line := "data: " + strings.Repeat("x", 64*1024) + "\n"
	for i := 0; i < sseFrameMaxBuf/len(line)+2; i++ {
		if err := frame.WriteLine(line); err != nil {
			t.Fatalf("WriteLine failed: %v", err)
		}
	}
	if rec.Body.Len() == 0 {
		t.Fatal("超过上限后仍未降级写出，无法验证抑制逻辑")
	}

	// 降级已发生：帧的一部分已经出网且没有帧尾，此刻 ping 必须让路
	written, err := sw.WritePing()
	if err != nil {
		t.Fatalf("WritePing failed: %v", err)
	}
	if written {
		t.Error("超大帧降级写出期间 ping 未被抑制，空行会把帧劈成两个事件")
	}

	// 帧结束后恢复：分隔空行到达，ping 重新放行
	if err := frame.WriteLine("\n"); err != nil {
		t.Fatalf("WriteLine failed: %v", err)
	}
	written, err = sw.WritePing()
	if err != nil {
		t.Fatalf("WritePing failed: %v", err)
	}
	if !written {
		t.Error("帧已完整写出，ping 应恢复")
	}

	assertNoEmptyDataEvent(t, rec.Body.String())
}

// TestSSEFrameWriter_DiscardClearsPingSuppression 降级后走中断路径（Discard）时，
// 必须解除 ping 抑制，否则标记会永久留在 writer 上、后续保活全部失效。
func TestSSEFrameWriter_DiscardClearsPingSuppression(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := NewSafeWriter(rec)
	frame := NewSSEFrameWriter(sw)

	line := "data: " + strings.Repeat("x", 64*1024) + "\n"
	for i := 0; i < sseFrameMaxBuf/len(line)+2; i++ {
		if err := frame.WriteLine(line); err != nil {
			t.Fatalf("WriteLine failed: %v", err)
		}
	}
	if written, _ := sw.WritePing(); written {
		t.Fatal("前置条件不成立：降级期间 ping 应被抑制")
	}

	frame.Discard()
	if written, err := sw.WritePing(); err != nil || !written {
		t.Errorf("Discard 后 ping 抑制未解除: written=%v err=%v", written, err)
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
