package common

import (
	"net"
	"testing"
)

func mustParseIP(t *testing.T, s string) net.IP {
	t.Helper()
	ip := net.ParseIP(s)
	if ip == nil {
		t.Fatalf("测试用例的 IP 非法: %s", s)
	}
	return ip
}

func TestParseIpBlacklistEntries(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantRaw []string // 期望解析出的条目顺序
	}{
		{"空串", "", nil},
		{"JSON 空数组", "[]", nil},
		{"JSON 数组含精确 IP 与 CIDR", `["1.2.3.4", "10.0.0.0/8", "2001:db8::/32"]`,
			[]string{"1.2.3.4", "10.0.0.0/8", "2001:db8::/32"}},
		{"JSON 数组去重去空", `["1.2.3.4", "1.2.3.4", "", " 5.6.7.8 "]`, []string{"1.2.3.4", "5.6.7.8"}},
		{"裸文本逗号分隔（手工改库容错）", "1.2.3.4,10.0.0.0/8", []string{"1.2.3.4", "10.0.0.0/8"}},
		{"非法条目静默跳过", `["1.2.3.4", "not-an-ip", "300.1.2.3"]`, []string{"1.2.3.4"}},
		{"JSON 语法错误返回空", `["1.2.3.4"`, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := ParseIpBlacklistEntries(tt.raw)
			if len(entries) != len(tt.wantRaw) {
				t.Fatalf("条目数不符: got %d (%v), want %d", len(entries), entries, len(tt.wantRaw))
			}
			for i, e := range entries {
				if e.Raw != tt.wantRaw[i] {
					t.Errorf("条目[%d] = %q, want %q", i, e.Raw, tt.wantRaw[i])
				}
			}
		})
	}
}

func TestIpBlacklistEntryContains(t *testing.T) {
	entries := ParseIpBlacklistEntries(`["1.2.3.4", "10.0.0.0/8", "2001:db8::/32"]`)
	if len(entries) != 3 {
		t.Fatalf("前置解析失败: %v", entries)
	}

	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"精确 IPv4 命中", "1.2.3.4", true},
		{"精确 IPv4 相邻不命中", "1.2.3.5", false},
		{"CIDR 内命中", "10.1.2.3", true},
		{"CIDR 外不命中", "11.0.0.1", false},
		{"IPv6 CIDR 命中", "2001:db8::1234", true},
		{"IPv6 CIDR 外不命中", "2001:db9::1", false},
		{"IPv4-mapped IPv6 命中 v4 CIDR", "::ffff:10.2.3.4", true},
		{"空 IP 不命中", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 空串对应 net.ParseIP 返回 nil 的场景（客户端 IP 解析失败），验证 Contains(nil) 安全返回 false
			var ip net.IP
			if tt.ip != "" {
				ip = mustParseIP(t, tt.ip)
			}
			got := false
			for i := range entries {
				if entries[i].Contains(ip) {
					got = true
					break
				}
			}
			if got != tt.want {
				t.Errorf("Contains(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestValidateIpBlacklistRaw(t *testing.T) {
	valid := []string{
		"",
		"[]",
		`["1.2.3.4"]`,
		`["1.2.3.4", "10.0.0.0/8"]`,
		`["2001:db8::/32", "::1"]`,
	}
	for _, raw := range valid {
		if err := ValidateIpBlacklistRaw(raw); err != nil {
			t.Errorf("ValidateIpBlacklistRaw(%q) = %v, want nil", raw, err)
		}
	}

	invalid := []string{
		`["1.2.3.4", "not-an-ip"]`,
		`["300.1.2.3"]`,
		`["10.0.0.0/33"]`,
		`["1.2.3.4"`,
	}
	for _, raw := range invalid {
		if err := ValidateIpBlacklistRaw(raw); err == nil {
			t.Errorf("ValidateIpBlacklistRaw(%q) = nil, want error", raw)
		}
	}
}
