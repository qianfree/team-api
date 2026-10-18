package handler

import (
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// TestApplyMissingUsageFallback 守卫「上游未返回 usage 时按请求侧估算结算」的
// 兜底口径：仅 embeddings/rerank 生效、上游已返回 usage 时不覆盖、估算为零不生效。
func TestApplyMissingUsageFallback(t *testing.T) {
	tests := []struct {
		name        string
		mode        constant.RelayMode
		usage       *common.Usage
		estimated   int
		wantApplied bool
		wantPrompt  int
		wantTotal   int
	}{
		{
			name:        "embeddings 零 usage 回退估算",
			mode:        constant.RelayModeEmbeddings,
			usage:       &common.Usage{},
			estimated:   100,
			wantApplied: true,
			wantPrompt:  100,
			wantTotal:   100,
		},
		{
			name:        "rerank 零 usage 回退估算",
			mode:        constant.RelayModeRerank,
			usage:       &common.Usage{},
			estimated:   60,
			wantApplied: true,
			wantPrompt:  60,
			wantTotal:   60,
		},
		{
			name:        "上游已返回 usage 不覆盖",
			mode:        constant.RelayModeEmbeddings,
			usage:       &common.Usage{PromptTokens: 50, TotalTokens: 50},
			estimated:   100,
			wantApplied: false,
			wantPrompt:  50,
			wantTotal:   50,
		},
		{
			name:        "仅 total 有值也不覆盖",
			mode:        constant.RelayModeRerank,
			usage:       &common.Usage{TotalTokens: 30},
			estimated:   100,
			wantApplied: false,
			wantPrompt:  0,
			wantTotal:   30,
		},
		{
			name:        "chat 模式不启用",
			mode:        constant.RelayModeChatCompletions,
			usage:       &common.Usage{},
			estimated:   100,
			wantApplied: false,
			wantPrompt:  0,
			wantTotal:   0,
		},
		{
			name:        "估算为零不生效",
			mode:        constant.RelayModeEmbeddings,
			usage:       &common.Usage{},
			estimated:   0,
			wantApplied: false,
			wantPrompt:  0,
			wantTotal:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &relayValidation{relayMode: tt.mode, estimatedTokens: tt.estimated}
			applied := applyMissingUsageFallback(tt.usage, v)
			if applied != tt.wantApplied {
				t.Errorf("applied = %v, want %v", applied, tt.wantApplied)
			}
			if tt.usage.PromptTokens != tt.wantPrompt {
				t.Errorf("PromptTokens = %d, want %d", tt.usage.PromptTokens, tt.wantPrompt)
			}
			if tt.usage.TotalTokens != tt.wantTotal {
				t.Errorf("TotalTokens = %d, want %d", tt.usage.TotalTokens, tt.wantTotal)
			}
		})
	}
}
