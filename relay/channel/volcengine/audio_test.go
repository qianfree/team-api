package volcengine

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
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

// Ark OpenAI 兼容音频端点：speech/transcription 的 URL 拼接；
// translations 无 Ark 支持证据，命中火山渠道须显式报错而非错误转发。
func TestGetRequestURL_Audio(t *testing.T) {
	adaptor := &openaiAdaptor{}

	cases := []struct {
		name    string
		mode    constant.RelayMode
		wantURL string
		wantErr bool
	}{
		{"speech", constant.RelayModeAudioSpeech, "https://ark.cn-beijing.volces.com/api/v3/audio/speech", false},
		{"transcription", constant.RelayModeAudioTranscription, "https://ark.cn-beijing.volces.com/api/v3/audio/transcriptions", false},
		{"translation 不支持", constant.RelayModeAudioTranslation, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info := &common.RelayInfo{
				RelayMode:   int(c.mode),
				ChannelMeta: &common.ChannelMeta{BaseURL: "https://ark.cn-beijing.volces.com"},
			}
			got, err := adaptor.GetRequestURL(info)
			if c.wantErr {
				if err == nil {
					t.Errorf("want error for mode %v, got URL %q", c.mode, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetRequestURL 错误: %v", err)
			}
			if got != c.wantURL {
				t.Errorf("URL = %q, want %q", got, c.wantURL)
			}
		})
	}
}

// STT 的 multipart Content-Type（含 boundary）必须透传到上游；
// speech（JSON 请求体）与其余模式维持 JSON 头。
func TestSetupHeader_AudioMultipart(t *testing.T) {
	_, multipartCT := buildAudioMultipart(t, "doubao-asr", []byte("fake-audio"))

	cases := []struct {
		name       string
		mode       constant.RelayMode
		reqHeaders http.Header
		wantCT     string
	}{
		{
			"转写透传 multipart boundary",
			constant.RelayModeAudioTranscription,
			http.Header{"Content-Type": {multipartCT}},
			multipartCT,
		},
		{
			"转写缺请求头回退 JSON",
			constant.RelayModeAudioTranscription,
			nil,
			"application/json",
		},
		{
			"speech 维持 JSON",
			constant.RelayModeAudioSpeech,
			http.Header{"Content-Type": {multipartCT}},
			"application/json",
		},
		{
			"chat 维持 JSON",
			constant.RelayModeChatCompletions,
			http.Header{"Content-Type": {"application/json"}},
			"application/json",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info := &common.RelayInfo{
				RelayMode:      int(c.mode),
				ChannelMeta:    &common.ChannelMeta{ApiKey: "volc-key"},
				RequestHeaders: c.reqHeaders,
			}
			header := http.Header{}
			setupHeader(header, info)

			if got := header.Get("Content-Type"); got != c.wantCT {
				t.Errorf("Content-Type = %q, want %q", got, c.wantCT)
			}
			if got := header.Get("Authorization"); got != "Bearer volc-key" {
				t.Errorf("Authorization = %q, want Bearer volc-key", got)
			}
		})
	}
}

// multipart 模型改写的核心逻辑（model 字段替换、file part 原样、boundary 不变）
// 已提升为公共 helper.RewriteAudioMultipartModel，见 relay/helper/audio_multipart_test.go；
// 此处经 ConvertRequest 集成路径覆盖（见 TestConvertRequest_Audio）。

// ConvertRequest 音频路径：speech 的模型映射生效且不注入 stream_options；
// transcription 的 multipart 体在模型映射后仍完整可解析。
func TestConvertRequest_Audio(t *testing.T) {
	adaptor := &openaiAdaptor{}

	t.Run("speech 模型映射且不注入 stream_options", func(t *testing.T) {
		info := &common.RelayInfo{
			RelayMode:     int(constant.RelayModeAudioSpeech),
			InboundFormat: constant.RelayFormatOpenAI,
			ChannelMeta: &common.ChannelMeta{
				IsModelMapped:     true,
				UpstreamModelName: "ep-20261018-tts",
			},
		}
		reader, err := adaptor.ConvertRequest(nil, info, []byte(`{"model":"doubao-tts","input":"你好","voice":"zh_male"}`))
		if err != nil {
			t.Fatalf("ConvertRequest 错误: %v", err)
		}
		got, _ := io.ReadAll(reader)
		if !strings.Contains(string(got), `"ep-20261018-tts"`) {
			t.Errorf("模型未映射: %s", got)
		}
		if strings.Contains(string(got), "stream_options") {
			t.Errorf("音频请求不应注入 stream_options: %s", got)
		}
	})

	t.Run("transcription multipart 模型映射", func(t *testing.T) {
		body, contentType := buildAudioMultipart(t, "doubao-asr", []byte("audio-bytes"))
		info := &common.RelayInfo{
			RelayMode:      int(constant.RelayModeAudioTranscription),
			InboundFormat:  constant.RelayFormatOpenAI,
			RequestHeaders: http.Header{"Content-Type": {contentType}},
			ChannelMeta: &common.ChannelMeta{
				IsModelMapped:     true,
				UpstreamModelName: "ep-20261018-asr",
			},
		}
		reader, err := adaptor.ConvertRequest(nil, info, body)
		if err != nil {
			t.Fatalf("ConvertRequest 错误: %v", err)
		}
		got, _ := io.ReadAll(reader)
		if !strings.Contains(string(got), "ep-20261018-asr") {
			t.Errorf("multipart model 字段未映射: %s", got)
		}
		if !strings.Contains(string(got), "audio-bytes") {
			t.Errorf("file part 丢失: %s", got)
		}
	})
}
