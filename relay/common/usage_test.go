package common

import (
	"testing"

	"github.com/qianfree/team-api/relay/dto"
)

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

// TestDtoTokenDetailsToCommon_CacheWriteTTLDualBucket OpenAI TTL 细分归一双桶：
// ttl_30m → CachedCreation5mTokens（短桶）、ttl_12h → CachedCreation1hTokens（长桶），
// 计费侧 resolveTokenCounts 仅读双桶字段即完成跨供应商（Claude/GPT）分档计价。
func TestDtoTokenDetailsToCommon_CacheWriteTTLDualBucket(t *testing.T) {
	d := &dto.TokenDetails{
		CachedTokens:     4000,
		CacheWriteTokens: 7000,
		CacheWriteTokensDetails: &dto.CacheWriteTokensDetail{
			TTL30mTokens: 3000,
			TTL12hTokens: 4000,
		},
	}
	c := DtoTokenDetailsToCommon(d)
	if c.CachedCreation5mTokens != 3000 {
		t.Errorf("CachedCreation5mTokens = %d, want 3000 (ttl_30m 归一短桶)", c.CachedCreation5mTokens)
	}
	if c.CachedCreation1hTokens != 4000 {
		t.Errorf("CachedCreation1hTokens = %d, want 4000 (ttl_12h 归一长桶)", c.CachedCreation1hTokens)
	}
	// 观测字段原样保留（不参与计费）
	if c.CacheWriteTokens != 7000 || c.CacheWriteTokensDetails == nil {
		t.Errorf("CacheWriteTokens = %d details = %v, want 7000 + 非 nil（观测保留）", c.CacheWriteTokens, c.CacheWriteTokensDetails)
	}
}

// TestNormalizeCacheWriteTTL_Defenses 归一防御分支：
// Claude 细分已存在不覆盖（双报防御）、细分全 0 不动、细分和超出总量回退（不变式破坏）。
func TestNormalizeCacheWriteTTL_Defenses(t *testing.T) {
	// Claude 细分优先：OpenAI 细分同时存在（异常双报）不覆盖
	mixed := &TokenDetails{
		CachedCreation5mTokens:  100,
		CachedCreation1hTokens:  200,
		CacheWriteTokensDetails: &CacheWriteTokensDetail{TTL30mTokens: 999, TTL12hTokens: 999},
	}
	mixed.NormalizeCacheWriteTTL()
	if mixed.CachedCreation5mTokens != 100 || mixed.CachedCreation1hTokens != 200 {
		t.Errorf("Claude 细分被覆盖: 5m=%d 1h=%d, want 100/200", mixed.CachedCreation5mTokens, mixed.CachedCreation1hTokens)
	}

	// 细分全 0：按无细分处理，双桶保持 0（计费回退合并桶）
	zero := &TokenDetails{CacheWriteTokens: 5000, CacheWriteTokensDetails: &CacheWriteTokensDetail{}}
	zero.NormalizeCacheWriteTTL()
	if zero.CachedCreation5mTokens != 0 || zero.CachedCreation1hTokens != 0 {
		t.Errorf("全 0 细分不应归一: 5m=%d 1h=%d, want 0/0", zero.CachedCreation5mTokens, zero.CachedCreation1hTokens)
	}

	// 官方不变式 ttl30m+ttl12h=cache_write 被打破：细分不可信，回退（双桶保持 0）
	broken := &TokenDetails{
		CacheWriteTokens:        100,
		CacheWriteTokensDetails: &CacheWriteTokensDetail{TTL30mTokens: 3000, TTL12hTokens: 4000},
	}
	broken.NormalizeCacheWriteTTL()
	if broken.CachedCreation5mTokens != 0 || broken.CachedCreation1hTokens != 0 {
		t.Errorf("不变式破坏应回退: 5m=%d 1h=%d, want 0/0", broken.CachedCreation5mTokens, broken.CachedCreation1hTokens)
	}

	// cache_write_tokens 未报（=0）但细分有效：信任细分（部分兼容上游省略总量键）
	noTotal := &TokenDetails{CacheWriteTokensDetails: &CacheWriteTokensDetail{TTL12hTokens: 4000}}
	noTotal.NormalizeCacheWriteTTL()
	if noTotal.CachedCreation1hTokens != 4000 {
		t.Errorf("无总量时细分应生效: 1h=%d, want 4000", noTotal.CachedCreation1hTokens)
	}

	// nil 细分 / nil 接收者：no-op
	var nilTD *TokenDetails
	nilTD.NormalizeCacheWriteTTL() // 不 panic 即通过
}
