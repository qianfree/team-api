package common

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ===== 脱敏 =====

func TestMaskSecret(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"short", "****"},                           // < 12 全遮蔽
		{"12345678901", "****"},                     // 11 位仍全遮蔽
		{"sk-abcdef1234wxyz", "sk-abc****wxyz"},     // 前 6 后 4
		{"", "****"},                                // 空值
		{"Bearer sk-abcdefghijk", "Bearer****hijk"}, // 带前缀的完整头值
	}
	for _, c := range cases {
		if got := MaskSecret(c.in); got != c.want {
			t.Errorf("MaskSecret(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMaskSensitiveHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer sk-abcdefghijklmnop")
	h.Set("X-Api-Key", "short")
	h.Set("Content-Type", "application/json")
	h.Add("X-Multi", "a")
	h.Add("X-Multi", "b")

	out := MaskHeaders(h)
	if got := out["Authorization"]; got != "Bearer****mnop" {
		t.Errorf("Authorization 未按前6后4脱敏: %q", got)
	}
	if got := out["X-Api-Key"]; got != "****" {
		t.Errorf("短凭证值应全遮蔽: %q", got)
	}
	if got := out["Content-Type"]; got != "application/json" {
		t.Errorf("非敏感头不应改动: %q", got)
	}
	if got := out["X-Multi"]; got != "a, b" {
		t.Errorf("多值头应逗号连接: %q", got)
	}
	if MaskHeaders(nil) != nil {
		t.Error("nil 头应返回 nil")
	}
}

func TestMaskURLSecret(t *testing.T) {
	raw := "https://example.com/v1/models?key=abcdefghijk1234&x=1"
	got := MaskURLSecret(raw)
	if strings.Contains(got, "abcdefghijk1234") {
		t.Errorf("URL query 凭证未脱敏: %q", got)
	}
	if !strings.Contains(got, "abcdef****1234") {
		t.Errorf("脱敏格式不符合前6后4: %q", got)
	}
	if !strings.Contains(got, "x=1") {
		t.Errorf("非凭证参数不应改动: %q", got)
	}
	// 无凭证参数的 URL 原样返回
	plain := "https://example.com/v1/chat"
	if got := MaskURLSecret(plain); got != plain {
		t.Errorf("无凭证 URL 被改动: %q", got)
	}
}

// ===== body 编码 =====

func TestEncodeBody(t *testing.T) {
	// 有效 UTF-8 → plain
	data, enc := EncodeBody([]byte(`{"model":"gpt"}`))
	if enc != "plain" || data != `{"model":"gpt"}` {
		t.Errorf("UTF-8 body 应原样 plain: %q %q", data, enc)
	}
	// 含 NUL → base64（PG TEXT 不接受 NUL）
	nul := []byte{'a', 0x00, 'b'}
	data, enc = EncodeBody(nul)
	if enc != "base64" {
		t.Errorf("含 NUL body 应 base64: %q", enc)
	}
	if decoded, err := base64Decode(data); err != nil || !bytes.Equal(decoded, nul) {
		t.Errorf("base64 往返不一致: %v %v", decoded, err)
	}
	// 无效 UTF-8 → base64
	invalid := []byte{0xff, 0xfe, 0xfd}
	if _, enc = EncodeBody(invalid); enc != "base64" {
		t.Errorf("无效 UTF-8 body 应 base64: %q", enc)
	}
	// 空 body
	if data, enc = EncodeBody(nil); enc != "plain" || data != "" {
		t.Errorf("空 body 应 plain 空串: %q %q", data, enc)
	}
}

// ===== DebugClientWriter：不截断 =====

func TestDebugClientWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewDebugClientWriter(rec)

	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(200)

	// 写 2MB，验证完整累积不截断
	chunk := bytes.Repeat([]byte("x"), 64*1024)
	total := 0
	for i := 0; i < 32; i++ {
		n, err := w.Write(chunk)
		if err != nil || n != len(chunk) {
			t.Fatalf("write 失败: n=%d err=%v", n, err)
		}
		total += n
	}
	if got := len(w.Bytes()); got != total {
		t.Errorf("捕获 %d 字节，期望 %d（不允许截断）", got, total)
	}
	if w.StatusCode() != 200 {
		t.Errorf("状态码 = %d, want 200", w.StatusCode())
	}
	if ct := w.HeaderSnapshot()["Content-Type"]; ct != "text/event-stream" {
		t.Errorf("响应头快照错误: %q", ct)
	}
	// 未显式 WriteHeader 时默认 200
	if code := NewDebugClientWriter(httptest.NewRecorder()).StatusCode(); code != 200 {
		t.Errorf("默认状态码 = %d, want 200", code)
	}
}

// ===== DebugRoundTripper =====

func TestDebugRoundTripperPassthrough(t *testing.T) {
	// ctx 无捕获器：直接透传，行为与裸 transport 一致
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := &http.Client{Transport: WrapDebugTransport(http.DefaultTransport.(*http.Transport).Clone())}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("passthrough 请求失败: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("透传响应被篡改: %q", body)
	}
}

func TestDebugRoundTripperCapture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 分块写入模拟流式响应，验证 tee 完整镜像
		_, _ = w.Write([]byte(`{"chunk":1}`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		_, _ = w.Write([]byte(`{"chunk":2}`))
	}))
	defer srv.Close()

	a := &DebugAttemptCapture{}
	ctx := WithDebugAttempt(context.Background(), a)
	client := &http.Client{Transport: WrapDebugTransport(http.DefaultTransport.(*http.Transport).Clone())}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/chat?key=topsecret12345",
		strings.NewReader(`{"model":"gpt","messages":[]}`))
	if err != nil {
		t.Fatalf("构建请求失败: %v", err)
	}
	req.Header.Set("Authorization", "Bearer sk-abcdefghijklmnop")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	snap := a.snapshot()

	// 段2：上游请求捕获（headers 脱敏 + URL 凭证脱敏 + body 完整）
	if snap.Method != http.MethodPost {
		t.Errorf("method = %q", snap.Method)
	}
	if !strings.Contains(snap.URL, "topsec****2345") {
		t.Errorf("URL 凭证未脱敏: %q", snap.URL)
	}
	if got := snap.ReqHeaders["Authorization"]; got != "Bearer****mnop" {
		t.Errorf("Authorization 未脱敏: %q", got)
	}
	if string(snap.ReqBody) != `{"model":"gpt","messages":[]}` {
		t.Errorf("上游请求体捕获不完整: %q", snap.ReqBody)
	}

	// 段3：上游响应捕获（分块写入也应完整）
	if snap.Status != 200 {
		t.Errorf("status = %d", snap.Status)
	}
	if !strings.Contains(string(body), `{"chunk":1}`) {
		t.Errorf("客户端读取的响应不完整: %q", body)
	}
	if string(snap.RespBody) != `{"chunk":1}{"chunk":2}` {
		t.Errorf("上游响应体镜像不完整: %q", snap.RespBody)
	}
	if snap.LatencyMs < 0 {
		t.Errorf("latency = %d", snap.LatencyMs)
	}
}

func TestDebugRoundTripperError(t *testing.T) {
	// 连接失败：err 路径记录 upstreamError，不 panic
	a := &DebugAttemptCapture{}
	ctx := WithDebugAttempt(context.Background(), a)
	client := &http.Client{Transport: WrapDebugTransport(http.DefaultTransport.(*http.Transport).Clone())}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:1/nope", nil)
	_, err := client.Do(req)
	if err == nil {
		t.Fatal("期望连接失败")
	}
	snap := a.snapshot()
	if snap.Err == "" {
		t.Error("传输层错误未被记录")
	}
}

func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
