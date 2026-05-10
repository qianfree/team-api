package common

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// sharedTransport 全局共享的 HTTP 传输层，所有渠道适配器共用一个连接池。
// http.Client 本身是轻量的，真正持有 TCP 连接池的是 Transport。
var sharedTransport = &http.Transport{
	MaxIdleConns:          500,
	MaxIdleConnsPerHost:   100,
	IdleConnTimeout:       90 * time.Second,
	DisableKeepAlives:     false,
	ForceAttemptHTTP2:     true,
	DisableCompression:    false,
	TLSHandshakeTimeout:   10 * time.Second,
	ResponseHeaderTimeout: 180 * time.Second,
}

// nonStreamClient / streamClient 预创建的单例 Client（无代理）。
var (
	nonStreamClient = &http.Client{
		Transport: sharedTransport,
		Timeout:   300 * time.Second,
	}
	streamClient = &http.Client{
		Transport: sharedTransport,
	}
)

// proxiedState 管理带代理的 HTTP 客户端，按需初始化并在代理 URL 变更时重建。
var proxiedState struct {
	mu             sync.RWMutex
	proxyURL       string
	transport      *http.Transport
	nonStream      *http.Client
	stream         *http.Client
	proxyURLCached atomic.Value // string
	cacheTime      atomic.Int64 // unix seconds of last config read
}

const proxyCacheTTL = 10 // seconds

// GetSystemProxyURL reads channel_proxy_url from system config with local cache.
// Exported for use by WebSocket dialers and other non-HTTP-client consumers.
func GetSystemProxyURL() string {
	now := time.Now().Unix()
	last := proxiedState.cacheTime.Load()
	if now-last < proxyCacheTTL {
		if v, ok := proxiedState.proxyURLCached.Load().(string); ok {
			return v
		}
	}

	proxyURL := g.Cfg().MustGet(context.Background(), "channel_proxy_url").String()
	proxiedState.proxyURLCached.Store(proxyURL)
	proxiedState.cacheTime.Store(now)
	return proxyURL
}

// getProxiedClients returns (nonStream, stream) clients configured with the current proxy.
// Rebuilds transport when proxy URL changes.
func getProxiedClients() (*http.Client, *http.Client) {
	proxyURL := GetSystemProxyURL()

	proxiedState.mu.RLock()
	if proxiedState.transport != nil && proxiedState.proxyURL == proxyURL {
		ns, s := proxiedState.nonStream, proxiedState.stream
		proxiedState.mu.RUnlock()
		return ns, s
	}
	proxiedState.mu.RUnlock()

	proxiedState.mu.Lock()
	defer proxiedState.mu.Unlock()

	// Double-check after acquiring write lock
	if proxiedState.transport != nil && proxiedState.proxyURL == proxyURL {
		return proxiedState.nonStream, proxiedState.stream
	}

	// Build new proxied transport
	transport := &http.Transport{
		MaxIdleConns:          500,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,
		ForceAttemptHTTP2:     true,
		DisableCompression:    false,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 180 * time.Second,
	}

	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			g.Log().Warningf(context.Background(), "proxy: invalid proxy URL %q: %v", proxyURL, err)
		} else {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}

	ns := &http.Client{
		Transport: transport,
		Timeout:   300 * time.Second,
	}
	s := &http.Client{
		Transport: transport,
	}

	proxiedState.proxyURL = proxyURL
	proxiedState.transport = transport
	proxiedState.nonStream = ns
	proxiedState.stream = s

	return ns, s
}

// NewPooledClient returns an http.Client with connection pooling.
// useProxy=true: uses the system proxy configured in channel_proxy_url.
// isStream=true: no Client.Timeout, managed by StreamScanner.
func NewPooledClient(timeoutSeconds int, useProxy bool, isStream ...bool) *http.Client {
	if useProxy {
		ns, s := getProxiedClients()
		if len(isStream) > 0 && isStream[0] {
			return s
		}
		if timeoutSeconds <= 0 {
			return ns
		}
		proxiedState.mu.RLock()
		transport := proxiedState.transport
		proxiedState.mu.RUnlock()
		return &http.Client{
			Transport: transport,
			Timeout:   time.Duration(timeoutSeconds) * time.Second,
		}
	}

	if len(isStream) > 0 && isStream[0] {
		return streamClient
	}
	if timeoutSeconds <= 0 {
		return nonStreamClient
	}
	return &http.Client{
		Transport: sharedTransport,
		Timeout:   time.Duration(timeoutSeconds) * time.Second,
	}
}
