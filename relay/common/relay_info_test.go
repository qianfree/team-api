package common

import (
	"testing"

	"github.com/qianfree/team-api/relay/constant"
)

// 图片生成/编辑模式强制 600s 下限：即使渠道或全局配了更短超时也不生效
// （edits 曾被全局 30s 配置在读响应体阶段取消）。
func TestGetTimeoutSeconds_ImageModeFloor(t *testing.T) {
	cases := []struct {
		name      string
		relayMode int
		channel   int // 渠道 settings.timeout_seconds，0=未配置
		want      int
	}{
		{"generations 渠道未配置 → 600s 下限", int(constant.RelayModeImagesGenerations), 0, ImagesGenerationTimeoutSecs},
		{"edits 渠道未配置 → 600s 下限", int(constant.RelayModeImagesEdits), 0, ImagesGenerationTimeoutSecs},
		{"edits 渠道配置 30s → 被下限顶到 600s", int(constant.RelayModeImagesEdits), 30, ImagesGenerationTimeoutSecs},
		{"generations 渠道配置 30s → 被下限顶到 600s", int(constant.RelayModeImagesGenerations), 30, ImagesGenerationTimeoutSecs},
		{"generations 渠道配置 900s > 下限 → 尊重渠道值", int(constant.RelayModeImagesGenerations), 900, 900},
		{"edits 渠道配置 900s > 下限 → 尊重渠道值", int(constant.RelayModeImagesEdits), 900, 900},
		{"chat 渠道配置 30s → 尊重渠道值（非图片模式无下限）", int(constant.RelayModeChatCompletions), 30, 30},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := ChannelSettings{TimeoutSeconds: tc.channel}
			if got := s.GetTimeoutSeconds(tc.relayMode); got != tc.want {
				t.Fatalf("GetTimeoutSeconds(mode=%d, channel=%d) = %d, want %d",
					tc.relayMode, tc.channel, got, tc.want)
			}
		})
	}
}
