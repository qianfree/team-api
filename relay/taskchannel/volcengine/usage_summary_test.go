package volcengine

import (
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

// ParseTaskResult 成功终态应填充用量摘要：任务级 duration/resolution + usage tokens
// （官方口径输入 token 恒 0，照常透出）
func TestParseTaskResult_UsageSummary(t *testing.T) {
	body := `{
		"id": "cgt-1", "model": "doubao-seedance", "status": "succeeded",
		"content": {"video_url": "https://example.com/v.mp4"},
		"seed": 42, "resolution": "720p", "ratio": "16:9", "duration": 5, "framespersecond": 24,
		"usage": {"completion_tokens": 38000, "total_tokens": 38000}
	}`
	info, err := (&VolcengineVideoAdaptor{}).ParseTaskResult([]byte(body))
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
	if s.OutputVideoSeconds == nil || *s.OutputVideoSeconds != 5 {
		t.Fatalf("output_video_seconds: %v", s.OutputVideoSeconds)
	}
	if s.Resolution != "720p" {
		t.Fatalf("resolution: %s", s.Resolution)
	}
	if s.CompletionTokens == nil || *s.CompletionTokens != 38000 || s.TotalTokens == nil || *s.TotalTokens != 38000 {
		t.Fatalf("tokens: %v", s)
	}
	// 火山官方：输入 token 恒 0（total = completion），0 亦有对账意义
	if s.PromptTokens == nil || *s.PromptTokens != 0 {
		t.Fatalf("prompt_tokens 应透出 0: %v", s.PromptTokens)
	}
	// 火山无输入素材计量，相关字段不出现
	if s.InputVideoSeconds != nil || s.InputImageCount != nil || s.InputAudioSeconds != nil || s.TotalSeconds != nil {
		t.Fatalf("火山无素材计量字段: %+v", s)
	}
}

// 非成功终态不产出用量摘要（duration/resolution 仅成功时有意义）
func TestParseTaskResult_UsageSummaryFailed(t *testing.T) {
	body := `{"id": "cgt-2", "status": "failed", "error": {"code": "E", "message": "boom"}, "usage": {"completion_tokens": 100, "total_tokens": 100}}`
	info, err := (&VolcengineVideoAdaptor{}).ParseTaskResult([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if info.UsageSummary != nil {
		t.Fatalf("failed 任务不应有 usage summary: %+v", info.UsageSummary)
	}
}
