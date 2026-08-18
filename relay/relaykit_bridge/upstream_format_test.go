package relaykit_bridge

import (
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// TestBridgeUpstreamFormat UseResponsesAPI × clientFormat 的上游格式判定。
func TestBridgeUpstreamFormat(t *testing.T) {
	newInfo := func(useResponses bool) *common.RelayInfo {
		return &common.RelayInfo{
			UseResponsesAPI: useResponses,
			ChannelMeta: &common.ChannelMeta{
				ChannelType: int(constant.ProviderOpenAI),
			},
		}
	}
	cases := []struct {
		useResponses bool
		client       constant.RelayFormat
		want         constant.RelayFormat
	}{
		// P3：claude/gemini 客户端 + UseResponsesAPI → responses 上游
		{true, constant.RelayFormatClaude, constant.RelayFormatResponses},
		{true, constant.RelayFormatGemini, constant.RelayFormatResponses},
		// openai 客户端不受本判定影响（其 ChatViaResponses 响应走 handleChatViaResponses*）
		{true, constant.RelayFormatOpenAI, constant.RelayFormatOpenAI},
		// 未置位：回退 ProviderNativeFormat
		{false, constant.RelayFormatClaude, constant.RelayFormatOpenAI},
		{false, constant.RelayFormatGemini, constant.RelayFormatOpenAI},
	}
	for _, c := range cases {
		if got := bridgeUpstreamFormat(newInfo(c.useResponses), c.client); got != c.want {
			t.Errorf("useResponses=%v client=%s: upstream=%s, want %s", c.useResponses, c.client, got, c.want)
		}
	}
}
