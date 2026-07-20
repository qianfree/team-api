package task

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/relay/dto"
)

func TestMemResponseWriter(t *testing.T) {
	w := newMemResponseWriter()
	if w.StatusCode() != http.StatusOK {
		t.Fatalf("default status = %d, want 200", w.StatusCode())
	}
	if _, err := w.Write([]byte("hello ")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := w.Write([]byte("world")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := string(w.Bytes()); got != "hello world" {
		t.Fatalf("body = %q, want %q", got, "hello world")
	}
	w.WriteHeader(http.StatusBadGateway)
	if w.StatusCode() != http.StatusBadGateway {
		t.Fatalf("status after WriteHeader = %d, want 502", w.StatusCode())
	}
	w.Header().Set("Content-Type", "application/json")
	if w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("header not stored")
	}
}

func TestTryOccupyChannel_Unlimited(t *testing.T) {
	const ch = int64(9000001)
	defer func() { channelInflight = map[int64]int{} }()

	// MaxConcurrency <= 0 视为不限：多次占用均成功。
	for i := 0; i < 5; i++ {
		if !tryOccupyChannel(ch, 0) {
			t.Fatalf("occupy #%d with maxConc=0 should succeed", i)
		}
	}
	if !tryOccupyChannel(ch, -1) {
		t.Fatalf("occupy with maxConc=-1 should succeed")
	}
}

func TestTryOccupyChannel_Saturation(t *testing.T) {
	const ch = int64(9000002)
	defer func() { channelInflight = map[int64]int{} }()

	if !tryOccupyChannel(ch, 2) {
		t.Fatal("occupy #1 should succeed")
	}
	if !tryOccupyChannel(ch, 2) {
		t.Fatal("occupy #2 should succeed")
	}
	if tryOccupyChannel(ch, 2) {
		t.Fatal("occupy #3 should fail (saturated)")
	}
	// 释放一个槽后应能再次占用
	decInflight(ch)
	if !tryOccupyChannel(ch, 2) {
		t.Fatal("occupy after release should succeed")
	}
}

func TestDecInflight_CleansUpAndFloorsAtZero(t *testing.T) {
	const ch = int64(9000003)
	defer func() { channelInflight = map[int64]int{} }()

	tryOccupyChannel(ch, 5)
	decInflight(ch)
	if _, ok := channelInflight[ch]; ok {
		t.Fatal("counter reaching 0 should be deleted from map")
	}
	// 多减不应变负 / panic
	decInflight(ch)
	if channelInflight[ch] != 0 {
		t.Fatalf("counter = %d, want 0", channelInflight[ch])
	}
}

func TestBuildImageResult_EmptyData(t *testing.T) {
	job := &SyncImageJob{PublicTaskID: "task_x"}
	body, _ := json.Marshal(map[string]any{"created": 1, "data": []any{}})
	if _, _, err := buildImageResult(context.Background(), job, body); err == nil {
		t.Fatal("empty data should error")
	}
}

// TestExtractImageUsage_GptImage gpt-image 系列返回 usage，应被解析出 token 用量。
func TestExtractImageUsage_GptImage(t *testing.T) {
	body := []byte(`{"created":1,"data":[{"b64_json":"x"}],"usage":{"total_tokens":4160,"input_tokens":160,"output_tokens":4000}}`)
	prompt, completion, total := extractImageUsage(body)
	if prompt != 160 || completion != 4000 || total != 4160 {
		t.Fatalf("got prompt=%d completion=%d total=%d, want 160/4000/4160", prompt, completion, total)
	}
}

// TestExtractImageUsage_TotalFallback total 缺失时用 input+output 兜底。
func TestExtractImageUsage_TotalFallback(t *testing.T) {
	body := []byte(`{"data":[{"b64_json":"x"}],"usage":{"input_tokens":100,"output_tokens":250}}`)
	_, _, total := extractImageUsage(body)
	if total != 350 {
		t.Fatalf("total = %d, want 350 (input+output fallback)", total)
	}
}

// TestExtractImageUsage_NoUsage DALL·E 等无 usage 字段，返回全 0（保持按次计费预扣）。
func TestExtractImageUsage_NoUsage(t *testing.T) {
	body := []byte(`{"created":1,"data":[{"url":"https://example.com/a.png"}]}`)
	prompt, completion, total := extractImageUsage(body)
	if prompt != 0 || completion != 0 || total != 0 {
		t.Fatalf("got prompt=%d completion=%d total=%d, want all 0", prompt, completion, total)
	}
}

// TestExtractImageUsage_Malformed 响应体非法 JSON 时安全返回全 0，不 panic。
func TestExtractImageUsage_Malformed(t *testing.T) {
	if p, c, tot := extractImageUsage([]byte("not json")); p != 0 || c != 0 || tot != 0 {
		t.Fatalf("malformed body should yield 0/0/0, got %d/%d/%d", p, c, tot)
	}
}

func TestBuildImageResult_B64WithoutStorageErrors(t *testing.T) {
	// 对象存储不可用时，b64_json 无法 re-host → 报错。覆盖 accessor 直接返回错误，
	// 避免在无数据库的单测环境触达 sys_options 查询。
	orig := acquireSyncImageFileSvc
	acquireSyncImageFileSvc = func(context.Context) (*lcommon.FileService, error) {
		return nil, lcommon.ErrStorageNotConfigured
	}
	defer func() { acquireSyncImageFileSvc = orig }()

	job := &SyncImageJob{PublicTaskID: "task_x", TenantID: 1, UserID: 2}
	raw, _ := json.Marshal(map[string]any{
		"created": 1,
		"data":    []any{map[string]any{"b64_json": base64.StdEncoding.EncodeToString([]byte("PNGDATA"))}},
	})
	if _, _, err := buildImageResult(context.Background(), job, raw); err == nil {
		t.Fatal("b64 without storage should error")
	}
}

// TestBuildImageResult_MultiURLPassthrough 多图 url 透传：全部图片进 normalized，首图作 ResultURL。
func TestBuildImageResult_MultiURLPassthrough(t *testing.T) {
	orig := rehostURLEnabled
	rehostURLEnabled = func(context.Context) bool { return false } // 透传，不下载 re-host
	defer func() { rehostURLEnabled = orig }()

	job := &SyncImageJob{PublicTaskID: "task_multi"}
	raw, _ := json.Marshal(map[string]any{
		"created": 123,
		"data": []any{
			map[string]any{"url": "https://cdn.example.com/a.png", "revised_prompt": "a"},
			map[string]any{"url": "https://cdn.example.com/b.png"},
			map[string]any{"url": "https://cdn.example.com/c.png"},
		},
	})
	resultURL, normalized, err := buildImageResult(context.Background(), job, raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resultURL != "https://cdn.example.com/a.png" {
		t.Fatalf("resultURL = %q, want first image url", resultURL)
	}
	var out dto.ImageResponse
	if e := json.Unmarshal(normalized, &out); e != nil {
		t.Fatalf("normalized not a valid ImageResponse: %v", e)
	}
	if len(out.Data) != 3 {
		t.Fatalf("normalized data len = %d, want 3", len(out.Data))
	}
	if out.Data[0].URL != "https://cdn.example.com/a.png" || out.Data[2].URL != "https://cdn.example.com/c.png" {
		t.Fatalf("normalized urls not preserved in order: %+v", out.Data)
	}
	if out.Data[0].RevisedPrompt != "a" {
		t.Fatalf("revised_prompt not preserved on first image")
	}
}

// TestBuildImageResult_AllOrNothingOnBadEntry 多图中任一条目无 url/b64 → 整体失败（all-or-nothing）。
func TestBuildImageResult_AllOrNothingOnBadEntry(t *testing.T) {
	orig := rehostURLEnabled
	rehostURLEnabled = func(context.Context) bool { return false }
	defer func() { rehostURLEnabled = orig }()

	job := &SyncImageJob{PublicTaskID: "task_bad"}
	raw, _ := json.Marshal(map[string]any{
		"created": 1,
		"data": []any{
			map[string]any{"url": "https://cdn.example.com/a.png"},
			map[string]any{}, // 无 url 无 b64 → 整单失败
		},
	})
	if _, _, err := buildImageResult(context.Background(), job, raw); err == nil {
		t.Fatal("entry without url/b64 should fail the whole task (all-or-nothing)")
	}
}

// TestBuildImageResult_MultiB64WithoutStorageErrors 多图 b64 无对象存储 → 整体失败。
func TestBuildImageResult_MultiB64WithoutStorageErrors(t *testing.T) {
	orig := acquireSyncImageFileSvc
	acquireSyncImageFileSvc = func(context.Context) (*lcommon.FileService, error) {
		return nil, lcommon.ErrStorageNotConfigured
	}
	defer func() { acquireSyncImageFileSvc = orig }()

	job := &SyncImageJob{PublicTaskID: "task_b64multi", TenantID: 1, UserID: 2}
	raw, _ := json.Marshal(map[string]any{
		"created": 1,
		"data": []any{
			map[string]any{"b64_json": base64.StdEncoding.EncodeToString([]byte("PNG1"))},
			map[string]any{"b64_json": base64.StdEncoding.EncodeToString([]byte("PNG2"))},
		},
	})
	if _, _, err := buildImageResult(context.Background(), job, raw); err == nil {
		t.Fatal("multi b64 without storage should error")
	}
}

// TestImageFailReason_StorageNotConfiguredFriendly 存储未配置的错误（穿过多层 %w 包裹）
// 应映射为中文友好提示；其它错误保留技术细节。
func TestImageFailReason_StorageNotConfiguredFriendly(t *testing.T) {
	// 模拟 buildImageResult → rehostImage 的多层包裹，验证 errors.Is 仍能识别哨兵。
	wrapped := fmt.Errorf("rehost b64_json[0]: %w",
		fmt.Errorf("object storage not configured, cannot re-host image: %w", lcommon.ErrStorageNotConfigured))
	msg := imageFailReason(wrapped)
	if !strings.Contains(msg, "对象存储") {
		t.Fatalf("storage-not-configured should map to friendly message, got %q", msg)
	}

	other := imageFailReason(errors.New("upstream returned garbage"))
	if strings.Contains(other, "对象存储") {
		t.Fatalf("non-storage error must not map to storage message, got %q", other)
	}
	if !strings.Contains(other, "upstream returned garbage") {
		t.Fatalf("non-storage error should retain technical detail, got %q", other)
	}
}

func TestExtFromContentType(t *testing.T) {
	cases := map[string]string{
		"image/png":  ".png",
		"image/jpeg": ".jpg",
		"image/webp": ".webp",
		"":           ".png",
		"text/plain": ".png",
	}
	for ct, want := range cases {
		if got := extFromContentType(ct); got != want {
			t.Errorf("extFromContentType(%q) = %q, want %q", ct, got, want)
		}
	}
}

func TestTruncateStr(t *testing.T) {
	if got := truncateStr("abcdef", 3); got != "abc" {
		t.Fatalf("truncate = %q, want abc", got)
	}
	if got := truncateStr("ab", 5); got != "ab" {
		t.Fatalf("truncate = %q, want ab", got)
	}

	// UTF-8 边界：不得把多字节字符（中文每字 3 字节）从中间切断。
	utf8Cases := []struct {
		s    string
		max  int
		want string
	}{
		{"中文", 3, "中"},  // 恰好一个完整字
		{"中文", 4, "中"},  // 第 4 字节落在「文」中间 → 回退到「中」
		{"中文", 5, "中"},  // 第 5 字节仍在「文」中间 → 回退到「中」
		{"中文", 6, "中文"}, // 恰好容纳两字
		{"中", 2, ""},    // 连一个字都放不下 → 空串（而非非法字节）
	}
	for _, c := range utf8Cases {
		got := truncateStr(c.s, c.max)
		if got != c.want {
			t.Fatalf("truncateStr(%q, %d) = %q, want %q", c.s, c.max, got, c.want)
		}
		if !utf8.ValidString(got) {
			t.Fatalf("truncateStr(%q, %d) = %q is not valid UTF-8", c.s, c.max, got)
		}
	}
}
