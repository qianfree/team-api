package task

import (
	"bytes"
	"net/http"
)

// memResponseWriter 是一个纯内存的 http.ResponseWriter 实现。
//
// sync_image worker 复刻 relay 管线时没有真实的客户端 HTTP 连接，
// adaptor.DoResponse 会把上游响应写入传入的 writer。使用本类型捕获响应体和状态码，
// 供 worker 后续解析（b64_json / url），而不会像 ResponseCaptureWriter 那样
// 因内嵌 nil 的真实 writer 而 panic。
type memResponseWriter struct {
	header http.Header
	buf    bytes.Buffer
	status int
}

// newMemResponseWriter 创建内存 writer，默认状态码 200（与 net/http 语义一致）。
func newMemResponseWriter() *memResponseWriter {
	return &memResponseWriter{
		header: make(http.Header),
		status: http.StatusOK,
	}
}

func (w *memResponseWriter) Header() http.Header {
	return w.header
}

func (w *memResponseWriter) WriteHeader(code int) {
	w.status = code
}

func (w *memResponseWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

// Bytes 返回已捕获的响应体。
func (w *memResponseWriter) Bytes() []byte {
	return w.buf.Bytes()
}

// StatusCode 返回捕获的状态码。
func (w *memResponseWriter) StatusCode() int {
	return w.status
}
