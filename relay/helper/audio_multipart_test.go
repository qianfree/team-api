package helper

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

// buildAudioMultipart 构造 Whisper 风格的 /v1/audio/transcriptions multipart 表单：
// model 标量字段 + file 文件 part。
func buildAudioMultipart(t *testing.T, model string, fileContent []byte) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("model", model); err != nil {
		t.Fatalf("write model field: %v", err)
	}
	fw, err := w.CreateFormFile("file", "audio.mp3")
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := fw.Write(fileContent); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

// multipart 模型改写：model 字段替换为上游模型名/接入点，file part 字节保持不变，
// boundary 保持不变（转发头中的 Content-Type 才能继续匹配）。
func TestRewriteAudioMultipartModel(t *testing.T) {
	audioPayload := []byte("binary-audio-bytes-\x00\x01\x02")
	body, contentType := buildAudioMultipart(t, "whisper-1", audioPayload)
	info := &common.RelayInfo{
		RequestHeaders: http.Header{"Content-Type": {contentType}},
		ChannelMeta: &common.ChannelMeta{
			IsModelMapped:     true,
			UpstreamModelName: "ep-20261018-asr",
		},
	}

	rewritten := RewriteAudioMultipartModel(body, info)

	// 用原 boundary 应能正常解析重写后的体
	_, params, parseErr := mime.ParseMediaType(contentType)
	if parseErr != nil {
		t.Fatalf("parse content type: %v", parseErr)
	}
	mr := multipart.NewReader(bytes.NewReader(rewritten), params["boundary"])
	var gotModel, gotFile []byte
	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}
		switch part.FormName() {
		case "model":
			gotModel = mustRead(t, part)
		case "file":
			gotFile = mustRead(t, part)
		}
	}
	if string(gotModel) != "ep-20261018-asr" {
		t.Errorf("model = %q, want ep-20261018-asr", gotModel)
	}
	if !bytes.Equal(gotFile, audioPayload) {
		t.Errorf("file part 被改写: %q, want 原样", gotFile)
	}
}

// 未配置映射 / 非 multipart Content-Type / 空 info：原样返回，不做任何改写。
func TestRewriteAudioMultipartModel_Passthrough(t *testing.T) {
	body, contentType := buildAudioMultipart(t, "whisper-1", []byte("audio"))

	unmapped := &common.RelayInfo{
		RequestHeaders: http.Header{"Content-Type": {contentType}},
		ChannelMeta:    &common.ChannelMeta{UpstreamModelName: "ep-x"},
	}
	if got := RewriteAudioMultipartModel(body, unmapped); !bytes.Equal(got, body) {
		t.Error("未映射时应原样返回")
	}

	mappedBadCT := &common.RelayInfo{
		RequestHeaders: http.Header{"Content-Type": {"application/json"}},
		ChannelMeta:    &common.ChannelMeta{IsModelMapped: true, UpstreamModelName: "ep-x"},
	}
	if got := RewriteAudioMultipartModel(body, mappedBadCT); !bytes.Equal(got, body) {
		t.Error("非 multipart Content-Type 应原样返回")
	}

	if got := RewriteAudioMultipartModel(body, nil); !bytes.Equal(got, body) {
		t.Error("nil info 应原样返回")
	}
}

// mustRead 读取 part 全部内容，读失败时终止测试。
func mustRead(t *testing.T, r io.Reader) []byte {
	t.Helper()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read part: %v", err)
	}
	return data
}
