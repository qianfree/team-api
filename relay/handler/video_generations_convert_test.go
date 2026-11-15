package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

// 收敛层：废弃字段 400、prompt/model 必填、词汇归一进通用任务体（与 /v1/videos 同构）

func TestParseVideoGenerationsRequest_DeprecatedTopLevel(t *testing.T) {
	for _, field := range []string{
		"images", "seconds", "mode", "size", "ratio", "generate_audio",
		"first_frame_image", "last_frame_image", "subject_reference",
		"media", "content", "prompt_optimizer", "fast_pretreatment", "aigc_watermark",
	} {
		body := `{"model":"m","prompt":"p","` + field + `":1}`
		_, terr := ParseVideoGenerationsRequest([]byte(body))
		if terr == nil || terr.StatusCode != 400 || !strings.Contains(terr.Message, field) {
			t.Fatalf("%s: 应 400 且 message 指明字段, got %+v", field, terr)
		}
	}
}

func TestParseVideoGenerationsRequest_MetadataRejected(t *testing.T) {
	// metadata 已并入 extra（合并后的厂商参数唯一通道），字段本身废弃
	_, terr := ParseVideoGenerationsRequest([]byte(`{"model":"m","prompt":"p","metadata":{"mode":"pro"}}`))
	if terr == nil || terr.StatusCode != 400 || !strings.Contains(terr.Message, "metadata") {
		t.Fatalf("metadata 字段应 400: %+v", terr)
	}
}

func TestParseVideoGenerationsRequest_Required(t *testing.T) {
	if _, terr := ParseVideoGenerationsRequest([]byte(`{"prompt":"p"}`)); terr == nil || !strings.Contains(terr.Message, "model") {
		t.Fatalf("缺 model 应 400: %+v", terr)
	}
	if _, terr := ParseVideoGenerationsRequest([]byte(`{"model":"m"}`)); terr == nil || !strings.Contains(terr.Message, "prompt") {
		t.Fatalf("缺 prompt 应 400: %+v", terr)
	}
	// prompt 必填但允许空串（纯图生视频场景）
	req, terr := ParseVideoGenerationsRequest([]byte(`{"model":"m","prompt":""}`))
	if terr != nil || req.Prompt != "" {
		t.Fatalf("空串 prompt 应放行: %+v", terr)
	}
}

func TestParseVideoGenerationsRequest_CanonicalBody(t *testing.T) {
	body := `{
		"model": "kling-v2-master", "prompt": "小猫奔跑", "duration": 10, "resolution": "1080P",
		"aspect_ratio": "16:9", "sound": true,
		"references": [{"role": "first_frame", "url": "https://example.com/f.png"}],
		"extra": {"mode": "pro", "cfg_scale": 0.5, "service_tier": "priority"}
	}`
	req, terr := ParseVideoGenerationsRequest([]byte(body))
	if terr != nil {
		t.Fatalf("unexpected error: %+v", terr)
	}
	taskBody, err := BuildVideosTaskBody(req)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Seconds    string           `json:"seconds"`
		Ratio      string           `json:"ratio"`
		References []map[string]any `json:"references"`
		Images     []string         `json:"images"`
		Metadata   struct {
			Duration      any             `json:"duration"`
			Resolution    string          `json:"resolution"`
			Ratio         string          `json:"ratio"`
			AspectRatio   string          `json:"aspect_ratio"`
			Sound         string          `json:"sound"`
			GenerateAudio bool            `json:"generate_audio"`
			Image         string          `json:"image"`
			Mode          string          `json:"mode"`
			CfgScale      json.RawMessage `json:"cfg_scale"`
		} `json:"metadata"`
		Extra json.RawMessage `json:"extra"`
	}
	if err := json.Unmarshal(taskBody, &out); err != nil {
		t.Fatal(err)
	}
	if out.Seconds != "10" || out.Metadata.Duration != float64(10) {
		t.Fatalf("duration 归一: seconds=%s metadata.duration=%v", out.Seconds, out.Metadata.Duration)
	}
	if out.Metadata.Resolution != "1080P" {
		t.Fatalf("resolution: %+v", out.Metadata)
	}
	if out.Ratio != "16:9" || out.Metadata.Ratio != "16:9" || out.Metadata.AspectRatio != "16:9" {
		t.Fatalf("aspect_ratio 三写: %+v", out.Metadata)
	}
	if out.Metadata.Sound != "on" || !out.Metadata.GenerateAudio {
		t.Fatalf("sound 双写: %+v", out.Metadata)
	}
	if out.Metadata.Mode != "pro" || string(out.Metadata.CfgScale) != "0.5" {
		t.Fatalf("已知厂商键应提升进 metadata: %+v", out.Metadata)
	}
	// references + 首帧兼容双写（喂老适配器/JS 插件）
	if len(out.References) != 1 || out.References[0]["role"] != "first_frame" {
		t.Fatalf("references: %+v", out.References)
	}
	if len(out.Images) != 1 || out.Images[0] != "https://example.com/f.png" || out.Metadata.Image != "https://example.com/f.png" {
		t.Fatalf("首帧兼容双写: images=%v metadata.image=%s", out.Images, out.Metadata.Image)
	}
	var extraKeys map[string]json.RawMessage
	if err := json.Unmarshal(out.Extra, &extraKeys); err != nil {
		t.Fatal(err)
	}
	if _, still := extraKeys["mode"]; still {
		t.Fatalf("已知键应从 extra 剔除: %s", out.Extra)
	}
	if st := extraKeys["service_tier"]; string(st) != `"priority"` {
		t.Fatalf("未知键应留在 extra: %s", out.Extra)
	}
}

// 已知键全部提升后 extra 为空 → 任务体不写 extra 键
func TestParseVideoGenerationsRequest_ExtraFullyPromoted(t *testing.T) {
	req, terr := ParseVideoGenerationsRequest([]byte(`{"model":"m","prompt":"p","extra":{"mode":"pro"}}`))
	if terr != nil {
		t.Fatal(terr)
	}
	taskBody, err := BuildVideosTaskBody(req)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]json.RawMessage
	_ = json.Unmarshal(taskBody, &out)
	if _, ok := out["extra"]; ok {
		t.Fatalf("全提升后不应有 extra: %s", out["extra"])
	}
	var meta struct {
		Mode string `json:"mode"`
	}
	_ = json.Unmarshal(out["metadata"], &meta)
	if meta.Mode != "pro" {
		t.Fatalf("mode 应在 metadata: %s", out["metadata"])
	}
}

func TestParseVideoGenerationsRequest_LenientForms(t *testing.T) {
	// duration 数字字符串、sound "off"、无可选字段
	req, terr := ParseVideoGenerationsRequest([]byte(`{"model":"m","prompt":"p","duration":"5","sound":"off"}`))
	if terr != nil || req.Seconds != "5" || req.Sound != "off" {
		t.Fatalf("宽松形态: seconds=%s sound=%s terr=%+v", req.Seconds, req.Sound, terr)
	}
	// -1 智能时长保留
	req, terr = ParseVideoGenerationsRequest([]byte(`{"model":"m","prompt":"p","duration":-1}`))
	if terr != nil || req.Seconds != "-1" {
		t.Fatalf("智能时长 -1: seconds=%s terr=%+v", req.Seconds, terr)
	}
	// 非法 duration 类型
	if _, terr := ParseVideoGenerationsRequest([]byte(`{"model":"m","prompt":"p","duration":"abc"}`)); terr == nil {
		t.Fatal("非法 duration 应 400")
	}
	// references 校验复用：非法 role
	if _, terr := ParseVideoGenerationsRequest([]byte(`{"model":"m","prompt":"p","references":[{"role":"bad","url":"https://x.com/a.png"}]}`)); terr == nil {
		t.Fatal("非法 role 应 400")
	}
}

// watermark：顶层 bool/宽松词汇归一进 metadata.watermark；与 extra.watermark 同传
// 必须一致（不静默覆盖）；非法值 400
func TestParseVideoGenerationsRequest_Watermark(t *testing.T) {
	// bool 归一进 metadata
	req, terr := ParseVideoGenerationsRequest([]byte(`{"model":"m","prompt":"p","watermark":true}`))
	if terr != nil || req.Watermark == nil || !*req.Watermark {
		t.Fatalf("watermark bool: %+v terr=%+v", req.Watermark, terr)
	}
	body, err := BuildVideosTaskBody(req)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Metadata struct {
			Watermark *bool `json:"watermark"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out.Metadata.Watermark == nil || !*out.Metadata.Watermark {
		t.Fatalf("metadata.watermark: %+v", out.Metadata)
	}

	// 宽松词汇归一："on"→true、"0"→false、null→未指定
	for _, c := range []struct {
		raw   string
		want  bool
		isNil bool
	}{
		{`"on"`, true, false},
		{`"true"`, true, false},
		{`"1"`, true, false},
		{`"off"`, false, false},
		{`"0"`, false, false},
		{`null`, false, true},
	} {
		req, terr := ParseVideoGenerationsRequest([]byte(`{"model":"m","prompt":"p","watermark":` + c.raw + `}`))
		if terr != nil {
			t.Fatalf("watermark %s: %+v", c.raw, terr)
		}
		if c.isNil {
			if req.Watermark != nil {
				t.Fatalf("watermark %s 应未指定: %+v", c.raw, req.Watermark)
			}
			continue
		}
		if req.Watermark == nil || *req.Watermark != c.want {
			t.Fatalf("watermark %s 归一: %+v", c.raw, req.Watermark)
		}
	}

	// 非法值
	if _, terr := ParseVideoGenerationsRequest([]byte(`{"model":"m","prompt":"p","watermark":"abc"}`)); terr == nil || terr.StatusCode != 400 {
		t.Fatalf("非法 watermark 应 400: %+v", terr)
	}

	// 顶层与 extra 同传：冲突 400，一致放行且 extra 中剔除
	if _, terr := ParseVideoGenerationsRequest([]byte(`{"model":"m","prompt":"p","watermark":true,"extra":{"watermark":false}}`)); terr == nil || !strings.Contains(terr.Message, "conflicting") {
		t.Fatalf("冲突应 400: %+v", terr)
	}
	req, terr = ParseVideoGenerationsRequest([]byte(`{"model":"m","prompt":"p","watermark":true,"extra":{"watermark":true,"custom":"v"}}`))
	if terr != nil {
		t.Fatalf("一致应放行: %+v", terr)
	}
	body, err = BuildVideosTaskBody(req)
	if err != nil {
		t.Fatal(err)
	}
	var out2 struct {
		Metadata struct {
			Watermark *bool `json:"watermark"`
		} `json:"metadata"`
		Extra map[string]json.RawMessage `json:"extra"`
	}
	if err := json.Unmarshal(body, &out2); err != nil {
		t.Fatal(err)
	}
	if out2.Metadata.Watermark == nil || !*out2.Metadata.Watermark {
		t.Fatalf("一致提升后 metadata.watermark: %+v", out2.Metadata)
	}
	if _, still := out2.Extra["watermark"]; still {
		t.Fatalf("watermark 应从 extra 剔除: %s", body)
	}
	if string(out2.Extra["custom"]) != `"v"` {
		t.Fatalf("未知键应留在 extra: %s", body)
	}
}
