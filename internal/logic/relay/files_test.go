package relay

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
)

// rewriteBody 用固定映射驱动 walkImageFileURLs 的替换语义测试。
func rewriteBody(t *testing.T, body string, mapping map[string]string) (string, bool) {
	t.Helper()
	newBody, changed := walkImageFileURLs([]byte(body), func(url string) (string, bool) {
		if target, ok := mapping[url]; ok {
			return target, true
		}
		return url, false
	})
	return string(newBody), changed
}

func TestWalkImageFileURLsReplacesFileRefs(t *testing.T) {
	body := `{"model":"gpt-4o","stream":true,"messages":[` +
		`{"role":"user","content":[` +
		`{"type":"text","text":"what is this"},{"type":"image_url","image_url":{"url":"file-abc123def4567890","detail":"low"}},` +
		`{"type":"image_url","image_url":{"url":"https://example.com/img.png"}}]}` +
		`,{"role":"user","content":"plain string content"}]}`

	got, changed := rewriteBody(t, body, map[string]string{
		"file-abc123def4567890": "https://bucket.oss.example.com/signed?sig=1",
	})
	if !changed {
		t.Fatal("expected changed=true")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("rewritten body is not valid JSON: %v\n%s", err, got)
	}

	// 结构完整性：顶层字段与消息数量保留
	if parsed["model"] != "gpt-4o" || parsed["stream"] != true {
		t.Errorf("top-level fields lost: %v", parsed)
	}
	msgs := parsed["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages count = %d, want 2", len(msgs))
	}

	// 第一条消息的三个分段：text 原样、file 引用已替换、外部 URL 原样
	parts := msgs[0].(map[string]any)["content"].([]any)
	if parts[0].(map[string]any)["text"] != "what is this" {
		t.Errorf("text part modified: %v", parts[0])
	}
	img1 := parts[1].(map[string]any)["image_url"].(map[string]any)
	if img1["url"] != "https://bucket.oss.example.com/signed?sig=1" {
		t.Errorf("file ref not replaced: %v", img1["url"])
	}
	if img1["detail"] != "low" {
		t.Errorf("sibling field detail lost: %v", img1)
	}
	img2 := parts[2].(map[string]any)["image_url"].(map[string]any)
	if img2["url"] != "https://example.com/img.png" {
		t.Errorf("external URL should pass through untouched: %v", img2["url"])
	}

	// 第二条消息（字符串 content）不应被改动或丢弃
	if msgs[1].(map[string]any)["content"] != "plain string content" {
		t.Errorf("string content message modified: %v", msgs[1])
	}
}

func TestWalkImageFileURLsNoRefsUnchanged(t *testing.T) {
	cases := []string{
		`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`,                                                         // 无 image_url
		`{"model":"gpt-4o","messages":[{"role":"user","content":[{"type":"text","text":"file-mentioned-in-text but not a ref"}]}]}`, // 文本里提到 file- 不算引用
		`{"model":"text-embedding","input":"file-not-a-ref"}`,                                                                       // 无 messages
		`not a json body`, // 非 JSON
		`[1,2,3]`,         // JSON 数组顶层
	}
	for _, body := range cases {
		got, changed := rewriteBody(t, body, map[string]string{"file-abc123def4567890": "x"})
		if changed {
			t.Errorf("expected no change for %s", body)
		}
		if got != body {
			t.Errorf("body should be byte-identical when unchanged:\n got: %s\nwant: %s", got, body)
		}
	}
}

func TestWalkImageFileURLsSameRefMultipleTimes(t *testing.T) {
	body := `{"messages":[{"role":"u","content":[{"type":"image_url","image_url":{"url":"file-1111111122222222"}}]},` +
		`{"role":"u","content":[{"type":"image_url","image_url":{"url":"file-1111111122222222"}}]}]}`

	got, changed := rewriteBody(t, body, map[string]string{"file-1111111122222222": "https://signed"})
	if !changed {
		t.Fatal("expected changed=true")
	}
	if want := 2; countSubstr(got, "https://signed") != want {
		t.Errorf("replaced %d times, want %d", countSubstr(got, "https://signed"), want)
	}
	if countSubstr(got, "file-1111111122222222") != 0 {
		t.Errorf("file ref leaked after rewrite: %s", got)
	}
}

func TestWalkImageFileURLsFastPath(t *testing.T) {
	// 不含 "file- 字面量的 body 走快速通道：返回同一字节切片（透传模式零开销关键）
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://x/y.png"}}]}]}`)
	got, changed := walkImageFileURLs(body, func(string) (string, bool) {
		t.Fatal("transform must not be called on fast path")
		return "", false
	})
	if changed || &body[0] != &got[0] {
		t.Errorf("fast path should return original slice unchanged")
	}
}

func TestRelayFileRefPattern(t *testing.T) {
	match := []string{"file-abc123def4567890", "file-ABCDEFGH1234", "file-" + strings.Repeat("a", 64)}
	noMatch := []string{
		"https://example.com/a.png", "data:image/png;base64,xxx",
		"file-short", "file-特殊字符!", "file-", "file-abc_def",
	}
	for _, s := range match {
		if !relayFileRefPattern.MatchString(s) {
			t.Errorf("should match %q", s)
		}
	}
	for _, s := range noMatch {
		if relayFileRefPattern.MatchString(s) {
			t.Errorf("should not match %q", s)
		}
	}
}

func TestNewRelayFileExtID(t *testing.T) {
	format := regexp.MustCompile(`^file-[0-9a-f]{32}$`)
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		id, err := newRelayFileExtID()
		if err != nil {
			t.Fatalf("newRelayFileExtID: %v", err)
		}
		if !format.MatchString(id) {
			t.Fatalf("ext id %q does not match file-<32hex>", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ext id generated: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func TestAllowedRelayFilePurposes(t *testing.T) {
	for _, p := range []string{"assistants", "batch", "fine-tune", "fine-tuning", "vision", "user_data", "evals"} {
		if _, ok := allowedRelayFilePurposes[p]; !ok {
			t.Errorf("official purpose %q should be allowed", p)
		}
	}
	for _, p := range []string{"", "Vision", "chat", "embedding"} {
		if _, ok := allowedRelayFilePurposes[p]; ok {
			t.Errorf("purpose %q should be rejected", p)
		}
	}
}

func TestMatchRelayFileType(t *testing.T) {
	imgOnly := []string{"image/*"}
	imgPdfExt := []string{"image/*", "application/pdf", ".txt"}
	cases := []struct {
		name        string
		filename    string
		contentType string
		allowed     []string
		want        bool
	}{
		// MIME 前缀命中
		{"png by mime", "a.png", "image/png", imgOnly, true},
		{"jpeg by mime uppercase", "a.JPG", "IMAGE/JPEG", imgOnly, true},
		// 工具常用 octet-stream，prefix 条目按扩展名兜底表命中
		{"png by ext with octet-stream", "a.png", "application/octet-stream", imgOnly, true},
		// 已知限制：内容伪装（exe 改名 .png）按扩展名放行——扩展名/MIME 均客户端
		// 提供，内容级校验（magic bytes）为后续项
		{"renamed exe passes by ext (known limitation)", "photo.png", "application/x-msdownload", imgOnly, true},
		// 真实扩展名/MIME 不在白名单：拦截
		{"exe by ext", "evil.exe", "application/octet-stream", imgOnly, false},
		{"exe by mime", "anything", "application/x-msdownload", imgOnly, false},
		// 扩展名条目（带点/不带点等价）
		{"txt by dotted ext", "note.txt", "application/octet-stream", imgPdfExt, true},
		{"pdf by mime", "doc.pdf", "application/pdf", imgPdfExt, true},
		{"html rejected", "x.html", "text/html", imgPdfExt, false},
		// 无扩展名且 MIME 不命中
		{"no ext unknown mime", "README", "application/octet-stream", imgOnly, false},
		// 通配
		{"star allows all", "virus.exe", "application/x-msdownload", []string{"*"}, true},
		{"star-slash allows all", "a.bin", "application/octet-stream", []string{"*/*"}, true},
		// 留空 = 不限制
		{"empty list allows all", "a.exe", "application/x-msdownload", nil, true},
		// 声明 MIME 带 charset 参数
		{"mime with charset", "a.png", "image/png; charset=binary", imgOnly, true},
	}
	for _, c := range cases {
		if got := matchRelayFileType(c.filename, c.contentType, c.allowed); got != c.want {
			t.Errorf("%s: matchRelayFileType(%q, %q, %v) = %v, want %v",
				c.name, c.filename, c.contentType, c.allowed, got, c.want)
		}
	}
}

func TestCheckRelayFileSize(t *testing.T) {
	const mb = 1024 * 1024
	if err := checkRelayFileSize(0, 0); err == nil {
		t.Error("empty file should be rejected even without limit")
	}
	if err := checkRelayFileSize(-5, 0); err == nil {
		t.Error("negative size should be rejected")
	}
	if err := checkRelayFileSize(1024, 0); err != nil {
		t.Errorf("size under unlimited config should pass: %v", err)
	}
	if err := checkRelayFileSize(5*mb, 10*mb); err != nil {
		t.Errorf("size under limit should pass: %v", err)
	}
	if err := checkRelayFileSize(10*mb+1, 10*mb); err == nil {
		t.Error("size over limit should be rejected")
	} else if !errors.Is(err, ErrRelayFileSizeExceeded) {
		t.Errorf("should wrap ErrRelayFileSizeExceeded, got %v", err)
	}
}

func countSubstr(s, sub string) int {
	return strings.Count(s, sub)
}
