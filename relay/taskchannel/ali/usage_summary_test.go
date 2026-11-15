package ali

import (
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

// wan3.0 SUCCEEDED 响应：官方 usage 全量归一进用量摘要，分辨率 SR → 档位词汇
func TestParseTaskResult_UsageSummary(t *testing.T) {
	body := `{
		"request_id": "req-1",
		"output": {"task_id": "t-1", "task_status": "SUCCEEDED", "video_url": "https://example.com/v.mp4"},
		"usage": {"video_count": 1, "duration": 8.0, "input_video_duration": 3.0, "output_video_duration": 5.0, "fps": 30, "SR": 720, "ratio": "16:9"}
	}`
	info, err := (&AliAdaptor{}).ParseTaskResult([]byte(body))
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
	if s.InputVideoSeconds == nil || *s.InputVideoSeconds != 3 {
		t.Fatalf("input_video_seconds: %v", s.InputVideoSeconds)
	}
	if s.OutputVideoSeconds == nil || *s.OutputVideoSeconds != 5 {
		t.Fatalf("output_video_seconds: %v", s.OutputVideoSeconds)
	}
	if s.TotalSeconds == nil || *s.TotalSeconds != 8 {
		t.Fatalf("total_seconds: %v", s.TotalSeconds)
	}
	if s.Resolution != "720P" {
		t.Fatalf("resolution: %s", s.Resolution)
	}
	// wan3.0 无 token 计量，token 字段不出现
	if s.PromptTokens != nil || s.CompletionTokens != nil || s.TotalTokens != nil {
		t.Fatalf("wan3.0 无 token 字段: %+v", s)
	}
}

// usage 全零（纯文生视频无输入素材且上游未回时长）不产出摘要
func TestParseTaskResult_UsageSummaryEmpty(t *testing.T) {
	body := `{"output": {"task_id": "t-2", "task_status": "SUCCEEDED", "video_url": "https://example.com/v.mp4"}}`
	info, err := (&AliAdaptor{}).ParseTaskResult([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if info.UsageSummary != nil {
		t.Fatalf("无 usage 不应有摘要: %+v", info.UsageSummary)
	}
}
