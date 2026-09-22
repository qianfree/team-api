package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/qianfree/team-api/relay/common"
)

func TestParseAliVideoCreateRequest_Full(t *testing.T) {
	body := `{
		"model": "wan3.0-video",
		"input": {
			"prompt": "一只小猫在月光下的屋顶上奔跑",
			"media": [
				{"type": "first_frame", "url": "https://example.com/f.png"},
				{"type": "reference_image", "url": "https://example.com/r.png"}
			]
		},
		"parameters": {
			"resolution": "1080P", "ratio": "16:9", "duration": 5,
			"audio": false, "seed": 42, "prompt_extend": true, "watermark": false
		}
	}`
	req, taskErr := ParseAliVideoCreateRequest([]byte(body))
	if taskErr != nil {
		t.Fatalf("unexpected error: %+v", taskErr)
	}
	if req.Model != "wan3.0-video" || req.Prompt != "一只小猫在月光下的屋顶上奔跑" {
		t.Fatalf("unexpected request: %+v", req)
	}
	if len(req.Media) != 2 || req.Media[0].Type != "first_frame" || req.Media[1].URL != "https://example.com/r.png" {
		t.Fatalf("unexpected media: %+v", req.Media)
	}
	if req.Parameters.Resolution != "1080P" || req.Parameters.Ratio != "16:9" {
		t.Fatalf("unexpected parameters: %+v", req.Parameters)
	}
	if req.Parameters.Duration == nil || *req.Parameters.Duration != 5 {
		t.Fatalf("unexpected duration: %+v", req.Parameters.Duration)
	}
	if req.Parameters.Audio == nil || *req.Parameters.Audio {
		t.Fatalf("unexpected audio: %+v", req.Parameters.Audio)
	}
}

func TestParseAliVideoCreateRequest_LegacyWan2(t *testing.T) {
	// wan2.x 旧格式（扁平 input，无 media）同样可解析
	body := `{"model":"wan2.2-t2v-plus","input":{"prompt":"p","negative_prompt":"np","audio_url":"https://a.mp3"},"parameters":{"size":"1024*576"}}`
	req, taskErr := ParseAliVideoCreateRequest([]byte(body))
	if taskErr != nil {
		t.Fatalf("unexpected error: %+v", taskErr)
	}
	if req.Model != "wan2.2-t2v-plus" || req.Prompt != "p" || req.NegativePrompt != "np" || req.AudioURL != "https://a.mp3" {
		t.Fatalf("unexpected request: %+v", req)
	}
	if len(req.Media) != 0 {
		t.Fatalf("wan2 request must not carry media: %+v", req.Media)
	}
}

func TestParseAliVideoCreateRequest_InvalidJSON(t *testing.T) {
	if _, taskErr := ParseAliVideoCreateRequest([]byte(`{invalid`)); taskErr == nil || taskErr.StatusCode != 400 {
		t.Fatalf("expected 400 task error, got %+v", taskErr)
	}
}

func TestValidateAliVideoCreateRequest(t *testing.T) {
	// 缺 model
	if taskErr := ValidateAliVideoCreateRequest(&aliVideoCreateRequest{Prompt: "p"}); taskErr == nil {
		t.Error("missing model should be rejected")
	}
	// prompt 与 media 都缺（文档约束：必填其一）
	if taskErr := ValidateAliVideoCreateRequest(&aliVideoCreateRequest{Model: "wan3.0-video"}); taskErr == nil {
		t.Error("missing both prompt and media should be rejected")
	}
	// 纯 media（无 prompt）合法
	if taskErr := ValidateAliVideoCreateRequest(&aliVideoCreateRequest{
		Model: "wan3.0-video",
		Media: []aliVideoMedia{{Type: "reference_video", URL: "https://v.mp4"}},
	}); taskErr != nil {
		t.Errorf("media-only request should be accepted: %+v", taskErr)
	}
}

func TestBuildAliVideoTaskBody(t *testing.T) {
	duration := 5
	audio := false
	body, err := BuildAliVideoTaskBody(&aliVideoCreateRequest{
		Model:  "wan3.0-video",
		Prompt: "p",
		Media:  []aliVideoMedia{{Type: "first_frame", URL: "https://f.png"}},
		Parameters: aliVideoParameters{
			Resolution: "480P", Ratio: "16:9", Duration: &duration, Audio: &audio,
		},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// 顶层保留：model/prompt/media/通用 seconds
	if m["model"] != "wan3.0-video" || m["prompt"] != "p" || m["seconds"] != "5" {
		t.Fatalf("unexpected top-level body: %s", body)
	}
	media, ok := m["media"].([]any)
	if !ok || len(media) != 1 {
		t.Fatalf("expected media passthrough: %s", body)
	}
	// metadata 双写：duration/resolution/ratio/sound/generate_audio
	meta, _ := m["metadata"].(map[string]any)
	if meta["duration"] != float64(5) || meta["resolution"] != "480P" || meta["ratio"] != "16:9" {
		t.Fatalf("unexpected metadata: %v", meta)
	}
	if meta["sound"] != "off" || meta["generate_audio"] != false {
		t.Fatalf("audio switch not normalized: %v", meta)
	}
}

func TestBuildAliVideoTaskBody_SmartDuration(t *testing.T) {
	// duration=-1（智能时长）：metadata 保留 -1 信号（适配器预扣按 30s 上限冻结），不写 seconds
	minusOne := -1
	body, err := BuildAliVideoTaskBody(&aliVideoCreateRequest{
		Model:      "wan3.0-video",
		Prompt:     "p",
		Parameters: aliVideoParameters{Duration: &minusOne},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(body, &m)
	if _, has := m["seconds"]; has {
		t.Fatalf("smart duration must not emit seconds: %s", body)
	}
	meta, _ := m["metadata"].(map[string]any)
	if meta["duration"] != float64(-1) {
		t.Fatalf("smart duration signal lost: %v", meta)
	}
}

func TestBuildAliTaskResponse(t *testing.T) {
	now := time.Now()
	submitted := now.Add(-time.Minute)

	// 终态成功：回放上游查询响应（usage/orig_prompt 保真），覆写 task_id/status
	task := &common.AsyncTask{
		PublicTaskID: "task_pub_1",
		RequestID:    "req_1",
		Platform:     "ali",
		Status:       "SUCCESS",
		CreatedAt:    submitted,
		Data:         []byte(`{"output":{"task_id":"up-stream-id","task_status":"RUNNING","video_url":"https://v.mp4","orig_prompt":"a cat"},"usage":{"video_count":1,"duration":5.0,"output_video_duration":5.13,"fps":30,"SR":720,"ratio":"16:9"},"request_id":"up-req"}`),
	}
	resp := buildAliTaskResponse(task)
	if resp.Output.TaskID != "task_pub_1" || resp.Output.TaskStatus != "SUCCEEDED" {
		t.Fatalf("unexpected replay: %+v", resp.Output)
	}
	if resp.Output.VideoURL != "https://v.mp4" || resp.Output.OrigPrompt != "a cat" {
		t.Fatalf("upstream fields must be preserved: %+v", resp.Output)
	}
	var usage map[string]any
	if err := json.Unmarshal(resp.Usage, &usage); err != nil {
		t.Fatalf("usage not preserved: %v", err)
	}
	if usage["output_video_duration"] != 5.13 || usage["SR"] != float64(720) {
		t.Fatalf("usage metering lost: %v", usage)
	}
	if resp.RequestID != "req_1" {
		t.Fatalf("request id should be platform's: %s", resp.RequestID)
	}

	// 非终态（提交后首拍，Data 为提交响应形态）：构造 PENDING
	pending := &common.AsyncTask{
		PublicTaskID: "task_pub_2",
		RequestID:    "req_2",
		Platform:     "ali",
		Status:       "SUBMITTED",
		Data:         []byte(`{"output":{"task_status":"PENDING","task_id":"up-2"},"request_id":"up-req-2"}`),
	}
	resp = buildAliTaskResponse(pending)
	if resp.Output.TaskID != "task_pub_2" || resp.Output.TaskStatus != "PENDING" {
		t.Fatalf("unexpected pending replay: %+v", resp.Output)
	}

	// Data 为空：构造最小响应
	empty := &common.AsyncTask{PublicTaskID: "task_pub_3", RequestID: "req_3", Platform: "ali", Status: "IN_PROGRESS"}
	resp = buildAliTaskResponse(empty)
	if resp.Output.TaskID != "task_pub_3" || resp.Output.TaskStatus != "RUNNING" {
		t.Fatalf("unexpected empty-data replay: %+v", resp.Output)
	}

	// 失败任务：上游 code/message 保留；缺失时用平台 FailReason 兜底
	failed := &common.AsyncTask{
		PublicTaskID: "task_pub_4", RequestID: "req_4", Platform: "ali", Status: "FAILURE",
		FailReason: "upstream quota exceeded",
		Data:       []byte(`{"output":{"task_id":"up-4","task_status":"FAILED","code":"Throttling"}}`),
	}
	resp = buildAliTaskResponse(failed)
	if resp.Output.Code != "Throttling" || resp.Output.Message != "upstream quota exceeded" {
		t.Fatalf("unexpected failure replay: %+v", resp.Output)
	}
}
