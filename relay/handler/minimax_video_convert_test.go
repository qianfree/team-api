package handler

import (
	"encoding/json"
	"testing"
)

// MiniMax 官方创建请求的解析 / 校验 / 通用任务体转换单测。

func TestParseMiniMaxVideoCreateRequest_Full(t *testing.T) {
	body := `{
		"model": "MiniMax-H3",
		"content": [
			{"type": "text", "text": "海浪拍打礁石"},
			{"type": "image_url", "image_url": {"url": "https://cdn.example/a.png"}, "role": "first_frame"}
		],
		"resolution": "2K",
		"duration": 8,
		"ratio": "16:9",
		"aigc_watermark": true,
		"callback_url": "https://example.com/cb"
	}`
	req, taskErr := ParseMiniMaxVideoCreateRequest([]byte(body))
	if taskErr != nil {
		t.Fatalf("parse: %+v", taskErr)
	}
	if req.Model != "MiniMax-H3" || req.Prompt != "海浪拍打礁石" || req.Resolution != "2K" || req.Duration != 8 || req.Ratio != "16:9" {
		t.Fatalf("parsed fields: %+v", req)
	}
	if req.AigcWatermark == nil || !*req.AigcWatermark {
		t.Fatalf("aigc_watermark: %v", req.AigcWatermark)
	}
	if len(req.Content) != 2 {
		t.Fatalf("content items: %d", len(req.Content))
	}
}

func TestParseMiniMaxVideoCreateRequest_DurationString(t *testing.T) {
	// duration 宽松兼容字符串数字
	req, taskErr := ParseMiniMaxVideoCreateRequest([]byte(`{"model":"MiniMax-H3","content":[{"type":"text","text":"hi"}],"resolution":"768P","duration":"6"}`))
	if taskErr != nil || req.Duration != 6 {
		t.Fatalf("duration string parse: %+v err=%+v", req, taskErr)
	}
	// 非法 duration 报 400
	if _, taskErr := ParseMiniMaxVideoCreateRequest([]byte(`{"model":"MiniMax-H3","content":[{"type":"text","text":"hi"}],"resolution":"768P","duration":"abc"}`)); taskErr == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestParseMiniMaxVideoCreateRequest_InvalidContent(t *testing.T) {
	if _, taskErr := ParseMiniMaxVideoCreateRequest([]byte(`{"model":"MiniMax-H3","content":{"type":"text"},"resolution":"768P","duration":6}`)); taskErr == nil {
		t.Fatal("expected error when content is not an array")
	}
}

func TestValidateMiniMaxVideoCreateRequest(t *testing.T) {
	valid := &minimaxVideoCreateRequest{
		Model: "MiniMax-H3", Prompt: "hi", Content: []any{map[string]any{"type": "text", "text": "hi"}},
		Resolution: "768P", Duration: 6,
	}
	if taskErr := ValidateMiniMaxVideoCreateRequest(valid); taskErr != nil {
		t.Fatalf("valid request should pass: %+v", taskErr)
	}

	cases := map[string]*minimaxVideoCreateRequest{
		"missing model":      {Prompt: "hi", Resolution: "768P", Duration: 6},
		"missing content":    {Model: "MiniMax-H3", Prompt: "hi", Resolution: "768P", Duration: 6},
		"missing text item":  {Model: "MiniMax-H3", Resolution: "768P", Duration: 6},
		"missing resolution": {Model: "MiniMax-H3", Prompt: "hi", Duration: 6},
		"missing duration":   {Model: "MiniMax-H3", Prompt: "hi", Resolution: "768P"},
	}
	for name, req := range cases {
		if taskErr := ValidateMiniMaxVideoCreateRequest(req); taskErr == nil {
			t.Errorf("%s: expected 400", name)
		}
	}
}

func TestBuildMiniMaxVideoTaskBody(t *testing.T) {
	req := &minimaxVideoCreateRequest{
		Model:  "MiniMax-H3",
		Prompt: "海浪拍打礁石",
		Content: []any{
			map[string]any{"type": "text", "text": "海浪拍打礁石"},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://cdn.example/a.png"}, "role": "first_frame"},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://cdn.example/ref.png"}, "role": "reference_image"},
		},
		Resolution: "768P",
		Duration:   6,
		Ratio:      "16:9",
	}
	body, err := BuildMiniMaxVideoTaskBody(req)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// 原生字段保留
	if m["model"] != "MiniMax-H3" || m["prompt"] != "海浪拍打礁石" || m["resolution"] != "768P" || m["duration"] != float64(6) || m["ratio"] != "16:9" {
		t.Fatalf("native fields: %v", m)
	}
	if _, has := m["callback_url"]; has {
		t.Fatal("callback_url must never enter the task body")
	}

	// 通用词汇双写
	if m["seconds"] != "6" {
		t.Fatalf("seconds dual-write: %v", m["seconds"])
	}
	images, _ := m["images"].([]any)
	// 只提取 first_frame / 无 role 的图片，reference_image 不算图生视频输入
	if len(images) != 1 || images[0] != "https://cdn.example/a.png" {
		t.Fatalf("images dual-write: %v", m["images"])
	}
	meta, _ := m["metadata"].(map[string]any)
	if meta["resolution"] != "768P" || meta["duration"] != float64(6) || meta["ratio"] != "16:9" {
		t.Fatalf("metadata dual-write: %v", m["metadata"])
	}
	// content 原样保留（3 项）
	if content, _ := m["content"].([]any); len(content) != 3 {
		t.Fatalf("content preserved: %v", m["content"])
	}
}

func TestBuildMiniMaxVideoTaskBody_Minimal(t *testing.T) {
	// 无 ratio / 无图片：ratio 与 images 键不产出
	req := &minimaxVideoCreateRequest{
		Model: "MiniMax-H3", Prompt: "hi",
		Content:    []any{map[string]any{"type": "text", "text": "hi"}},
		Resolution: "768P", Duration: 4,
	}
	body, err := BuildMiniMaxVideoTaskBody(req)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(body, &m)
	if _, has := m["ratio"]; has {
		t.Fatalf("ratio should be absent when not specified: %v", m)
	}
	if _, has := m["images"]; has {
		t.Fatalf("images should be absent without image input: %v", m)
	}
}
