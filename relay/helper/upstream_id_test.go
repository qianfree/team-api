package helper

import (
	"net/http"
	"testing"
)

func TestExtractUpstreamRequestID(t *testing.T) {
	cases := []struct {
		name   string
		header http.Header
		want   string
	}{
		{"new-api 头优先", http.Header{"X-Oneapi-Request-Id": {"oneapi-123"}}, "oneapi-123"},
		{"OpenAI 头", http.Header{"X-Request-Id": {"req_openai"}}, "req_openai"},
		{"小写形式同样命中", func() http.Header {
			// 真实 HTTP 响应头由 net/http 规范化存储，用 Set 模拟（map 字面量
			// 直接塞小写键绕过规范化，不代表实际行为）
			h := http.Header{}
			h.Set("x-request-id", "req_lower")
			return h
		}(), "req_lower"},
		{"Anthropic 头", http.Header{"Request-Id": {"req_anthropic"}}, "req_anthropic"},
		{"AWS 头", http.Header{"X-Amzn-Requestid": {"amzn-1"}}, "amzn-1"},
		{"多候选时按序取第一个", http.Header{
			"X-Oneapi-Request-Id": {"oneapi-1"},
			"X-Request-Id":        {"openai-1"},
		}, "oneapi-1"},
		{"仅第二个候选存在", http.Header{
			"X-Request-Id": {"openai-2"},
			"Request-Id":   {"anthropic-2"},
		}, "openai-2"},
		{"无候选头返回空", http.Header{"Content-Type": {"application/json"}}, ""},
		{"空值候选继续向后找", http.Header{
			"X-Oneapi-Request-Id": {""},
			"X-Request-Id":        {"fallback"},
		}, "fallback"},
		{"nil Header 返回空", nil, ""},
		{"超长截断到 128 字节", http.Header{"X-Request-Id": {string(make([]byte, 200))}}, string(make([]byte, 128))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ExtractUpstreamRequestID(c.header); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}
