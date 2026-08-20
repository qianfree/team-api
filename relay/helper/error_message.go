package helper

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"regexp"
	"syscall"
)

// 用户可见错误消息中的敏感信息替换占位符。
const redactedPlaceholder = "[redacted]"

var (
	// redactURLPattern 匹配 http(s):// 开头的 URL（含 host、path、query）。
	redactURLPattern = regexp.MustCompile(`(?i)https?://[^\s"'<>]+`)
	// redactIPPattern 匹配 IPv4 地址。版本号偶有误伤，但在错误消息中无害。
	redactIPPattern = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
)

// SafeUpstreamErrorMessage 把可能包含上游渠道域名 / 内部传输细节的错误，
// 转换为对最终用户安全的消息文本。
//
// 设计原则：
//   - 传输层错误（client.Do 返回的 *url.Error）按类别归一化，保留
//     "超时 / 拒绝 / DNS / 重置" 等对用户有用的类别信息，但彻底抹掉
//     上游域名与 URL（这是本修复的核心目标）。
//   - 其余错误（业务错误、上游响应体解析错误等）保留原始消息，但做
//     URL / IP 抹除作为防御性兜底。
//
// 日志侧不受影响：调用方写日志时应使用原始 err（%v），保留完整诊断信息。
func SafeUpstreamErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	// 传输层错误：http.Client.Do 失败时返回 *url.Error，其 Error() 形如
	// `Post "https://api.openai.com/v1/chat/completions": dial tcp: ...`，
	// 完整暴露上游域名，必须归一化。
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return categorizeTransportError(urlErr)
	}

	// 非传输层错误：保留消息，但抹除其中可能残留的 URL / IP。
	return RedactMessage(err.Error())
}

// categorizeTransportError 按 *url.Error 内部成因归类为通用消息，不泄露域名。
func categorizeTransportError(urlErr *url.Error) string {
	inner := urlErr.Err

	// 客户端主动取消：不可重试，向用户说明即可
	if errors.Is(inner, context.Canceled) {
		return "请求已取消"
	}
	// 请求 / 响应超时（context 截止、i/o timeout、Client.Timeout 等）
	if errors.Is(inner, context.DeadlineExceeded) {
		return "请求超时，请重试"
	}
	var netErr net.Error
	if errors.As(inner, &netErr) && netErr.Timeout() {
		return "请求超时，请重试"
	}
	// 连接被上游拒绝（端口未开放 / 服务未启动）
	if errors.Is(inner, syscall.ECONNREFUSED) {
		return "服务暂时不可用，请稍后重试"
	}
	// DNS 解析失败：域名不存在或解析异常
	var dnsErr *net.DNSError
	if errors.As(inner, &dnsErr) {
		return "服务暂时不可用，请稍后重试"
	}
	// 连接被对端重置 / 提前关闭（GFW 干扰、上游瞬断的典型表现）
	if errors.Is(inner, syscall.ECONNRESET) ||
		errors.Is(inner, io.EOF) ||
		errors.Is(inner, io.ErrUnexpectedEOF) {
		return "连接中断，请重试"
	}
	// 其余无法归类的传输层错误
	return "服务暂时不可用，请稍后重试"
}

// RedactMessage 全量抹除字符串中的 URL / IP，替换为 [redacted]。
// 用作非传输层错误的防御性兜底（传输层域名已由 SafeUpstreamErrorMessage
// 的结构化分类兜住，此处只处理偶发含 URL/IP 的业务错误消息）。
func RedactMessage(msg string) string {
	if msg == "" {
		return msg
	}
	msg = redactURLPattern.ReplaceAllString(msg, redactedPlaceholder)
	msg = redactIPPattern.ReplaceAllString(msg, redactedPlaceholder)
	return msg
}
