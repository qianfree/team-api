package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

// buildEditsMultipart 构造官方 SDK / curl 形态的 /v1/images/edits multipart 表单：
// 标量字段 + 多张 image[] 文件 part + mask 文件 part。
func buildEditsMultipart(t *testing.T, fields map[string]string, files map[string][]byte) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for name, val := range fields {
		if err := w.WriteField(name, val); err != nil {
			t.Fatalf("write field %s: %v", name, err)
		}
	}
	for name, content := range files {
		fw, err := w.CreateFormFile(name, name+".png")
		if err != nil {
			t.Fatalf("create file %s: %v", name, err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatalf("write file %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

func TestParseRequestBodyFields_JSON(t *testing.T) {
	body := []byte(`{"model":"gpt-image-1","prompt":"a cat","stream":false,"max_tokens":100}`)
	headers := http.Header{"Content-Type": []string{"application/json"}}

	fields, err := parseRequestBodyFields(body, headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var model string
	if err := json.Unmarshal(fields["model"], &model); err != nil || model != "gpt-image-1" {
		t.Fatalf("model = %q, err = %v", model, err)
	}
}

// JSON 请求未带 Content-Type 时按 JSON 解析（保持旧行为）
func TestParseRequestBodyFields_JSONWithoutContentType(t *testing.T) {
	fields, err := parseRequestBodyFields([]byte(`{"model":"dall-e-2"}`), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var model string
	if err := json.Unmarshal(fields["model"], &model); err != nil || model != "dall-e-2" {
		t.Fatalf("model = %q, err = %v", model, err)
	}
}

// 官方 SDK / curl 形态：image[] 多文件 + mask 文件 + 标量字段，模型名可正常提取
func TestParseRequestBodyFields_MultipartImagesEdits(t *testing.T) {
	body, contentType := buildEditsMultipart(t,
		map[string]string{"model": "gpt-image-1", "prompt": "a cat on sofa", "size": "1024x1024"},
		map[string][]byte{
			"image[]": []byte("fake-png-bytes-1"),
			"mask":    []byte("fake-mask-bytes"),
		},
	)
	headers := http.Header{"Content-Type": []string{contentType}}

	fields, err := parseRequestBodyFields(body, headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var model string
	if err := json.Unmarshal(fields["model"], &model); err != nil || model != "gpt-image-1" {
		t.Fatalf("model = %q, err = %v", model, err)
	}
	// 白名单外的标量字段（prompt/size）不收集；文件 part 更不进 map
	for _, key := range []string{"prompt", "size", "image[]", "mask"} {
		if _, ok := fields[key]; ok {
			t.Errorf("unexpected field %q collected", key)
		}
	}
}

// 音频转写同形态：file 文件 part + model 标量
func TestParseRequestBodyFields_MultipartAudioTranscription(t *testing.T) {
	body, contentType := buildEditsMultipart(t,
		map[string]string{"model": "whisper-1"},
		map[string][]byte{"file": []byte("fake-audio-bytes")},
	)
	headers := http.Header{"Content-Type": []string{contentType}}

	fields, err := parseRequestBodyFields(body, headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var model string
	if err := json.Unmarshal(fields["model"], &model); err != nil || model != "whisper-1" {
		t.Fatalf("model = %q, err = %v", model, err)
	}
}

func TestParseRequestBodyFields_MultipartMissingBoundary(t *testing.T) {
	headers := http.Header{"Content-Type": []string{"multipart/form-data"}}
	_, err := parseRequestBodyFields([]byte("--x\r\n"), headers)
	if err == nil || !strings.Contains(err.Error(), "boundary") {
		t.Fatalf("want boundary error, got %v", err)
	}
}

// 坏 JSON 体（如把 multipart 发给了声明 JSON 的端点）仍返回解析错误，
// 由上层包装为 400 invalid request body
func TestParseRequestBodyFields_InvalidJSON(t *testing.T) {
	headers := http.Header{"Content-Type": []string{"application/json"}}
	if _, err := parseRequestBodyFields([]byte("--boundary\r\nnot-json"), headers); err == nil {
		t.Fatal("want parse error for non-JSON body with JSON content type")
	}
}
