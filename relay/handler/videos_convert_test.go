package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"
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
	body := `{"model":"kling-v2-master","prompt":"a cat","seconds":"8","size":"1280x720","input_reference":{"image_url":"https://example.com/ref.png"}}`
	req, taskErr := ParseVideosCreateRequest([]byte(body), "application/json")
	if taskErr != nil {
		t.Fatalf("unexpected error: %+v", taskErr)
	}
	if req.Model != "kling-v2-master" || req.Prompt != "a cat" || req.Seconds != "8" || req.Size != "1280x720" {
		t.Fatalf("unexpected request: %+v", req)
	}
	if req.InputReference != "https://example.com/ref.png" {
		t.Fatalf("unexpected input_reference: %s", req.InputReference)
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

func TestParseVideosCreateRequest_JSONFileIDRejected(t *testing.T) {
	body := `{"model":"m","prompt":"p","input_reference":{"file_id":"file-abc"}}`
	_, taskErr := ParseVideosCreateRequest([]byte(body), "application/json")
	if taskErr == nil {
		t.Fatal("expected file_id to be rejected")
	}
	if taskErr.StatusCode != 400 || taskErr.ErrCode != "file_id_not_supported" {
		t.Fatalf("unexpected error: %+v", taskErr)
	}
}

func TestParseVideosCreateRequest_MultipartWithFile(t *testing.T) {
	// PNG 魔数，DetectContentType 应识别为 image/png（part 声明为 octet-stream）
	pngBytes := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	body, ct := buildMultipartVideosBody(t,
		map[string]string{"model": "kling-v2-master", "prompt": "a cat", "seconds": "12", "size": "720x1280"},
		"input_reference", "ref.png", pngBytes)

	req, taskErr := ParseVideosCreateRequest(body, ct)
	if taskErr != nil {
		t.Fatalf("unexpected error: %+v", taskErr)
	}
	if req.Model != "kling-v2-master" || req.Seconds != "12" || req.Size != "720x1280" {
		t.Fatalf("unexpected request: %+v", req)
	}
	if !strings.HasPrefix(req.InputReference, "data:image/png;base64,") {
		t.Fatalf("expected data url with sniffed image/png, got %.40q", req.InputReference)
	}
}

func TestParseVideosCreateRequest_MultipartStringRef(t *testing.T) {
	body, ct := buildMultipartVideosBody(t,
		map[string]string{
			"model":           "m",
			"prompt":          "p",
			"input_reference": `{"image_url":"https://example.com/x.jpg"}`,
		}, "", "", nil)

	req, taskErr := ParseVideosCreateRequest(body, ct)
	if taskErr != nil {
		t.Fatalf("unexpected error: %+v", taskErr)
	}
	if req.InputReference != "https://example.com/x.jpg" {
		t.Fatalf("unexpected input_reference: %s", req.InputReference)
	}
}

// TestBuildVideosTaskBody_SizePassthrough size 只借协议字段形态、不做档位换算：
// 原值进 metadata.size，不派生 aspect_ratio/ratio/resolution（值词汇由调用方按目标供应商原生口径提供）。
func TestBuildVideosTaskBody_SizePassthrough(t *testing.T) {
	req := &videosCreateRequest{
		Model:          "kling-v2-master",
		Prompt:         "a cat",
		Seconds:        "8",
		Size:           "768P",
		InputReference: "https://example.com/ref.png",
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
		t.Fatal("expected no images field without input_reference")
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
