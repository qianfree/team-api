package minimax

import (
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

// H3 v2 succeeded：官方 usage + 任务级 resolution 全量归一进用量摘要
func TestParseV2TaskResult_UsageSummary(t *testing.T) {
	body := `{"task": {
		"id": "t1", "model": "MiniMax-H3", "status": "succeeded", "resolution": "768P", "ratio": "16:9",
		"content": {"url": "https://example.com/v.mp4"},
		"usage": {"total_seconds": 9, "input_seconds": 3, "output_seconds": 6, "input_image_count": 2, "input_audio_seconds": 4, "total_tokens": 1200, "prompt_tokens": 200, "completion_tokens": 1000}
	}}`
	info, err := (&Adaptor{}).ParseTaskResult([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if info.Status != common.TaskStatusSuccess {
		t.Fatalf("expected SUCCESS, got %s", info.Status)
	}
	s := info.UsageSummary
	if s == nil {
		t.Fatal("expected usage summary")
	}
	if s.InputVideoSeconds == nil || *s.InputVideoSeconds != 3 ||
		s.OutputVideoSeconds == nil || *s.OutputVideoSeconds != 6 ||
		s.TotalSeconds == nil || *s.TotalSeconds != 9 {
		t.Fatalf("时长字段: %+v", s)
	}
	if s.InputImageCount == nil || *s.InputImageCount != 2 ||
		s.InputAudioSeconds == nil || *s.InputAudioSeconds != 4 {
		t.Fatalf("素材字段: %+v", s)
	}
	if s.Resolution != "768P" {
		t.Fatalf("resolution: %s", s.Resolution)
	}
	if s.PromptTokens == nil || *s.PromptTokens != 200 ||
		s.CompletionTokens == nil || *s.CompletionTokens != 1000 ||
		s.TotalTokens == nil || *s.TotalTokens != 1200 {
		t.Fatalf("token 字段: %+v", s)
	}
}

// 零值素材字段不出现（omitempty 语义：没有的数据不返回）
func TestParseV2TaskResult_UsageSummaryOmitsZero(t *testing.T) {
	body := `{"task": {"id": "t2", "model": "MiniMax-H3", "status": "succeeded", "resolution": "768P",
		"content": {"url": "https://example.com/v.mp4"},
		"usage": {"output_seconds": 6, "total_seconds": 6}}}`
	info, err := (&Adaptor{}).ParseTaskResult([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	s := info.UsageSummary
	if s == nil {
		t.Fatal("expected usage summary")
	}
	if s.InputVideoSeconds != nil || s.InputImageCount != nil || s.InputAudioSeconds != nil ||
		s.PromptTokens != nil || s.CompletionTokens != nil || s.TotalTokens != nil {
		t.Fatalf("零值字段应省略: %+v", s)
	}
	if s.OutputVideoSeconds == nil || *s.OutputVideoSeconds != 6 {
		t.Fatalf("output_video_seconds: %v", s.OutputVideoSeconds)
	}
}
