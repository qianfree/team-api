package moonshot

import (
	"net/http"
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

// TestCacheWriteTTLFromHeader TTL 响应头解析：合法值读取、缺失/非法/负值按 0
func TestCacheWriteTTLFromHeader(t *testing.T) {
	h := http.Header{}
	h.Set(mshHeaderCacheWrite5m, "3000")
	h.Set(mshHeaderCacheWrite1h, "4000")
	if m5, h1 := cacheWriteTTLFromHeader(h); m5 != 3000 || h1 != 4000 {
		t.Errorf("cacheWriteTTLFromHeader = %d/%d, want 3000/4000", m5, h1)
	}

	// 缺失 + 非法 + 负值：全部按 0（Header.Get 大小写不敏感走 canonical 化）
	bad := http.Header{}
	bad.Set(mshHeaderCacheWrite5m, "abc")
	bad.Set(mshHeaderCacheWrite1h, "-5")
	if m5, h1 := cacheWriteTTLFromHeader(bad); m5 != 0 || h1 != 0 {
		t.Errorf("非法头值应按 0: got %d/%d", m5, h1)
	}
	if m5, h1 := cacheWriteTTLFromHeader(http.Header{}); m5 != 0 || h1 != 0 {
		t.Errorf("缺失头应返回 0/0: got %d/%d", m5, h1)
	}
}

// TestApplyCacheWriteTTL 头值填入计费双桶：Kimi 5m/1h 与桶字面同构直接填。
// 计费侧 resolveTokenCounts 读双桶分别按短/长桶价计价，1h 写入不再按短桶基础价少收
func TestApplyCacheWriteTTL(t *testing.T) {
	// 正常：JSON 报了写入总量，头细分与其吻合 → 双桶填入，总量不变
	usage := &common.Usage{
		PromptTokensDetails: &common.TokenDetails{
			CachedTokens:     4000,
			CacheWriteTokens: 7000,
		},
	}
	applyCacheWriteTTL(3000, 4000, usage)
	d := usage.PromptTokensDetails
	if d.CachedCreation5mTokens != 3000 || d.CachedCreation1hTokens != 4000 {
		t.Errorf("双桶 = 5m:%d 1h:%d, want 3000/4000", d.CachedCreation5mTokens, d.CachedCreation1hTokens)
	}
	if d.CacheWriteTokens != 7000 {
		t.Errorf("CacheWriteTokens = %d, want 7000（总量不覆盖）", d.CacheWriteTokens)
	}

	// JSON 未报明细（details 为 nil）：头值补齐双桶 + 写入总量
	fresh := &common.Usage{}
	applyCacheWriteTTL(0, 5000, fresh)
	if fresh.PromptTokensDetails == nil {
		t.Fatal("details 应被创建")
	}
	if fresh.PromptTokensDetails.CachedCreation1hTokens != 5000 || fresh.PromptTokensDetails.CacheWriteTokens != 5000 {
		t.Errorf("无明细场景 = 1h:%d 总量:%d, want 5000/5000",
			fresh.PromptTokensDetails.CachedCreation1hTokens, fresh.PromptTokensDetails.CacheWriteTokens)
	}

	// 防御 1：usage 已带 TTL 细分（上游开始在 JSON 报）→ 头值不覆盖
	jsonFirst := &common.Usage{
		PromptTokensDetails: &common.TokenDetails{
			CachedCreation5mTokens: 100,
			CacheWriteTokens:       100,
		},
	}
	applyCacheWriteTTL(999, 999, jsonFirst)
	if jsonFirst.PromptTokensDetails.CachedCreation5mTokens != 100 {
		t.Errorf("JSON 细分不应被头值覆盖: 5m=%d, want 100", jsonFirst.PromptTokensDetails.CachedCreation5mTokens)
	}

	// 防御 2：头值与 JSON 写入总量矛盾 → 丢弃，回退合并桶口径
	conflict := &common.Usage{
		PromptTokensDetails: &common.TokenDetails{CacheWriteTokens: 100},
	}
	applyCacheWriteTTL(3000, 4000, conflict)
	if conflict.PromptTokensDetails.CachedCreation5mTokens != 0 || conflict.PromptTokensDetails.CachedCreation1hTokens != 0 {
		t.Errorf("矛盾头值应丢弃: 5m=%d 1h=%d, want 0/0",
			conflict.PromptTokensDetails.CachedCreation5mTokens, conflict.PromptTokensDetails.CachedCreation1hTokens)
	}

	// 头值全 0 / nil usage：no-op（不 panic）
	applyCacheWriteTTL(0, 0, &common.Usage{})
	applyCacheWriteTTL(1, 0, nil)
}
