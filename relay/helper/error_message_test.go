package helper

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"strings"
	"syscall"
	"testing"

	"github.com/qianfree/team-api/relay/constant"
)

// fakeTimeoutError 是一个独立的 net.Error 实现，Timeout() 返回 true，
// 用于验证 categorizeTransportError 中 net.Error.Timeout() 分支（区别于 context.DeadlineExceeded）。
type fakeTimeoutError struct{}

func (fakeTimeoutError) Error() string   { return "i/o timeout" }
func (fakeTimeoutError) Timeout() bool   { return true }
func (fakeTimeoutError) Temporary() bool { return true }

// 上游域名泄露的典型形态：client.Do 失败返回的 *url.Error 含完整 URL。
func leakyURLError(inner error) *url.Error {
	return &url.Error{
		Op:  "Post",
		URL: "https://api.openai.com/v1/chat/completions",
		Err: inner,
	}
}

func TestSafeUpstreamErrorMessage_TransportCategories(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"超时-context截止", leakyURLError(context.DeadlineExceeded), "请求超时，请重试"},
		{"超时-i/o timeout", leakyURLError(fakeTimeoutError{}), "请求超时，请重试"},
		{"客户端取消", leakyURLError(context.Canceled), "请求已取消"},
		{"连接拒绝", leakyURLError(syscall.ECONNREFUSED), "服务暂时不可用，请稍后重试"},
		{"DNS失败", leakyURLError(&net.DNSError{Err: "no such host", Name: "api.openai.com"}), "服务暂时不可用，请稍后重试"},
		{"连接重置", leakyURLError(syscall.ECONNRESET), "连接中断，请重试"},
		{"EOF", leakyURLError(io.EOF), "连接中断，请重试"},
		{"意外EOF", leakyURLError(io.ErrUnexpectedEOF), "连接中断，请重试"},
		{"其余传输层错误", leakyURLError(errors.New("tls: handshake failure")), "服务暂时不可用，请稍后重试"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SafeUpstreamErrorMessage(c.err)
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
			// 核心安全断言：任何传输层错误的归一化结果都不得包含上游域名或协议
			if strings.Contains(got, "openai.com") {
				t.Errorf("结果泄露了上游域名: %q", got)
			}
			if strings.Contains(got, "http") {
				t.Errorf("结果泄露了协议/URL: %q", got)
			}
		})
	}
}

func TestSafeUpstreamErrorMessage_Nil(t *testing.T) {
	if got := SafeUpstreamErrorMessage(nil); got != "" {
		t.Fatalf("nil 应返回空串，got %q", got)
	}
}

func TestSafeUpstreamErrorMessage_BusinessErrorPreservedButRedacted(t *testing.T) {
	// 非传输层错误：消息保留，但其中的 URL/IP 必须被抹除
	err := errors.New(`dial upstream websocket failed: Get "https://api.x.com/v1": connection refused at 10.0.0.5`)
	got := SafeUpstreamErrorMessage(err)

	if strings.Contains(got, "https://api.x.com") {
		t.Errorf("URL 未被抹除: %q", got)
	}
	if strings.Contains(got, "10.0.0.5") {
		t.Errorf("IP 未被抹除: %q", got)
	}
	if !strings.Contains(got, "dial upstream websocket failed") {
		t.Errorf("业务消息正文不应被破坏: %q", got)
	}
	if !strings.Contains(got, "[redacted]") {
		t.Errorf("应包含 [redacted] 占位: %q", got)
	}
}

func TestSafeUpstreamErrorMessage_LegitimateBusinessMessageUntouched(t *testing.T) {
	// 上游 provider 的合法业务错误（无 URL/IP）应原样保留
	cases := []string{
		"rate limit exceeded",
		"invalid model: gpt-5-mini",
		"This model's maximum context length is 8192 tokens.",
	}
	for _, msg := range cases {
		got := SafeUpstreamErrorMessage(errors.New(msg))
		if got != msg {
			t.Errorf("合法消息被改动: got %q, want %q", got, msg)
		}
	}
}

func TestSafeUpstreamErrorMessage_WrappedRelayError(t *testing.T) {
	// 端到端：编排层把 *url.Error 塞进 NewUpstreamError 的 Cause，
	// 传入 SafeUpstreamErrorMessage 应能经 errors.As 找到 *url.Error 并归一化。
	relayErr := constant.NewUpstreamError(502, "upstream request failed",
		leakyURLError(&net.DNSError{Err: "no such host", Name: "api.openai.com"}))

	got := SafeUpstreamErrorMessage(relayErr)
	if got != "服务暂时不可用，请稍后重试" {
		t.Fatalf("经 RelayError 包装后归一化失败，got %q", got)
	}
	if strings.Contains(got, "openai.com") || strings.Contains(got, "https") {
		t.Errorf("仍泄露上游信息: %q", got)
	}
}

func TestRedactMessage(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"no sensitive info here", "no sensitive info here"},
		{
			`Post "https://api.openai.com/v1/chat": dial tcp: i/o timeout`,
			`Post "[redacted]": dial tcp: i/o timeout`,
		},
		{
			`connect to http://10.0.0.5:8080 failed`,
			`connect to [redacted] failed`,
		},
		{
			"https://sub.example.co.uk/a/b?x=1 and 192.168.1.1 both redacted",
			"[redacted] and [redacted] both redacted",
		},
	}
	for _, c := range cases {
		got := RedactMessage(c.in)
		if got != c.want {
			t.Errorf("RedactMessage(%q)\n  got  %q\n  want %q", c.in, got, c.want)
		}
	}
}
