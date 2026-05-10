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
	proxyClientOnce  sync.Once
	proxyClientValue *http.Client
)

// GetHTTPClient returns an HTTP client configured with proxy if channel_proxy_url is set in system settings.
func GetHTTPClient() *http.Client {
	proxyClientOnce.Do(func() {
		proxyClientValue = buildProxyClient()
	})
	return proxyClientValue
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
