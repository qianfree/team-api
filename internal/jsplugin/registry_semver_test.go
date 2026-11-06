package jsplugin

import "testing"

// compareSemver 语义化版本比较：字典序会把 0.10.0 排在 0.9.0 之前、prerelease
// 排在正式版之前——fallbackActivateBuiltin 据此选「最新内置版本」，必须按语义序。
func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int // 1: a>b, -1: a<b, 0: 相等
	}{
		{"1.0.0", "1.0.0", 0},
		{"0.10.0", "0.9.0", 1},                // 数字段按值比（字典序会得出反向结论）
		{"1.10.0", "1.9.0", 1},                //
		{"2.0.0", "1.99.99", 1},               // 高位优先
		{"1.0.0", "1.0.0-beta", 1},            // 正式版高于 prerelease
		{"1.0.0-beta.2", "1.0.0-beta.10", -1}, // prerelease 段内字典序（覆盖 versionPattern 允许形态）
		{"1.0.0-beta", "1.0.0-alpha", 1},      //
		{"1.0.0", "1.0.1", -1},                //
		{"10.0.0", "9.0.0", 1},                // 双位数主版本
	}
	for _, c := range cases {
		if got := compareSemver(c.a, c.b); got != c.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
		if got := compareSemver(c.b, c.a); got != -c.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d（对称性）", c.b, c.a, got, -c.want)
		}
	}
}
