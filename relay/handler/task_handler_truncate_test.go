package handler

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTruncateUTF8SafeBoundary 日志体截断必须落在合法 UTF-8 边界。
//
// 回归背景：初版实现用 !utf8.RuneStart(末字节) 判断"是否切断字符"，但完整多字节
// 字符的最后一个字节同样是 continuation byte，会被误判成"被切断"而继续回退，
// 于是「截断点恰好落在字符边界」这种本该原样保留的情况反而被删出非法序列
// （如 "AB€" max=5 被截成 "AB\xe2"）。正确判据是整个 cut 是否 utf8.ValidString。
func TestTruncateUTF8SafeBoundary(t *testing.T) {
	cases := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"不超长原样返回", "abc", 10, "abc"},
		{"长度恰好等于上限原样返回", "中文", 6, "中文"},
		{"纯 ASCII 精确截断", strings.Repeat("a", 3000), 2000, strings.Repeat("a", 2000)},
		{"3 字节字符恰好在边界", "中ABC", 3, "中"},
		{"3 字节欧元符恰好在边界", "AB€XYZ", 5, "AB€"},
		{"4 字节 emoji 恰好在边界", "A😀BC", 5, "A😀"},
		{"切在 3 字节字符中间需回退", "中文测试ABC", 4, "中"},
		{"切在 4 字节 emoji 中间需回退", "A😀BC", 3, "A"},
		{"全中文大串", strings.Repeat("中", 1000), 2048, strings.Repeat("中", 682)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truncateUTF8Safe(c.s, c.max)
			if got != c.want {
				t.Fatalf("truncateUTF8Safe(%q, %d) = %q, want %q", c.s, c.max, got, c.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("截断结果含非法 UTF-8 序列: %q", got)
			}
			if len(got) > c.max {
				t.Fatalf("截断结果超长 len=%d > max=%d", len(got), c.max)
			}
		})
	}
}

// TestTruncateUTF8SafeAllCutPoints 穷举混合串的每一个字节截断点：
//   - 输出必须是合法 UTF-8、是原串前缀、长度不超 max；
//   - 截断点本身合法时（s[:max] ValidString），必须原样返回，不得多删；
//   - 输出必须是最长合法前缀（再加一字节就会非法或超长）。
func TestTruncateUTF8SafeAllCutPoints(t *testing.T) {
	s := strings.Repeat("中文😀a", 100) // 3+3+4+1 字节混合
	for max := 1; max < len(s); max++ {
		got := truncateUTF8Safe(s, max)
		if !utf8.ValidString(got) {
			t.Fatalf("max=%d: 截断结果含非法 UTF-8 序列 %q", max, got)
		}
		if !strings.HasPrefix(s, got) {
			t.Fatalf("max=%d: 结果不是原串前缀 %q", max, got)
		}
		if len(got) > max {
			t.Fatalf("max=%d: 结果超长 len=%d", max, len(got))
		}
		if utf8.ValidString(s[:max]) && got != s[:max] {
			t.Fatalf("max=%d: 截断点本就合法却被改动: got %q want %q", max, got, s[:max])
		}
		if len(got) < max && utf8.ValidString(s[:len(got)+1]) {
			t.Fatalf("max=%d: 结果非最长合法前缀 len=%d", max, len(got))
		}
	}
}
