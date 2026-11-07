package artifact

import "testing"

// TestSameChannelHost channel-bearer 同源判定：host 一致 + 端口归一，
// 子域/不同 host/非法输入都判不同源。
func TestSameChannelHost(t *testing.T) {
	cases := []struct {
		name    string
		rawURL  string
		baseURL string
		want    bool
	}{
		{"同 host 同端口", "https://api.vendor.com/v1/file", "https://api.vendor.com", true},
		{"显式默认端口归一", "https://api.vendor.com:443/v1/file", "https://api.vendor.com", true},
		{"http 缺省端口归一", "http://relay.internal:80/x", "http://relay.internal", true},
		{"自建内网渠道同源", "http://10.0.0.5:9000/artifacts/a", "http://10.0.0.5:9000", true},
		{"子域不同源", "https://cdn.vendor.com/file", "https://api.vendor.com", false},
		{"不同 host 不同源", "https://evil.example/file", "https://api.vendor.com", false},
		{"同 host 不同端口", "https://api.vendor.com:8443/file", "https://api.vendor.com", false},
		{"base 非法", "https://api.vendor.com/file", "://bad", false},
	}
	for _, c := range cases {
		if got := sameChannelHost(c.rawURL, c.baseURL); got != c.want {
			t.Errorf("%s: sameChannelHost(%s, %s) = %v, want %v", c.name, c.rawURL, c.baseURL, got, c.want)
		}
	}
}
