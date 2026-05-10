package relay

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

const maxCaptureSize = 1 << 18           // 256KB total capture budget
const headLimit = maxCaptureSize * 3 / 4 // 192KB for head (first bytes)
const tailLimit = maxCaptureSize / 4     // 64KB for tail ring buffer (last bytes)

// ResponseCaptureWriter 包装 http.ResponseWriter，在写入客户端的同时捕获响应体。
// 当响应超过 headLimit 时，使用环形缓冲区持续捕获尾部数据，
// 确保流式响应末尾的 usage、finish_reason 等重要信息不会丢失。
type ResponseCaptureWriter struct {
	http.ResponseWriter
	mu           sync.Mutex
	buf          []byte // head: first headLimit bytes
	tailBuf      []byte // ring buffer: last tailLimit bytes (lazily allocated)
	tailPos      int    // next write position in tailBuf
	tailWrapped  bool   // whether tailBuf has wrapped around
	status       int
	bytesWritten atomic.Int64
}

// NewResponseCaptureWriter 创建响应体捕获 writer
func NewResponseCaptureWriter(w http.ResponseWriter) *ResponseCaptureWriter {
	return &ResponseCaptureWriter{ResponseWriter: w}
}

// WriteHeader 捕获状态码并委托给底层 writer
func (w *ResponseCaptureWriter) WriteHeader(code int) {
	w.mu.Lock()
	w.status = code
	w.mu.Unlock()
	w.ResponseWriter.WriteHeader(code)
}

// Write 写入客户端并同时捕获到 buffer（head + tail 环形缓冲区）
func (w *ResponseCaptureWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten.Add(int64(n))
	w.mu.Lock()
	if len(w.buf) < headLimit {
		remaining := headLimit - len(w.buf)
		if len(b) <= remaining {
			w.buf = append(w.buf, b...)
		} else {
			w.buf = append(w.buf, b[:remaining]...)
			w.appendToTail(b[remaining:])
		}
	} else {
		w.appendToTail(b)
	}
	w.mu.Unlock()
	return n, err
}

// appendToTail 写入环形缓冲区，始终保留最后 tailLimit 字节
func (w *ResponseCaptureWriter) appendToTail(b []byte) {
	if w.tailBuf == nil {
		w.tailBuf = make([]byte, tailLimit)
	}
	for len(b) > 0 {
		remaining := tailLimit - w.tailPos
		if len(b) <= remaining {
			copy(w.tailBuf[w.tailPos:], b)
			w.tailPos += len(b)
			break
		}
		copy(w.tailBuf[w.tailPos:], b[:remaining])
		b = b[remaining:]
		w.tailPos = 0
		w.tailWrapped = true
	}
}

// Flush 实现 http.Flusher 接口，立即将缓冲数据推送到客户端。
func (w *ResponseCaptureWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Body 返回已捕获的响应体内容。
// 短响应（<= headLimit）直接返回；长响应保留 head + truncation marker + tail。
func (w *ResponseCaptureWriter) Body() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.tailBuf == nil {
		return string(w.buf)
	}
	var sb strings.Builder
	sb.Write(w.buf)
	sb.WriteString("\n...[truncated]...\n")
	if w.tailWrapped {
		sb.Write(w.tailBuf[w.tailPos:])
		sb.Write(w.tailBuf[:w.tailPos])
	} else {
		sb.Write(w.tailBuf[:w.tailPos])
	}
	return sb.String()
}

// StatusCode 返回捕获的 HTTP 状态码
func (w *ResponseCaptureWriter) StatusCode() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		return 200
	}
	return w.status
}

// BytesWritten 返回写入客户端的总字节数
func (w *ResponseCaptureWriter) BytesWritten() int64 {
	return w.bytesWritten.Load()
}

// ResponseHeaders 返回响应头快照（调用时从底层 ResponseWriter 提取）
func (w *ResponseCaptureWriter) ResponseHeaders() map[string]string {
	headers := make(map[string]string)
	for k, vals := range w.ResponseWriter.Header() {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}
	return headers
}
