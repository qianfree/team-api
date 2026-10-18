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

	req, _ := ApplyAuditLevel(AuditLevelFull, body, "", false, "/v1/chat/completions", 200)
	if strings.Contains(req, payload) {
		t.Error("full 级别仍把 base64 载荷落库")
	}

	req, resp := ApplyAuditLevel(AuditLevelNone, body, body, false, "/v1/chat/completions", 200)
	if req != "" || resp != "" {
		t.Error("none 级别应返回空")
	}
}

// meta_only 仅留痕：内容一律不记，仅错误响应（>=400）保留响应体作为错误原因。
func TestApplyAuditLevelMetaOnly(t *testing.T) {
	reqBody := `{"model":"gpt-4o","messages":[{"role":"user","content":"很长的提问内容"}]}`
	errBody := `{"error":{"message":"upstream timeout","type":"upstream_error"}}`

	req, resp := ApplyAuditLevel(AuditLevelMetaOnly, reqBody, `{"choices":[]}`, false, "/v1/chat/completions", 200)
	if req != "" || resp != "" {
		t.Errorf("成功请求应全空, got req=%q resp=%q", req, resp)
	}

	req, resp = ApplyAuditLevel(AuditLevelMetaOnly, reqBody, errBody, false, "/v1/chat/completions", 500)
	if req != "" {
		t.Error("meta_only 任何情况下都不应记录请求体")
	}
	if resp != errBody {
		t.Errorf("错误响应应保留原文, got %q", resp)
	}

	// 400 边界同样保留错误原因
	_, resp = ApplyAuditLevel(AuditLevelMetaOnly, reqBody, errBody, false, "/v1/chat/completions", 400)
	if resp != errBody {
		t.Errorf("400 响应应保留原文, got %q", resp)
	}

	// 错误响应里的内联媒体载荷仍需剥离
	payload := bigPayload(400)
	mediaErr := `{"error":{"data":"data:image/png;base64,` + payload + `"}}`
	_, resp = ApplyAuditLevel(AuditLevelMetaOnly, reqBody, mediaErr, false, "/v1/chat/completions", 502)
	if strings.Contains(resp, payload) {
		t.Error("meta_only 错误响应仍把 base64 载荷落库")
	}
}

// 裸 base64 载荷（input_audio.data 等无 data: 前缀的协议形态）的扫描与剥离。
// 修复前仅识别 ";base64," 标记，聊天页音频附件的裸 base64 会按文本折算 token
// （4MB 音频 ≈ 100 万 token 预扣估算）并原文灌入审计日志。
func TestScanInlineMediaBytes_BareBase64(t *testing.T) {
	audio := bigPayload(4096)
	cases := []struct {
		name      string
		body      string
		wantBytes int
		wantCount int
	}{
		{
			"input_audio 裸 base64",
			`{"messages":[{"content":[{"type":"input_audio","input_audio":{"data":"` + audio + `","format":"mp3"}}]}]}`,
			4096, 1,
		},
		{
			"多段音频分别计入",
			`{"a":{"data":"` + audio + `"},"b":{"data":"` + audio + `"}}`,
			8192, 2,
		},
		{
			"data URL 与裸 base64 混合",
			`{"img":"data:image/png;base64,` + bigPayload(400) + `","audio":{"data":"` + audio + `"}}`,
			400 + 4096, 2,
		},
		// hex 哈希是 base64 字母表子集，但长度远低于阈值，不误伤
		{"hex 哈希不命中", `{"id":"` + strings.Repeat("ab", 64) + `","name":"x"}`, 0, 0},
		{"长自然文本不命中", `{"text":"` + strings.Repeat("word ", 300) + `"}`, 0, 0},
		{"未达阈值不命中", `{"data":"` + bigPayload(1023) + `"}`, 0, 0},
		// 含转义字符的字符串不可能是纯 base64 载荷
		{"含转义不命中", `{"text":"` + bigPayload(1024) + `\"quoted\" tail"}`, 0, 0},
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

func TestStripInlineMedia_BareBase64(t *testing.T) {
	audio := bigPayload(4096)

	t.Run("input_audio 剥离且 JSON 结构保留", func(t *testing.T) {
		in := `{"input_audio":{"data":"` + audio + `","format":"mp3"}}`
		got := StripInlineMedia(in)
		if strings.Contains(got, audio) {
			t.Error("裸 base64 载荷未被剥离")
		}
		if !strings.Contains(got, `"input_audio":{"data":"<省略 4096 字节>","format":"mp3"}`) {
			t.Errorf("占位或字段结构不符: %s", got)
		}
	})

	t.Run("混合形态全部剥离", func(t *testing.T) {
		in := `{"img":"data:image/png;base64,` + bigPayload(400) + `","audio":{"data":"` + audio + `"}}`
		got := StripInlineMedia(in)
		if strings.Count(got, "省略") != 2 {
			t.Errorf("两种形态应各剥离一次: %s", got)
		}
	})

	t.Run("阈值以下原样返回", func(t *testing.T) {
		in := `{"data":"` + bigPayload(1023) + `"}`
		if got := StripInlineMedia(in); got != in {
			t.Errorf("StripInlineMedia() = %q, want 原样返回", got)
		}
	})

	t.Run("字节版纯文本零拷贝", func(t *testing.T) {
		plain := []byte(`{"content":"纯文本音频讨论"}`)
		got := StripInlineMediaBytes(plain)
		if &got[0] != &plain[0] {
			t.Error("纯文本路径应原样返回入参切片，不做拷贝")
		}
	})
}
