package response

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTruncateStringUTF8Boundary 截断点必须落在合法 UTF-8 边界：
// 中文请求体按字节截断会切在多字节字符中间，非法序列会被 PostgreSQL（UTF8 库）
// 拒绝，整条错误日志落库失败（sys_error_logs invalid byte sequence）。
func TestTruncateStringUTF8Boundary(t *testing.T) {
	// 全中文，任意字节截断点都大概率落在字符中间
	s := strings.Repeat("产物流水线转存对象存储", 500) // 每字 3 字节
	for _, maxLen := range []int{1, 2, 3, 4, 100, 1999, 2000, 2001} {
		got := truncateString(s, maxLen)
		if !utf8.ValidString(got) {
			t.Fatalf("maxLen=%d: 截断结果含非法 UTF-8 序列", maxLen)
		}
		if len(got) > maxLen+3 { // 允许 "..." 后缀
			t.Fatalf("maxLen=%d: 截断结果超长 len=%d", maxLen, len(got))
		}
	}

	// 不超长时原样返回
	short := "abc"
	if got := truncateString(short, 10); got != short {
		t.Fatalf("短串应原样返回，got %q", got)
	}

	// 纯 ASCII 不受影响
	ascii := strings.Repeat("a", 3000)
	got := truncateString(ascii, 2000)
	if got != ascii[:2000]+"..." {
		t.Fatalf("ASCII 截断结果异常")
	}
}
