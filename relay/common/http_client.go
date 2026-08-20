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

// defaultResponseHeaderTimeout 是普通同步请求等待上游响应头的最长时间。
// 用于在上游「连接已建立却迟迟不回响应头」（假死）时尽早失败，保护短超时请求。
//
// 但对总超时本身就较长的请求（图片/音频生成等：上游需先完成生成才发送响应头），
// 这个头超时会反过来误杀正常的长耗时请求——因此 NewPooledClient 在
// timeoutSeconds > defaultResponseHeaderTimeout 时改用禁用头超时的 longRun 传输层，
// 完全交由 http.Client.Timeout 兜底（见 longRunSharedTransport）。
const defaultResponseHeaderTimeout = 180 * time.Second

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
	ResponseHeaderTimeout: defaultResponseHeaderTimeout,
}

// longRunSharedTransport 用于总超时 > defaultResponseHeaderTimeout 的长耗时同步请求
// （如图片/音频生成：上游在生成完成前不会发送响应头）。禁用 ResponseHeaderTimeout，
// 避免被 180s 头超时在中途误杀连接，改由 http.Client.Timeout 统一控制总时长。
var longRunSharedTransport = &http.Transport{
	MaxIdleConns:          500,
	MaxIdleConnsPerHost:   100,
	IdleConnTimeout:       90 * time.Second,
	DisableKeepAlives:     false,
	ForceAttemptHTTP2:     true,
	DisableCompression:    false,
	TLSHandshakeTimeout:   10 * time.Second,
	ResponseHeaderTimeout: 0,
}

// nonStreamClient / streamClient 预创建的单例 Client（无代理）。
// Transport 统一经 WrapDebugTransport 包装：渠道调试开关开启的请求经 ctx 捕获器
// 镜像上游报文，其余请求一次 ctx 查找后直接透传（连接池/H2 仍由底层 Transport 管理）。
var (
	nonStreamClient = &http.Client{
		Transport: WrapDebugTransport(sharedTransport),
		Timeout:   300 * time.Second,
	}
	streamClient = &http.Client{
		Transport: WrapDebugTransport(sharedTransport),
	}
)

// proxiedState 管理带代理的 HTTP 客户端，按需初始化并在代理 URL 变更时重建。
var proxiedState struct {
	mu             sync.RWMutex
	proxyURL       string
	transport      *http.Transport // ResponseHeaderTimeout=defaultResponseHeaderTimeout（普通）
	longRun        *http.Transport // ResponseHeaderTimeout=0（长耗时同步）
	nonStream      *http.Client
	stream         *http.Client
	proxyURLCached atomic.Value // string
	cacheTime      atomic.Int64 // 上次读取配置的 unix 时间戳（秒）
}

const proxyCacheTTL = 10 // seconds

// GetSystemProxyURL 从系统配置读取 channel_proxy_url（带本地缓存）。
// 导出供 WebSocket 拨号器等非 HTTP 客户端调用方使用。
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

// buildProxiedTransport 构建一个带（可选）代理的传输层，ResponseHeaderTimeout 由 rht 决定。
func buildProxiedTransport(proxyURL string, rht time.Duration) *http.Transport {
	transport := &http.Transport{
		MaxIdleConns:          500,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,
		ForceAttemptHTTP2:     true,
		DisableCompression:    false,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: rht,
	}

	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			g.Log().Warningf(context.Background(), "proxy: invalid proxy URL %q: %v", proxyURL, err)
		} else {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}
	return transport
}

// getProxiedClients 返回基于当前代理配置的 (nonStream, stream) 客户端。
// 代理 URL 变更时重建传输层。同时维护一个禁用响应头超时的 longRun 传输层，
// 供 NewPooledClient 在长超时请求时取用。
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

	// 取得写锁后二次检查
	if proxiedState.transport != nil && proxiedState.proxyURL == proxyURL {
		return proxiedState.nonStream, proxiedState.stream
	}

	// 构建新的代理传输层（普通 + 长耗时同步各一份）
	proxiedState.transport = buildProxiedTransport(proxyURL, defaultResponseHeaderTimeout)
	proxiedState.longRun = buildProxiedTransport(proxyURL, 0)

	proxiedState.nonStream = &http.Client{
		Transport: WrapDebugTransport(proxiedState.transport),
		Timeout:   300 * time.Second,
	}
	proxiedState.stream = &http.Client{
		Transport: WrapDebugTransport(proxiedState.transport),
	}

	proxiedState.proxyURL = proxyURL

	return proxiedState.nonStream, proxiedState.stream
}

// NewPooledClient 返回带连接池的 http.Client。
// useProxy=true：使用 channel_proxy_url 中配置的系统代理。
// isStream=true：不设置 Client.Timeout，由 StreamScanner 管理。
//
// 当 timeoutSeconds > defaultResponseHeaderTimeout（180s）时，使用禁用响应头超时的
// longRun 传输层——图片/音频等长耗时同步请求的上游在生成完成前不发响应头，
// 否则会被 180s 头超时误杀。该阈值由 GetTimeoutSeconds 对图片模式强制下限到 600s 自然满足，
// 因此各适配器无需改动即可受益。
func NewPooledClient(timeoutSeconds int, useProxy bool, isStream ...bool) *http.Client {
	isStreamReq := len(isStream) > 0 && isStream[0]
	longRun := timeoutSeconds > int(defaultResponseHeaderTimeout/time.Second)

	if useProxy {
		ns, s := getProxiedClients()
		if isStreamReq {
			return s
		}
		if timeoutSeconds <= 0 {
			return ns
		}
		proxiedState.mu.RLock()
		transport := proxiedState.transport
		if longRun {
			transport = proxiedState.longRun
		}
		proxiedState.mu.RUnlock()
		return &http.Client{
			Transport: WrapDebugTransport(transport),
			Timeout:   time.Duration(timeoutSeconds) * time.Second,
		}
	}

	if isStreamReq {
		return streamClient
	}
	if timeoutSeconds <= 0 {
		return nonStreamClient
	}
	transport := sharedTransport
	if longRun {
		transport = longRunSharedTransport
	}
	return &http.Client{
		Transport: WrapDebugTransport(transport),
		Timeout:   time.Duration(timeoutSeconds) * time.Second,
	}
}
