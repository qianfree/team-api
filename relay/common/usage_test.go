package common

import "testing"

// TestTotalInputTokens 验证「含缓存总输入」归一化口径：
// OpenAI/Gemini 口径（CacheIncludedInPrompt=true，prompt 已含缓存）原样返回；
// Claude 口径（false，input_tokens 与缓存三项并列）补加 cache_read + cache_creation。
func TestTotalInputTokens(t *testing.T) {
	// OpenAI 口径：prompt=2006 已含 cached=1920，总输入即 2006，不得再叠加
	openai := &Usage{
		PromptTokens:          2006,
		CompletionTokens:      46,
		CacheIncludedInPrompt: true,
		PromptTokensDetails:   &TokenDetails{CachedTokens: 1920},
	}
	if got := openai.TotalInputTokens(); got != 2006 {
		t.Errorf("TotalInputTokens() = %d, want 2006 (included 口径不叠加)", got)
	}

	// Claude 口径：input=204 + cache_read=1800 + cache_creation=248 = 2252
	claude := &Usage{
		PromptTokens:     204,
		CompletionTokens: 503,
		PromptTokensDetails: &TokenDetails{
			CachedTokens:         1800,
			CachedCreationTokens: 248,
		},
	}
	if got := claude.TotalInputTokens(); got != 2252 {
		t.Errorf("TotalInputTokens() = %d, want 2252 (excluded 口径补加缓存)", got)
	}

	// Claude 口径但无缓存明细：原样返回
	plain := &Usage{PromptTokens: 100}
	if got := plain.TotalInputTokens(); got != 100 {
		t.Errorf("TotalInputTokens() = %d, want 100", got)
	}

	// nil 安全
	var nilUsage *Usage
	if got := nilUsage.TotalInputTokens(); got != 0 {
		t.Errorf("TotalInputTokens() on nil = %d, want 0", got)
	}
}
