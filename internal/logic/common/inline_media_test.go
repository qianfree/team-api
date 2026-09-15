package common

import (
	"strings"
	"testing"
)

// bigPayload 构造一段长度足够被认定为内联媒体的 base64 载荷。
func bigPayload(n int) string {
	return strings.Repeat("A", n)
}

func TestScanInlineMediaBytes(t *testing.T) {
	payload := bigPayload(400)
	cases := []struct {
		name      string
		body      string
		wantBytes int
		wantCount int
	}{
		{"纯文本", `{"model":"gpt-4o","messages":[{"role":"user","content":"你好"}]}`, 0, 0},
		{"空体", "", 0, 0},
		{
			"单图",
			`{"content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,` + payload + `"}}]}`,
			400, 1,
		},
		{
			"双图",
			`{"a":"data:image/png;base64,` + payload + `","b":"data:image/jpeg;base64,` + payload + `"}`,
			800, 2,
		},
		{
			"音频",
			`{"input_audio":{"data":"data:audio/mp3;base64,` + payload + `","format":"mp3"}}`,
			400, 1,
		},
		// 载荷过短：正文里恰好出现该标记，不应被当作媒体扣减
		{"短载荷不计入", `{"text":"写法是 ;base64,abc 这样"}`, 0, 0},
		// 无结束引号（截断的请求体）：载荷取到末尾
		{"未闭合", `{"url":"data:image/png;base64,` + payload, 400, 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotBytes, gotCount := ScanInlineMediaBytes([]byte(c.body))
			if gotBytes != c.wantBytes || gotCount != c.wantCount {
				t.Errorf("ScanInlineMediaBytes() = (%d, %d), want (%d, %d)",
					gotBytes, gotCount, c.wantBytes, c.wantCount)
			}
		})
	}
}

func TestStripInlineMedia(t *testing.T) {
	payload := bigPayload(400)

	t.Run("剥离载荷保留 mime 前缀", func(t *testing.T) {
		in := `{"url":"data:image/png;base64,` + payload + `","prompt":"描述这张图"}`
		got := StripInlineMedia(in)
		if strings.Contains(got, payload) {
			t.Error("base64 载荷未被剥离")
		}
		if !strings.Contains(got, "data:image/png;base64,<省略 400 字节>") {
			t.Errorf("占位文案不符: %s", got)
		}
		if !strings.Contains(got, `"prompt":"描述这张图"`) {
			t.Error("载荷之外的内容被破坏")
		}
	})

	t.Run("纯文本原样返回", func(t *testing.T) {
		in := `{"model":"gpt-4o","messages":[{"role":"user","content":"你好"}]}`
		if got := StripInlineMedia(in); got != in {
			t.Errorf("StripInlineMedia() = %q, want 原样返回", got)
		}
	})

	t.Run("多个载荷全部剥离", func(t *testing.T) {
		in := `{"a":"data:image/png;base64,` + payload + `","b":"data:image/jpeg;base64,` + payload + `"}`
		got := StripInlineMedia(in)
		if strings.Contains(got, payload) {
			t.Error("仍残留 base64 载荷")
		}
		if n := strings.Count(got, "<省略 400 字节>"); n != 2 {
			t.Errorf("占位数量 = %d, want 2", n)
		}
	})

	t.Run("短载荷保持原样", func(t *testing.T) {
		in := `{"text":"写法是 ;base64,abc 这样"}`
		if got := StripInlineMedia(in); got != in {
			t.Errorf("StripInlineMedia() = %q, want 原样返回", got)
		}
	})

	t.Run("剥离后可被字节版复用且无拷贝", func(t *testing.T) {
		plain := []byte(`{"content":"纯文本"}`)
		got := StripInlineMediaBytes(plain)
		if &got[0] != &plain[0] {
			t.Error("纯文本路径应原样返回入参切片，不做拷贝")
		}
	})
}

func TestApplyAuditLevelStripsInlineMedia(t *testing.T) {
	payload := bigPayload(400)
	body := `{"url":"data:image/png;base64,` + payload + `"}`

	req, _ := ApplyAuditLevel(AuditLevelFull, body, "", false, "/v1/chat/completions")
	if strings.Contains(req, payload) {
		t.Error("full 级别仍把 base64 载荷落库")
	}

	req, resp := ApplyAuditLevel(AuditLevelNone, body, body, false, "/v1/chat/completions")
	if req != "" || resp != "" {
		t.Error("none 级别应返回空")
	}
}
