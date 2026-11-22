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

// TestTruncateUTF8SafeInvalidInput 输入本身含非法 UTF-8（上游 GBK/二进制错误页）时，
// 不得一路回退到空串把排障线索全丢掉：最多回退 utf8.UTFMax-1 字节仍找不到合法边界，
// 就退化为原始字节截断（乱码但可辨认，与旧的字节截断行为一致）。
//
// 回归背景：初版回退循环没有上限，GBK 输入会逐字节退到 0 字节——
// "GBK错误 + upstream error 500" 截断后日志里只剩 "...(truncated)"，内容全丢。
func TestTruncateUTF8SafeInvalidInput(t *testing.T) {
	// GBK「错误」（4 字节非法序列）+ ASCII，共 23 字节
	gbk := "\xb4\xed\xce\xf3" + " upstream error 500"
	if got := truncateUTF8Safe(gbk, 20); got != gbk[:20] {
		t.Fatalf("GBK 输入应退化为字节截断: got %q want %q", got, gbk[:20])
	}
	if got := truncateUTF8Safe(gbk, 2); got != gbk[:2] {
		t.Fatalf("GBK 短截断应退化为字节截断: got %q want %q", got, gbk[:2])
	}
	// 二进制前缀：回退满 3 字节仍非法，同样退化
	bin := "\x00\xff\xfe\x01abc"
	if got := truncateUTF8Safe(bin, 5); got != bin[:5] {
		t.Fatalf("二进制输入应退化为字节截断: got %q want %q", got, bin[:5])
	}
	// 非法字节在截断点之后：截断点本身合法，正常返回合法前缀
	tailInvalid := "abc\xb4\xed"
	if got := truncateUTF8Safe(tailInvalid, 3); got != "abc" {
		t.Fatalf("非法字节在截断点后应正常截断: got %q want %q", got, "abc")
	}
	// 混合：合法 UTF-8 前缀 + 非法字节恰在截断点前 1 字节——回退 1 字节即达
	// 合法边界「中文」，正常返回（不触发退化）
	mixed := "中文" + "\xff\xfe" + "tail"
	if got := truncateUTF8Safe(mixed, 7); got != "中文" {
		t.Fatalf("混合输入应回退到合法边界: got %q want %q", got, "中文")
	}
	// 混合：非法字节距截断点超过 utf8.UTFMax-1 字节，回退够不到边界——退化
	// 为原始字节截断，ASCII 部分仍完整保留
	farInvalid := "中" + "\xff" + "abcdefgh"
	if got := truncateUTF8Safe(farInvalid, 9); got != farInvalid[:9] {
		t.Fatalf("远距非法字节应退化为字节截断: got %q want %q", got, farInvalid[:9])
	}
	// 合法输入但 max 小于首字符宽度：返回空串，不输出半个字符
	tinyMax := "中文abc"
	if got := truncateUTF8Safe(tinyMax, 2); got != "" {
		t.Fatalf("max 小于首字符宽度应返回空串: got %q", got)
	}
}
