package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

// buildMultipartVideosBody 构造 multipart/form-data 的创建请求体（模拟官方 SDK 形态）。
// 文件 part 的 Content-Type 统一声明为 application/octet-stream，以覆盖内容类型嗅探路径。
func buildMultipartVideosBody(t *testing.T, fields map[string]string, fileField, fileName string, fileBytes []byte) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	if fileField != "" {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fileField, fileName))
		h.Set("Content-Type", "application/octet-stream")
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatalf("create file part: %v", err)
		}
		if _, err := part.Write(fileBytes); err != nil {
			t.Fatalf("write file part: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

func TestParseVideosCreateRequest_JSON(t *testing.T) {
	body := `{"model":"kling-v2-master","prompt":"a cat","seconds":"8","size":"1280x720"}`
	req, taskErr := ParseVideosCreateRequest([]byte(body), "application/json")
	if taskErr != nil {
		t.Fatalf("unexpected error: %+v", taskErr)
	}
	if req.Model != "kling-v2-master" || req.Prompt != "a cat" || req.Seconds != "8" || req.Size != "1280x720" {
		t.Fatalf("unexpected request: %+v", req)
	}
}

func TestParseVideosCreateRequest_JSONSecondsNumber(t *testing.T) {
	body := `{"model":"m","prompt":"p","seconds":8}`
	req, taskErr := ParseVideosCreateRequest([]byte(body), "application/json; charset=utf-8")
	if taskErr != nil {
		t.Fatalf("unexpected error: %+v", taskErr)
	}
	if req.Seconds != "8" {
		t.Fatalf("expected seconds normalized to \"8\", got %q", req.Seconds)
	}
}

func TestParseVideosCreateRequest_JSONInvalidSeconds(t *testing.T) {
	body := `{"model":"m","prompt":"p","seconds":true}`
	_, taskErr := ParseVideosCreateRequest([]byte(body), "application/json")
	if taskErr == nil || taskErr.StatusCode != 400 {
		t.Fatalf("expected 400 for bool seconds, got %+v", taskErr)
	}
}

// TestParseVideosCreateRequest_DeprecatedFieldsRejected 已移除字段（input_reference/sound/
// generate_audio）命中即 400 并给出替代指引，不静默忽略——静默丢弃首帧图会把图生视频
// 当文生视频、按请求计费按默认档生成
func TestParseVideosCreateRequest_DeprecatedFieldsRejected(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"input_reference object", `{"model":"m","prompt":"p","input_reference":{"image_url":"https://example.com/ref.png"}}`},
		{"input_reference file_id", `{"model":"m","prompt":"p","input_reference":{"file_id":"file-abc12345"}}`},
		{"sound string", `{"model":"m","prompt":"p","sound":"on"}`},
		{"sound bool", `{"model":"m","prompt":"p","sound":true}`},
		{"generate_audio", `{"model":"m","prompt":"p","generate_audio":true}`},
		{"watermark bool", `{"model":"m","prompt":"p","watermark":true}`},
		{"watermark string", `{"model":"m","prompt":"p","watermark":"on"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, taskErr := ParseVideosCreateRequest([]byte(c.body), "application/json")
			if taskErr == nil || taskErr.StatusCode != 400 || taskErr.ErrCode != "field_deprecated" {
				t.Fatalf("expected 400 field_deprecated, got %+v", taskErr)
			}
		})
	}

	// multipart 形态：文件 part、字符串 part、brackets 编码与 sound/generate_audio 同样拒绝
	pngBytes := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	for name, fields := range map[string]map[string]string{
		"file part":      {"model": "m", "prompt": "p", "input_reference": `{"image_url":"https://x"}`},
		"brackets编码":     {"model": "m", "prompt": "p", "input_reference[image_url]": "https://x"},
		"sound":          {"model": "m", "prompt": "p", "sound": "on"},
		"generate_audio": {"model": "m", "prompt": "p", "generate_audio": "true"},
		"watermark":      {"model": "m", "prompt": "p", "watermark": "true"},
	} {
		t.Run("multipart "+name, func(t *testing.T) {
			fileField := ""
			if name == "file part" {
				fileField = "input_reference"
				delete(fields, "input_reference")
			}
			body, ct := buildMultipartVideosBody(t, fields, fileField, "ref.png", pngBytes)
			_, taskErr := ParseVideosCreateRequest(body, ct)
			if taskErr == nil || taskErr.StatusCode != 400 || taskErr.ErrCode != "field_deprecated" {
				t.Fatalf("expected 400 field_deprecated, got %+v", taskErr)
			}
		})
	}
}

// TestBuildVideosTaskBody_SizePassthrough size 只借协议字段形态、不做档位换算：
// 原值进 metadata.size，不派生 aspect_ratio/ratio/resolution（值词汇由调用方按目标供应商原生口径提供）。
func TestBuildVideosTaskBody_SizePassthrough(t *testing.T) {
	req := &videosCreateRequest{
		Model:      "kling-v2-master",
		Prompt:     "a cat",
		Seconds:    "8",
		Size:       "768P",
		References: []common.VideoReference{{Role: common.RefRoleFirstFrame, URL: "https://example.com/ref.png"}},
	}
	body, err := BuildVideosTaskBody(req)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed["model"] != "kling-v2-master" || parsed["prompt"] != "a cat" {
		t.Fatalf("unexpected body: %s", body)
	}
	if parsed["seconds"] != "8" {
		t.Fatalf("expected top-level seconds \"8\", got %v", parsed["seconds"])
	}

	meta, ok := parsed["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata object: %s", body)
	}
	if meta["duration"] != float64(8) {
		t.Fatalf("expected metadata.duration 8, got %v", meta["duration"])
	}
	if meta["size"] != "768P" {
		t.Fatalf("expected raw size passthrough, got %v", meta["size"])
	}
	// 不做换算：不从 size 派生任何其他参数键
	for _, key := range []string{"aspect_ratio", "ratio", "resolution"} {
		if _, has := meta[key]; has {
			t.Fatalf("size must not derive %s: %s", key, body)
		}
	}
	if meta["image"] != "https://example.com/ref.png" {
		t.Fatalf("expected metadata.image, got %v", meta["image"])
	}
	images, ok := parsed["images"].([]any)
	if !ok || len(images) != 1 || images[0] != "https://example.com/ref.png" {
		t.Fatalf("expected root images array with reference, got %v", parsed["images"])
	}
}

func TestBuildVideosTaskBody_Minimal(t *testing.T) {
	req := &videosCreateRequest{Model: "m", Prompt: "p"}
	body, err := BuildVideosTaskBody(req)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := parsed["seconds"]; ok {
		t.Fatal("expected no seconds field")
	}
	if _, ok := parsed["images"]; ok {
		t.Fatal("expected no images field without first_frame reference")
	}
	if _, ok := parsed["metadata"]; ok {
		t.Fatal("expected no metadata field")
	}
}

func TestBuildVideosTaskBody_UnknownSizeNoRatio(t *testing.T) {
	req := &videosCreateRequest{Model: "m", Prompt: "p", Size: "1920x1080"}
	body, _ := BuildVideosTaskBody(req)
	var parsed map[string]any
	json.Unmarshal(body, &parsed)
	meta := parsed["metadata"].(map[string]any)
	if _, ok := meta["aspect_ratio"]; ok {
		t.Fatal("unknown size must not map aspect_ratio")
	}
	if _, ok := meta["ratio"]; ok {
		t.Fatal("unknown size must not map ratio")
	}
	if _, ok := meta["resolution"]; ok {
		t.Fatal("unknown size must not map resolution")
	}
	if meta["size"] != "1920x1080" {
		t.Fatalf("expected raw size passthrough, got %v", meta["size"])
	}
}

func TestValidateVideosCreateRequest(t *testing.T) {
	if taskErr := ValidateVideosCreateRequest(&videosCreateRequest{Model: "m", Prompt: "p"}); taskErr != nil {
		t.Fatalf("unexpected: %+v", taskErr)
	}
	if taskErr := ValidateVideosCreateRequest(&videosCreateRequest{Prompt: "p"}); taskErr == nil || taskErr.Message != "model is required" {
		t.Fatalf("expected model required, got %+v", taskErr)
	}
	if taskErr := ValidateVideosCreateRequest(&videosCreateRequest{Model: "m", Prompt: "  "}); taskErr == nil || taskErr.Message != "prompt is required" {
		t.Fatalf("expected prompt required, got %+v", taskErr)
	}
}

// TestParseVideosCreateRequest_JSONAspectRatio 扩展字段（aspect_ratio）解析
func TestParseVideosCreateRequest_JSONAspectRatio(t *testing.T) {
	req, taskErr := ParseVideosCreateRequest(
		[]byte(`{"model":"m","prompt":"p","aspect_ratio":"9:16"}`), "application/json")
	if taskErr != nil {
		t.Fatalf("unexpected error: %+v", taskErr)
	}
	if req.AspectRatio != "9:16" || req.Sound != "" {
		t.Fatalf("unexpected request: %+v", req)
	}
}

// TestParseVideosCreateRequest_MultipartAspectRatio multipart 裸字符串 part 走同一归一逻辑
func TestParseVideosCreateRequest_MultipartAspectRatio(t *testing.T) {
	body, ct := buildMultipartVideosBody(t, map[string]string{
		"model":        "kling-v2-master",
		"prompt":       "a cat",
		"aspect_ratio": "1:1",
	}, "", "", nil)

	req, taskErr := ParseVideosCreateRequest(body, ct)
	if taskErr != nil {
		t.Fatalf("parse: %+v", taskErr)
	}
	if req.AspectRatio != "1:1" {
		t.Fatalf("unexpected request: %+v", req)
	}
}

// TestBuildVideosTaskBody_AspectRatioAndSoundDualWrite 比例与音频开关需按各上游词汇双写
func TestBuildVideosTaskBody_AspectRatioAndSoundDualWrite(t *testing.T) {
	req := &videosCreateRequest{
		Model:       "kling-v2-master",
		Prompt:      "a cat",
		AspectRatio: "9:16",
		Sound:       "on",
	}
	body, err := BuildVideosTaskBody(req)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// minimax 的 resolveRatio 顶层 ratio 优先，必须写
	if parsed["ratio"] != "9:16" {
		t.Fatalf("expected top-level ratio, got %v", parsed["ratio"])
	}

	meta := parsed["metadata"].(map[string]any)
	if meta["ratio"] != "9:16" || meta["aspect_ratio"] != "9:16" {
		t.Fatalf("expected metadata ratio + aspect_ratio, got %v", meta)
	}
	if meta["sound"] != "on" {
		t.Fatalf("expected metadata.sound on, got %v", meta["sound"])
	}
	if meta["generate_audio"] != true {
		t.Fatalf("expected metadata.generate_audio true, got %v", meta["generate_audio"])
	}
}

// TestBuildVideosTaskBody_SoundOff 关闭音频时 generate_audio 必须为 false（而非缺省）
func TestBuildVideosTaskBody_SoundOff(t *testing.T) {
	body, _ := BuildVideosTaskBody(&videosCreateRequest{Model: "m", Prompt: "p", Sound: "off"})
	var parsed map[string]any
	json.Unmarshal(body, &parsed)
	meta := parsed["metadata"].(map[string]any)
	if meta["generate_audio"] != false {
		t.Fatalf("expected generate_audio false, got %v", meta["generate_audio"])
	}
	if _, ok := parsed["ratio"]; ok {
		t.Fatal("expected no ratio without aspect_ratio")
	}
}

// TestBuildVideosTaskBody_ExtraSoundPromoted /v1/videos 已移除 sound/generate_audio 顶层
// 字段，厂商音频控制走 extra：已知键提升进任务体 metadata（适配器/JS 插件按现有路径
// 消费），并从 extra 剔除（不平铺、不重复下发）
func TestBuildVideosTaskBody_ExtraSoundPromoted(t *testing.T) {
	req := &videosCreateRequest{
		Model:  "kling-v2-master",
		Prompt: "p",
		Extra:  json.RawMessage(`{"sound":"on","generate_audio":true,"custom_key":"v"}`),
	}
	body, err := BuildVideosTaskBody(req)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	meta := parsed["metadata"].(map[string]any)
	if meta["sound"] != "on" || meta["generate_audio"] != true {
		t.Fatalf("expected sound/generate_audio promoted into metadata, got %v", meta)
	}
	extra, _ := parsed["extra"].(map[string]any)
	if _, ok := extra["sound"]; ok {
		t.Fatalf("sound must be removed from extra after promotion: %s", body)
	}
	if extra["custom_key"] != "v" {
		t.Fatalf("unknown keys must stay in extra: %s", body)
	}
}

// TestBuildVideosTaskBody_Watermark 顶层收敛词汇 watermark 写 metadata.watermark；
// 与 extra 提升值（一致，由解析层冲突检查保证）共存时顶层覆盖无歧义
func TestBuildVideosTaskBody_Watermark(t *testing.T) {
	wm := false
	body, _ := BuildVideosTaskBody(&videosCreateRequest{Model: "m", Prompt: "p", Watermark: &wm})
	var parsed map[string]any
	json.Unmarshal(body, &parsed)
	meta := parsed["metadata"].(map[string]any)
	if meta["watermark"] != false {
		t.Fatalf("expected metadata.watermark false, got %v", meta["watermark"])
	}
	if _, ok := parsed["watermark"]; ok {
		t.Fatal("watermark 不应写任务体顶层（仅 metadata 通道）")
	}

	// 未传时（无其他可选参数）整个 metadata 都不写入
	body, _ = BuildVideosTaskBody(&videosCreateRequest{Model: "m", Prompt: "p"})
	var parsedNoWM map[string]json.RawMessage
	json.Unmarshal(body, &parsedNoWM)
	if _, has := parsedNoWM["metadata"]; has {
		t.Fatalf("未传 watermark 不应有 metadata.watermark: %s", body)
	}
}

// TestNewVideosRequestEcho_ExtendedFields 回显需带上扩展字段（submit/retrieve 回填与排障用）
func TestNewVideosRequestEcho_ExtendedFields(t *testing.T) {
	echo := NewVideosRequestEcho(&videosCreateRequest{
		Prompt: "p", Seconds: "8", Size: "768P", AspectRatio: "16:9",
	})
	b, err := json.Marshal(echo)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	json.Unmarshal(b, &got)
	if got["aspect_ratio"] != "16:9" {
		t.Fatalf("unexpected echo: %s", b)
	}
	if _, has := got["sound"]; has {
		t.Fatalf("sound should not be echoed: %s", b)
	}
}
