package openai

import (
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
)

// TestAccumulateUsage_CachedTokens Realtime usage 累加必须包含缓存读明细——
// 多轮会话 response.done 事件的 input_token_details.cached_tokens 参与计费扣减，
// 丢弃会导致缓存部分按输入全价计费
func TestAccumulateUsage_CachedTokens(t *testing.T) {
	sum := &dto.RealtimeUsage{}
	accumulateUsage(sum, &dto.RealtimeUsage{
		TotalTokens: 150, InputTokens: 100, OutputTokens: 50,
		InputTokenDetails: &dto.RealtimeTokenDetails{CachedTokens: 60, TextTokens: 40},
	})
	accumulateUsage(sum, &dto.RealtimeUsage{
		TotalTokens: 90, InputTokens: 60, OutputTokens: 30,
		InputTokenDetails: &dto.RealtimeTokenDetails{CachedTokens: 20},
	})

	if sum.InputTokens != 160 || sum.OutputTokens != 80 || sum.TotalTokens != 240 {
		t.Errorf("token sums = %+v, want input=160 output=80 total=240", sum)
	}
	if sum.InputTokenDetails == nil || sum.InputTokenDetails.CachedTokens != 80 {
		t.Errorf("cached tokens = %+v, want 80 (60+20)", sum.InputTokenDetails)
	}

	// 无明细的事件不得清零累计值
	accumulateUsage(sum, &dto.RealtimeUsage{InputTokens: 5, OutputTokens: 5, TotalTokens: 10})
	if sum.InputTokenDetails == nil || sum.InputTokenDetails.CachedTokens != 80 {
		t.Errorf("cached tokens after detail-less event = %+v, want 80 (保持累计)", sum.InputTokenDetails)
	}
}
