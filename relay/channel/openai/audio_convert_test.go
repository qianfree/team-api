package openai

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// 音频请求的模型映射：speech（JSON）替换 model 字段；STT/翻译（multipart）在
// 表单字段层改写、文件 part 原样。此前音频三模式直接透传导致映射静默丢失。
func TestConvertRequest_AudioModelMapping(t *testing.T) {
	a := &Adaptor{}

	t.Run("speech JSON 模型映射", func(t *testing.T) {
		info := &common.RelayInfo{
			RelayMode:     int(constant.RelayModeAudioSpeech),
			InboundFormat: constant.RelayFormatOpenAI,
			ChannelMeta: &common.ChannelMeta{
				IsModelMapped:     true,
				UpstreamModelName: "upstream-tts",
			},
		}
		reader, err := a.ConvertRequest(context.Background(), info,
			[]byte(`{"model":"tts-1","input":"你好","voice":"alloy","speed":1.5}`))
		if err != nil {
			t.Fatalf("ConvertRequest 错误: %v", err)
		}
		got, _ := io.ReadAll(reader)
		if !strings.Contains(string(got), `"upstream-tts"`) {
			t.Errorf("speech 模型未映射: %s", got)
		}
		if strings.Contains(string(got), "stream_options") {
			t.Errorf("音频请求不应注入 stream_options: %s", got)
		}
	})

	t.Run("transcription multipart 模型映射", func(t *testing.T) {
		var buf strings.Builder
		w := multipart.NewWriter(&buf)
		_ = w.WriteField("model", "whisper-1")
		fw, _ := w.CreateFormFile("file", "audio.mp3")
		_, _ = fw.Write([]byte("audio-bytes"))
		_ = w.Close()

		info := &common.RelayInfo{
			RelayMode:      int(constant.RelayModeAudioTranscription),
			InboundFormat:  constant.RelayFormatOpenAI,
			RequestHeaders: http.Header{"Content-Type": {w.FormDataContentType()}},
			ChannelMeta: &common.ChannelMeta{
				IsModelMapped:     true,
				UpstreamModelName: "upstream-asr",
			},
		}
		reader, err := a.ConvertRequest(context.Background(), info, []byte(buf.String()))
		if err != nil {
			t.Fatalf("ConvertRequest 错误: %v", err)
		}
		got, _ := io.ReadAll(reader)
		if !strings.Contains(string(got), "upstream-asr") {
			t.Errorf("multipart model 字段未映射: %s", got)
		}
		if !strings.Contains(string(got), "audio-bytes") {
			t.Errorf("file part 丢失: %s", got)
		}
	})
}
