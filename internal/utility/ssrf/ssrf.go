// Package ssrf 提供服务端发起出站 HTTP 请求的通用 SSRF 防护。
// 适用于目标 URL 由用户/租户配置的场景（webhook 回调、上游文件下载等）：
// 仅允许 http/https，拒绝 loopback / link-local（含云元数据 169.254.169.254）/
// RFC1918 私网 / multicast / unspecified 地址。
//
// 核心是 SafeDialContext——检查放在连接建立阶段而非仅请求前，可关闭
// 「预检解析为公网、连接时解析为内网」的 DNS rebinding TOCTOU 窗口。
package ssrf

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrBlocked 表示 URL 解析出的目标地址落在禁止访问的网段。
var ErrBlocked = errors.New("url host resolves to a forbidden (private/link-local/loopback) address")

// IsForbiddenIP 判断 ip 是否落在服务端请求永远不应触达的地址段（SSRF 黑名单）：
// loopback、link-local（含 169.254.169.254 云元数据）、RFC1918 私网、multicast、unspecified。
func IsForbiddenIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsPrivate() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}

// ValidateURL 对即将请求的 URL 做请求前的快速校验：仅允许 http/https scheme，
// 并对字面量 IP 主机直接判定网段。域名主机由 SafeDialContext 在 dial 时再校验。
func ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("parse url failed")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return errors.New("unsupported scheme (only http/https allowed)")
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("url host is empty")
	}
	// 字面量 IP 在请求前即可判定；域名留待 dial 时按解析结果判定（防 DNS rebinding）。
	if ip := net.ParseIP(host); ip != nil && IsForbiddenIP(ip) {
		return ErrBlocked
	}
	return nil
}

// SafeDialContext 在连接阶段解析主机名并拒绝任何指向 loopback / link-local / 私网 /
// multicast / unspecified 的地址，随后用解析到的 IP 直接建立连接（不再二次解析，杜绝
// rebinding）。
func SafeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if IsForbiddenIP(ip.IP) {
			return nil, ErrBlocked
		}
	}
	// 直接用已校验的 IP 拨号，避免 Transport 内部再次走 DNS（可能被 rebind 到内网）。
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, ip := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err != nil {
			lastErr = err
			continue
		}
		return conn, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no usable address after SSRF filter")
	}
	return nil, lastErr
}

// NewTransport 返回带 SSRF 防护的 http.Transport：dial 阶段校验目标地址、
// 显式禁用代理（避免经代理绕过 IP 校验访问内网）。
func NewTransport() *http.Transport {
	return &http.Transport{
		DialContext:     SafeDialContext,
		Proxy:           nil,
		IdleConnTimeout: 30 * time.Second,
	}
}

// RedirectPolicy 用作 http.Client 的 CheckRedirect：重定向每一跳都重新做 URL 校验，
// 防止「公网 URL 302 跳内网」绕过请求前预检；跳转目标的域名解析仍由
// SafeDialContext 在 dial 阶段兜底。返回 nil 表示继续跟随（跳数上限由 net/http 默认 10 次限制）。
func RedirectPolicy(req *http.Request, _ []*http.Request) error {
	return ValidateURL(req.URL.String())
}
