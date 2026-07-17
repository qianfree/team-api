package task

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
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

func TestBuildImageResult_B64WithoutStorageErrors(t *testing.T) {
	// syncImageFileSvc 默认 nil（未启动 worker），b64_json 无法 re-host → 报错。
	syncImageFileSvc = nil
	job := &SyncImageJob{PublicTaskID: "task_x", TenantID: 1, UserID: 2}
	raw, _ := json.Marshal(map[string]any{
		"created": 1,
		"data":    []any{map[string]any{"b64_json": base64.StdEncoding.EncodeToString([]byte("PNGDATA"))}},
	})
	if _, _, err := buildImageResult(context.Background(), job, raw); err == nil {
		t.Fatal("b64 without storage should error")
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
}
