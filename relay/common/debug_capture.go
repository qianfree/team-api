package common

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// 渠道调试日志传输层捕获：仅在渠道调试开关开启时，经 ctx 携带捕获器镜像
// 「系统→远程」「远程→系统」两段报文（最终形态，含协议转换与 header override 后的结果）。
// 开关关闭的请求 ctx 中无捕获器，DebugRoundTripper 直接透传，热路径零开销。

type debugAttemptCtxKey struct{}

// WithDebugAttempt 将本次尝试的捕获器挂入 ctx（作用域仅限当前 attempt，勿跨重试轮次复用）
func WithDebugAttempt(ctx context.Context, a *DebugAttemptCapture) context.Context {
	return context.WithValue(ctx, debugAttemptCtxKey{}, a)
}

// DebugAttemptFromContext 取出当前 ctx 的捕获器；未开启调试时返回 nil
func DebugAttemptFromContext(ctx context.Context) *DebugAttemptCapture {
	if ctx == nil {
		return nil
	}
	a, _ := ctx.Value(debugAttemptCtxKey{}).(*DebugAttemptCapture)
	return a
}

// DebugAttemptCapture 单次上游尝试的传输层捕获器。tee 完整累积请求/响应字节，不截断
// （调试用途，可能含 base64 图片；靠管理端提示及时清理控制数据量）。
type DebugAttemptCapture struct {
	mu sync.Mutex

	upstreamMethod      string
	upstreamURL         string // query 中凭证参数已脱敏
	upstreamReqHeaders  map[string]string
	upstreamRespHeaders map[string]string

	upstreamReqBody  []byte
	upstreamRespBody []byte

	upstreamStatus    int
	upstreamError     string
	upstreamLatencyMs int64
}

// captureUpstreamRequest 快照最终上游请求并替换 Body 为 tee（在 RoundTrip 内调用）
func (a *DebugAttemptCapture) captureUpstreamRequest(req *http.Request) {
	a.mu.Lock()
	a.upstreamMethod = req.Method
	a.upstreamURL = MaskURLSecret(req.URL.String())
	a.upstreamReqHeaders = MaskHeaders(req.Header)
	a.mu.Unlock()
	if req.Body != nil {
		req.Body = newDebugTee(req.Body, a, &a.upstreamReqBody)
	}
}

// captureUpstreamResponse 快照上游响应并替换 Body 为 tee
func (a *DebugAttemptCapture) captureUpstreamResponse(resp *http.Response) {
	a.mu.Lock()
	a.upstreamStatus = resp.StatusCode
	a.upstreamRespHeaders = MaskHeaders(resp.Header)
	a.mu.Unlock()
	if resp.Body != nil {
		resp.Body = newDebugTee(resp.Body, a, &a.upstreamRespBody)
	}
}

func (a *DebugAttemptCapture) captureError(err error) {
	a.mu.Lock()
	a.upstreamError = err.Error()
	a.mu.Unlock()
}

func (a *DebugAttemptCapture) setLatency(d time.Duration) {
	a.mu.Lock()
	a.upstreamLatencyMs = d.Milliseconds()
	a.mu.Unlock()
}

// upstreamSnapshot 捕获数据快照（构建落库记录时调用一次；body 切片引用捕获缓冲，此后不再写入）
type upstreamSnapshot struct {
	Method      string
	URL         string
	ReqHeaders  map[string]string
	RespHeaders map[string]string
	ReqBody     []byte
	RespBody    []byte
	Status      int
	Err         string
	LatencyMs   int64
}

func (a *DebugAttemptCapture) snapshot() upstreamSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	return upstreamSnapshot{
		Method:      a.upstreamMethod,
		URL:         a.upstreamURL,
		ReqHeaders:  a.upstreamReqHeaders,
		RespHeaders: a.upstreamRespHeaders,
		ReqBody:     a.upstreamReqBody,
		RespBody:    a.upstreamRespBody,
		Status:      a.upstreamStatus,
		Err:         a.upstreamError,
		LatencyMs:   a.upstreamLatencyMs,
	}
}

// debugTeeReadCloser 透传读写并在读取时镜像字节到捕获缓冲
type debugTeeReadCloser struct {
	src   io.ReadCloser
	a     *DebugAttemptCapture
	field *[]byte
}

func newDebugTee(src io.ReadCloser, a *DebugAttemptCapture, field *[]byte) *debugTeeReadCloser {
	return &debugTeeReadCloser{src: src, a: a, field: field}
}

func (t *debugTeeReadCloser) Read(p []byte) (int, error) {
	n, err := t.src.Read(p)
	if n > 0 {
		t.a.mu.Lock()
		*t.field = append(*t.field, p[:n]...)
		t.a.mu.Unlock()
	}
	return n, err
}

func (t *debugTeeReadCloser) Close() error {
	return t.src.Close()
}

// DebugRoundTripper 渠道调试日志传输层包装。ctx 无捕获器时直接透传（一次 ctx 查找）；
// 有捕获器时镜像最终上游请求/响应。连接池与 HTTP/2 仍由底层 *http.Transport 管理。
type DebugRoundTripper struct {
	Base http.RoundTripper
}

// WrapDebugTransport 包装传输层供 http.Client 的 Transport 字段使用
func WrapDebugTransport(base *http.Transport) http.RoundTripper {
	return &DebugRoundTripper{Base: base}
}

func (t *DebugRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	a := DebugAttemptFromContext(req.Context())
	if a == nil {
		return t.Base.RoundTrip(req)
	}
	// Clone 后再替换 Body：不污染适配器持有的原始 req（适配器在 client.Do 返回后会
	// Close httpReq.Body，tee 的 Close 透传保持原有语义）。
	// 已知边角：底层 Transport 建连失败内部重放走 GetBody（重读原始 body），会绕过 tee，
	// 该次捕获的段2 可能不完整——罕见且不影响请求本身，可接受。
	clone := req.Clone(req.Context())
	a.captureUpstreamRequest(clone)
	start := time.Now()
	resp, err := t.Base.RoundTrip(clone)
	a.setLatency(time.Since(start))
	if err != nil {
		a.captureError(err)
		return resp, err
	}
	a.captureUpstreamResponse(resp)
	return resp, nil
}

// DebugClientWriter 渠道调试日志的客户端响应捕获 writer：完整累积、不截断
// （区别于审计的 ResponseCaptureWriter，后者有 256KB head+tail 截断）。
// 仅在调试开关开启时由 RelayHandler 包在 rc.Writer 外层，请求结束后由
// DebugSession.FinalizeAndSubmit 取走数据。
type DebugClientWriter struct {
	http.ResponseWriter
	mu     sync.Mutex
	buf    []byte
	status int
}

// NewDebugClientWriter 创建客户端响应捕获 writer
func NewDebugClientWriter(w http.ResponseWriter) *DebugClientWriter {
	return &DebugClientWriter{ResponseWriter: w}
}

// WriteHeader 捕获状态码并委托底层 writer（仅首次生效）
func (w *DebugClientWriter) WriteHeader(code int) {
	w.mu.Lock()
	if w.status != 0 {
		w.mu.Unlock()
		return
	}
	w.status = code
	w.mu.Unlock()
	w.ResponseWriter.WriteHeader(code)
}

// Write 委托底层 writer 并完整镜像实际写出的字节
func (w *DebugClientWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.mu.Unlock()

	n, err := w.ResponseWriter.Write(b)

	w.mu.Lock()
	w.buf = append(w.buf, b[:n]...)
	w.mu.Unlock()
	return n, err
}

// Flush 实现 http.Flusher，转发到底层 writer（流式响应依赖）
func (w *DebugClientWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Bytes 返回已捕获的完整响应体
func (w *DebugClientWriter) Bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf
}

// StatusCode 返回捕获的状态码（未显式设置时按 200）
func (w *DebugClientWriter) StatusCode() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// HeaderSnapshot 从底层 writer 提取当前响应头快照（响应已写出后调用，取最终形态）
func (w *DebugClientWriter) HeaderSnapshot() map[string]string {
	headers := make(map[string]string)
	for k, vals := range w.ResponseWriter.Header() {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}
	return headers
}

// ===== 脱敏与编码 =====

// sensitiveHeaderNames 凭证类请求/响应头，落库前脱敏（保留前6后4，足以区分 Key 又不泄露完整值）
var sensitiveHeaderNames = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"cookie":              true,
	"set-cookie":          true,
	"x-api-key":           true,
	"api-key":             true,
	"x-goog-api-key":      true,
	"x-auth-token":        true,
}

// urlSecretParams URL query 中携带凭证的参数名（如 Gemini 的 ?key=）
var urlSecretParams = map[string]bool{
	"key":          true,
	"api_key":      true,
	"apikey":       true,
	"token":        true,
	"access_token": true,
}

// MaskSecret 脱敏单个凭证值：长度 < 12 全遮蔽；否则保留前 6 后 4 字符
func MaskSecret(v string) string {
	if len(v) < 12 {
		return "****"
	}
	return v[:6] + "****" + v[len(v)-4:]
}

// MaskHeaders 头快照并脱敏凭证类头；多值头用 ", " 连接
func MaskHeaders(h http.Header) map[string]string {
	if h == nil {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, vals := range h {
		v := strings.Join(vals, ", ")
		if sensitiveHeaderNames[strings.ToLower(k)] {
			v = MaskSecret(v)
		}
		out[k] = v
	}
	return out
}

// MaskURLSecret 遮蔽 URL query 中的凭证参数值
func MaskURLSecret(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.RawQuery == "" {
		return rawURL
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return rawURL
	}
	changed := false
	for name, vals := range q {
		if !urlSecretParams[strings.ToLower(name)] {
			continue
		}
		for i := range vals {
			vals[i] = MaskSecret(vals[i])
		}
		changed = true
	}
	if !changed {
		return rawURL
	}
	// Encode 会把脱敏占位符 * 百分号转义成 %2A（可读性差），还原为字面 *
	u.RawQuery = strings.ReplaceAll(q.Encode(), "%2A", "*")
	return u.String()
}

// EncodeBody body 落库编码：有效 UTF-8 且不含 NUL 字节 → 原文 plain；
// 否则（multipart/图片/音频等二进制）→ base64（PostgreSQL TEXT 列不接受 NUL 与无效 UTF-8）
func EncodeBody(b []byte) (string, string) {
	if len(b) == 0 {
		return "", "plain"
	}
	if utf8.Valid(b) && !bytes.ContainsRune(b, 0) {
		return string(b), "plain"
	}
	return base64.StdEncoding.EncodeToString(b), "base64"
}
