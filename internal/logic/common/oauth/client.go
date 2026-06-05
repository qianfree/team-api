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

// GetHTTPClient returns an HTTP client configured with proxy if channel_proxy_url is set in system settings.
// Rebuilds on each call so runtime config changes take effect.
func GetHTTPClient() *http.Client {
	proxyClientMu.RLock()
	client := proxyClientValue
	proxyClientMu.RUnlock()
	if client != nil {
		return client
	}

	proxyClientMu.Lock()
	defer proxyClientMu.Unlock()
	// Double-check after acquiring write lock
	if proxyClientValue != nil {
		return proxyClientValue
	}
	proxyClientValue = buildProxyClient()
	return proxyClientValue
}

// ResetHTTPClient forces rebuilding the HTTP client on next GetHTTPClient call.
func ResetHTTPClient() {
	proxyClientMu.Lock()
	defer proxyClientMu.Unlock()
	proxyClientValue = nil
}

func buildProxyClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	proxyURL := common.Config().GetString(context.Background(), "channel_proxy_url")
	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			g.Log().Warningf(context.Background(), "oauth: invalid proxy URL %q: %v", proxyURL, err)
		} else {
			transport.Proxy = http.ProxyURL(parsed)
			g.Log().Infof(context.Background(), "oauth: using proxy %s", proxyURL)
		}
	}

	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}
}
