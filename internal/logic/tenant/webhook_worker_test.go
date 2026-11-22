package tenant

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestWebhookTruncateStringPGSafe webhook 投递日志的截断输出必须恒为合法 UTF-8：
// respBody 是接收方 HTTP 响应的原始字节（可能为 GBK/二进制错误页），非法序列会被
// PostgreSQL（UTF8 库）拒收，整条 opn_webhook_delivery_logs 记录丢失。
//
// 回归背景：旧实现是纯字节截断（s[:maxLen]），中文响应体切在多字节字符中间、
// GBK 响应体原样携带非法字节，均会触发 PG invalid byte sequence。
func TestWebhookTruncateStringPGSafe(t *testing.T) {
	// 合法 UTF-8：切在多字节字符中间时回退到字符边界
	zh := strings.Repeat("中文", 2000) // 每字 3 字节
	for _, maxLen := range []int{1, 2, 3, 4, 100, 1999, 2000, 2001} {
		got := truncateString(zh, maxLen)
		if !utf8.ValidString(got) {
			t.Fatalf("maxLen=%d: 截断结果含非法 UTF-8 序列", maxLen)
		}
		if len(got) > maxLen {
			t.Fatalf("maxLen=%d: 截断结果超长 len=%d", maxLen, len(got))
		}
	}
	// 不超长原样返回
	if got := truncateString("abc", 10); got != "abc" {
		t.Fatalf("短串应原样返回，got %q", got)
	}
	// 纯 ASCII 精确截断（无 "..." 后缀）
	if got := truncateString(strings.Repeat("a", 3000), 2000); got != strings.Repeat("a", 2000) {
		t.Fatalf("ASCII 截断结果异常")
	}
}

// TestWebhookTruncateStringInvalidInput 非 UTF-8 响应体归一化后截断，
// ASCII 排障线索保留。
func TestWebhookTruncateStringInvalidInput(t *testing.T) {
	// 不超长：连续非法段归一化为单个 U+FFFD
	gbk := "\xb4\xed\xce\xf3" + " upstream error 500"
	if got := truncateString(gbk, 2000); got != "\uFFFD upstream error 500" {
		t.Fatalf("非法输入不超长应归一化返回: got %q", got)
	}
	// 超长：归一化后按边界截断，ASCII 保留（1 个 U+FFFD = 3 字节 + 1997 x = 2000）
	long := "\xb4\xed" + strings.Repeat("x", 3000)
	got := truncateString(long, 2000)
	if !utf8.ValidString(got) {
		t.Fatalf("非法输入截断结果含非法序列: %q", got)
	}
	if want := "\uFFFD" + strings.Repeat("x", 1997); !strings.HasPrefix(got, want) {
		t.Fatalf("非法输入截断应保留 ASCII 内容: got %q", got[:40])
	}
	// maxLen 小于首字符宽度：返回空串，不输出半个字符
	if got := truncateString("中文abc", 2); got != "" {
		t.Fatalf("maxLen 小于首字符宽度应返回空串: got %q", got)
	}
}
