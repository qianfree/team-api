package shared

import "testing"

// 内嵌错误探测是 chat 上游方向共用的防线：漏判会把 200+SSE 形态的上游错误
// 包装成客户端眼中的"正常结束"，误判则会把正常 chunk 当错误中断流。
func TestExtractStreamEmbeddedError(t *testing.T) {
	cases := []struct {
		name    string
		data    string
		wantErr bool
		wantSub string
	}{
		{"标准错误对象", `{"error":{"message":"quota exceeded","type":"insufficient_quota"}}`, true, "quota exceeded"},
		{"字符串形态错误", `{"error":"upstream timeout"}`, true, "upstream timeout"},
		{"error 为 null 的正常 chunk", `{"id":"c","choices":[{"delta":{"content":"hi"}}],"error":null}`, false, ""},
		{"无 error 键的正常 chunk", `{"id":"c","choices":[{"delta":{"content":"hi"}}]}`, false, ""},
		{"非 JSON", `not json`, false, ""},
		{"空对象", `{}`, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, ok := ExtractStreamEmbeddedError([]byte(tc.data))
			if ok != tc.wantErr {
				t.Fatalf("ok = %v, want %v (body=%s)", ok, tc.wantErr, body)
			}
			if tc.wantErr && !containsStr(string(body), tc.wantSub) {
				t.Errorf("body = %s, want contains %q", body, tc.wantSub)
			}
		})
	}
}

func containsStr(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && findSubstring(s, sub) >= 0)
}
