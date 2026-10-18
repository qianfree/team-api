package handler

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// speech 的 speed 是 TTS 语速参数（0.25~4.0），字段清理须按模式放行；
// chat 的 speed（Claude 推理速度模式）仍默认清理。配置模型映射强制走
// 转换路径（非直连），SanitizeFields 才会实际执行。
func TestConvertRequestBody_SpeedSanitizeByMode(t *testing.T) {
	mappedMeta := func(mode constant.RelayMode) *common.RelayInfo {
		return &common.RelayInfo{
			Context:       context.Background(),
			RelayMode:     int(mode),
			InboundFormat: constant.RelayFormatOpenAI,
			ChannelMeta: &common.ChannelMeta{
				IsModelMapped:     true,
				UpstreamModelName: "upstream-model",
				Settings:          common.ChannelSettings{},
			},
		}
	}
	adaptor := &openai.Adaptor{}

	t.Run("speech 保留 speed", func(t *testing.T) {
		reader, err := convertRequestBody(context.Background(), mappedMeta(constant.RelayModeAudioSpeech),
			[]byte(`{"model":"tts-1","input":"你好","voice":"alloy","speed":1.5}`), adaptor)
		if err != nil {
			t.Fatalf("convertRequestBody 错误: %v", err)
		}
		got, _ := io.ReadAll(reader)
		if !strings.Contains(string(got), `"speed":1.5`) {
			t.Errorf("speech 的 speed 语速参数被误删: %s", got)
		}
	})

	t.Run("chat 清理 speed", func(t *testing.T) {
		reader, err := convertRequestBody(context.Background(), mappedMeta(constant.RelayModeChatCompletions),
			[]byte(`{"model":"gpt-4o","speed":"default","messages":[{"role":"user","content":"hi"}]}`), adaptor)
		if err != nil {
			t.Fatalf("convertRequestBody 错误: %v", err)
		}
		got, _ := io.ReadAll(reader)
		if strings.Contains(string(got), `"speed"`) {
			t.Errorf("chat 的 speed 应被字段清理移除: %s", got)
		}
	})
}
