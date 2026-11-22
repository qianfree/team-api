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

// TestTruncateStringInvalidInput 输入本身含非法 UTF-8（上游 GBK/二进制体）时，
// 归一化为替换字符后再截断——输出必须恒为合法 UTF-8，否则 PG（UTF8 库）拒收
// 整条 sys_error_logs 记录。
//
// 回归背景：旧实现的回退循环无上限，非法输入会逐字节退到空串，
// 截断结果只剩 "..."，日志内容全丢。
func TestTruncateStringInvalidInput(t *testing.T) {
	// 非法输入且不超长：归一化后原样语义返回（连续非法段 → 单个 U+FFFD）
	gbk := "\xb4\xed\xce\xf3" + " upstream error 500" // GBK「错误」+ ASCII
	want := "\uFFFD" + " upstream error 500"
	if got := truncateString(gbk, 2000); got != want {
		t.Fatalf("非法输入不超长应归一化返回: got %q want %q", got, want)
	}
	// 非法输入且超长：归一化后按边界截断，ASCII 排障线索保留
	//（\xb4\xed 为一段连续非法字节 → 1 个 U+FFFD = 3 字节 + 1997 个 x = 2000）
	long := "\xb4\xed" + strings.Repeat("x", 3000)
	got := truncateString(long, 2000)
	if !utf8.ValidString(got) {
		t.Fatalf("非法输入截断结果含非法序列: %q", got)
	}
	wantPrefix := "\uFFFD" + strings.Repeat("x", 1997)
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("非法输入截断应保留 ASCII 内容: got %q", got[:40])
	}
	// 归一化后的 U+FFFD（3 字节）后跟中文，截断点切在中文中间：
	// 回退 1 字节到「中文」边界前，输出 U+FFFD + "..."
	mixed := "\xb4" + "中文" + strings.Repeat("y", 3000)
	got = truncateString(mixed, 5) // U+FFFD(3) + 中(3)：第 5 字节切在「中」中间
	if got != "\uFFFD"+"..." {
		t.Fatalf("混合输入应回退到合法边界: got %q", got)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("混合输入截断结果含非法序列: %q", got)
	}
	// maxLen 小于首字符宽度：返回空串+"..."，不输出半个字符
	if got := truncateString("中文abc", 2); got != "..." {
		t.Fatalf("maxLen 小于首字符宽度应返回空串: got %q", got)
	}
}
