package jsplugin

import (
	"strings"
	"testing"
)

// TestRedactSecrets 敏感值脱敏：JSON 值形态与 URL query 形态都必须抹值留键。
func TestRedactSecrets(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "JSON camelCase 键",
			in:   `{"body":{"a":1},"apiKey":"sk-live-123","model":"m"}`,
			want: `{"body":{"a":1},"apiKey":"***","model":"m"}`,
		},
		{
			name: "JSON 大小写不敏感",
			in:   `{"APIKEY":"sk-1","Authorization":"Bearer eyJ.abc"}`,
			want: `{"APIKEY":"***","Authorization":"***"}`,
		},
		{
			name: "JSON 蛇形变体",
			in:   `{"api_key":"sk-2","secret":"s"}`,
			want: `{"api_key":"***","secret":"***"}`,
		},
		{
			name: "query 形态（fetch 报错里的 rawURL）",
			in:   `请求失败：url 不合法：https://v.example.com/file?api_key=abc123&sig=xyz&t=1`,
			want: `请求失败：url 不合法：https://v.example.com/file?api_key=***&sig=***&t=1`,
		},
		{
			name: "无敏感键原样返回",
			in:   `{"model":"kling-v1","prompt":"a cat"}`,
			want: `{"model":"kling-v1","prompt":"a cat"}`,
		},
		{
			name: "普通 token 单词不误伤",
			in:   `max_tokens exceeded`,
			want: `max_tokens exceeded`,
		},
	}
	for _, c := range cases {
		if got := RedactSecrets(c.in); got != c.want {
			t.Errorf("%s:\n got=%s\nwant=%s", c.name, got, c.want)
		}
	}
}

// TestRedactSecretsLargeInput 1MiB 输入不 panic（args 透传容器上限）。
func TestRedactSecretsLargeInput(t *testing.T) {
	big := `{"apiKey":"` + strings.Repeat("x", 1<<20) + `"}`
	_ = RedactSecrets(big)
}

// TestSanitizeHookMessage 对外错误净化：脱敏 + 剥控制字符 + 超长截断。
func TestSanitizeHookMessage(t *testing.T) {
	if got := SanitizeHookMessage(`jsplugin k hook h: sign failed with apiKey "sk-9"`); strings.Contains(got, "sk-9") {
		t.Errorf("密钥未脱敏: %s", got)
	}
	if got := SanitizeHookMessage("line1\nline2\x00\x07bad"); strings.ContainsAny(got, "\n\x00\x07") {
		t.Errorf("控制字符未剥除: %q", got)
	}
	long := strings.Repeat("触", 600)
	if got := SanitizeHookMessage(long); len([]rune(got)) > summaryLimit+3 {
		t.Errorf("超长消息未截断: %d runes", len([]rune(got)))
	}
}
