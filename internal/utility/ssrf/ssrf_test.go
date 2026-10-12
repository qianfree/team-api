package ssrf

import (
	"net"
	"testing"
)

func TestIsForbiddenIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		// 禁止：云元数据 / 私网 / loopback / link-local / multicast / unspecified
		{"169.254.169.254", true}, // AWS/云元数据
		{"169.254.0.1", true},     // link-local
		{"10.0.0.1", true},        // 私网 10/8
		{"172.16.0.1", true},      // 私网 172.16/12
		{"172.31.255.255", true},  // 私网 172.16/12 上界
		{"172.32.0.1", false},     // 公网（紧邻私网上界）
		{"192.168.1.1", true},     // 私网 192.168/16
		{"127.0.0.1", true},       // loopback
		{"0.0.0.0", true},         // unspecified
		{"224.0.0.1", true},       // multicast
		{"::1", true},             // IPv6 loopback
		{"fe80::1", true},         // IPv6 link-local
		{"fc00::1", true},         // IPv6 ULA（私网）
		// 允许：公网
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"104.16.0.1", false},
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("ParseIP(%q) failed", c.ip)
		}
		if got := IsForbiddenIP(ip); got != c.want {
			t.Errorf("IsForbiddenIP(%q) = %v, want %v", c.ip, got, c.want)
		}
	}
}

func TestValidateURL(t *testing.T) {
	blocked := []string{
		// 字面量内网 IP（预检阶段直接拒绝）
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/x.png",
		"http://127.0.0.1/x.png",
		"http://192.168.1.1/x.png",
		"http://172.16.0.1/x.png",
		"https://[::1]/x.png",
		// 非 http(s) scheme
		"file:///etc/passwd",
		"gopher://127.0.0.1:6379/_FLUSHALL",
		"ftp://example.com/x.png",
		// 空 host
		"http:///x.png",
	}
	for _, u := range blocked {
		if err := ValidateURL(u); err == nil {
			t.Errorf("ValidateURL(%q) = nil, want error", u)
		}
	}

	allowed := []string{
		// 公网字面量 IP 与域名均应通过预检（域名的网段校验由 dial 时兜底）
		"https://8.8.8.8/x.png",
		"http://example.com/image.png",
		"https://cdn.example.com/a/b/c.jpeg",
	}
	for _, u := range allowed {
		if err := ValidateURL(u); err != nil {
			t.Errorf("ValidateURL(%q) = %v, want nil", u, err)
		}
	}
}
