package ali

import "testing"

// TestNormalizeBaseURL 验证三种常见 base URL 写法都归一化为同一个裸域名，
// 防止再次出现 /compatible-mode/compatible-mode/v1/... 重复路径导致上游 404。
func TestNormalizeBaseURL(t *testing.T) {
	const want = "https://dashscope.aliyuncs.com"

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"裸域名", "https://dashscope.aliyuncs.com", want},
		{"裸域名带尾斜杠", "https://dashscope.aliyuncs.com/", want},
		{"带 compatible-mode", "https://dashscope.aliyuncs.com/compatible-mode", want},
		{"带 compatible-mode 尾斜杠", "https://dashscope.aliyuncs.com/compatible-mode/", want},
		{"带 compatible-mode/v1（阿里 SDK 文档写法）", "https://dashscope.aliyuncs.com/compatible-mode/v1", want},
		{"带 compatible-mode/v1 尾斜杠", "https://dashscope.aliyuncs.com/compatible-mode/v1/", want},
		{"首尾空格", "  https://dashscope.aliyuncs.com/compatible-mode/v1  ", want},
		{"自定义代理域名保持不变", "https://proxy.example.com/dashscope", "https://proxy.example.com/dashscope"},
		{"自定义代理域名剥离 compatible-mode", "https://proxy.example.com/compatible-mode/v1", "https://proxy.example.com"},
		{"空串", "", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normalizeBaseURL(c.in); got != c.want {
				t.Errorf("normalizeBaseURL(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestNormalizeBaseURLIdempotent 归一化必须幂等，重复调用不改变结果。
func TestNormalizeBaseURLIdempotent(t *testing.T) {
	inputs := []string{
		"https://dashscope.aliyuncs.com/compatible-mode/v1",
		"https://dashscope.aliyuncs.com/compatible-mode",
		"https://dashscope.aliyuncs.com",
	}
	for _, in := range inputs {
		once := normalizeBaseURL(in)
		if twice := normalizeBaseURL(once); twice != once {
			t.Errorf("非幂等: %q -> %q -> %q", in, once, twice)
		}
	}
}
