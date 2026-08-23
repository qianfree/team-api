package common

import (
	"context"
	"testing"

	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// TestBuildConvOptions_DefaultFallback 未注入 provider（单测 / 独立嵌入场景）时
// 使用包内默认值，且函数型钩子（DefaultMaxTokens / SupportsImagine）补齐。
func TestBuildConvOptions_DefaultFallback(t *testing.T) {
	SetConvOptionsProvider(nil)
	info := &RelayInfo{Context: context.Background()}
	opts := info.ConvOptions()

	if !opts.Claude.ThinkingAdapterEnabled {
		t.Error("default Claude thinking adapter should be enabled")
	}
	if opts.Claude.ThinkingAdapterBudgetTokensPercentage != 0.5 {
		t.Errorf("default Claude budget percentage = %v, want 0.5", opts.Claude.ThinkingAdapterBudgetTokensPercentage)
	}
	if opts.Claude.DefaultMaxTokens == nil {
		t.Error("default Claude DefaultMaxTokens hook should be filled")
	}
	if !opts.Gemini.ThinkingAdapterEnabled || !opts.Gemini.FunctionCallThoughtSignatureEnabled {
		t.Error("default Gemini adapters should be enabled")
	}
	if opts.Gemini.SupportsImagine == nil {
		t.Error("default Gemini SupportsImagine hook should be filled")
	}
	if opts.PreserveThinkingSuffix != nil {
		t.Error("default PreserveThinkingSuffix should be nil (never preserve)")
	}
	if opts.OpenRouterDialect {
		t.Error("OpenRouterDialect should be false without channel meta")
	}
}

// TestBuildConvOptions_ProviderInjected 注入 provider 后标量项来自 provider，
// 未填的函数型钩子按包内默认补齐，OpenRouterDialect 按渠道属性覆写。
func TestBuildConvOptions_ProviderInjected(t *testing.T) {
	t.Cleanup(func() { SetConvOptionsProvider(nil) })
	SetConvOptionsProvider(func(ctx context.Context) *convmeta.Options {
		return &convmeta.Options{
			Claude: convmeta.ClaudeOptions{
				ThinkingAdapterEnabled:                false,
				ThinkingAdapterBudgetTokensPercentage: 0.7,
			},
		}
	})

	info := &RelayInfo{Context: context.Background()}
	opts := info.ConvOptions()
	if opts.Claude.ThinkingAdapterEnabled {
		t.Error("provider value should win (adapter disabled)")
	}
	if opts.Claude.ThinkingAdapterBudgetTokensPercentage != 0.7 {
		t.Errorf("provider budget percentage = %v, want 0.7", opts.Claude.ThinkingAdapterBudgetTokensPercentage)
	}
	if opts.Claude.DefaultMaxTokens == nil {
		t.Error("DefaultMaxTokens hook should be backfilled from package default")
	}
	if opts.Gemini.SupportsImagine == nil {
		t.Error("SupportsImagine hook should be backfilled from package default")
	}
}
