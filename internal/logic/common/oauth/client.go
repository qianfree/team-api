package oauth

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/logic/common"
)

var (
	proxyClientMu    sync.RWMutex
	proxyClientValue *http.Client
)

// GetHTTPClient 返回按系统设置 channel_proxy_url 配置了代理的 HTTP 客户端。
// 每次调用都会复用已构建实例，运行时配置变更通过 ResetHTTPClient 生效。
func GetHTTPClient(ctx context.Context) *http.Client {
	proxyClientMu.RLock()
	client := proxyClientValue
	proxyClientMu.RUnlock()
	if client != nil {
		return client
	}

	proxyClientMu.Lock()
	defer proxyClientMu.Unlock()
	// 拿到写锁后再做双重检查
	if proxyClientValue != nil {
		return proxyClientValue
	}
	proxyClientValue = buildProxyClient(ctx)
	return proxyClientValue
}

// ResetHTTPClient forces rebuilding the HTTP client on next GetHTTPClient call.
func ResetHTTPClient() {
	proxyClientMu.Lock()
	defer proxyClientMu.Unlock()
	proxyClientValue = nil
}

// buildProxyClient 构建带代理配置的 HTTP 客户端，配置读取与日志均透传调用方 ctx
func buildProxyClient(ctx context.Context) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	proxyURL := common.Config().GetString(ctx, "channel_proxy_url")
	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			g.Log().Warningf(ctx, "oauth: invalid proxy URL %q: %v", proxyURL, err)
		} else {
			transport.Proxy = http.ProxyURL(parsed)
			g.Log().Infof(ctx, "oauth: using proxy %s", proxyURL)
		}
	}

	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}
}
