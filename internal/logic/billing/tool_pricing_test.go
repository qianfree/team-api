package billing

import (
	"context"
	"math"
	"strings"
	"testing"

	rcommon "github.com/qianfree/team-api/relay/common"
)

// buildIndexFromRaw 测试助手：从原始配置串构建索引（折算率 1 = 本位币 USD 口径）
func buildIndexFromRaw(t *testing.T, raw string) *toolPriceIndex {
	t.Helper()
	return buildToolPriceIndex(raw, 1.0)
}

// TestToolPriceIndex_SeedDefaults 种子价兜底：空配置退回内置默认价
func TestToolPriceIndex_SeedDefaults(t *testing.T) {
	idx := buildIndexFromRaw(t, "")
	if got := idx.resolve("web_search", "claude-sonnet-4-5"); got != 0.01 {
		t.Errorf("web_search seed = %v, want 0.01", got)
	}
	if got := idx.resolve("google_search", "gemini-3-pro"); got != 0.014 {
		t.Errorf("google_search seed = %v, want 0.014", got)
	}
	// Gemini ≤2.5 按查询免费（前缀置 0 是终结值）
	if got := idx.resolve("google_search", "gemini-2.5-pro"); got != 0 {
		t.Errorf("google_search gemini-2.5* = %v, want 0", got)
	}
	if got := idx.resolve("google_search_grounded_prompt", "gemini-2.5-flash"); got != 0.035 {
		t.Errorf("grounded_prompt gemini-2.5* = %v, want 0.035", got)
	}
}

// TestToolPriceIndex_AdminOverride 管理端覆盖：同 key 覆盖种子价，置 0 = 显式关闭
func TestToolPriceIndex_AdminOverride(t *testing.T) {
	idx := buildIndexFromRaw(t, `{"web_search": 0.02, "google_search": 0}`)
	if got := idx.resolve("web_search", "any-model"); got != 0.02 {
		t.Errorf("web_search override = %v, want 0.02", got)
	}
	if got := idx.resolve("google_search", "gemini-3-pro"); got != 0 {
		t.Errorf("google_search zeroed = %v, want 0", got)
	}
	// 未覆盖的工具回落种子价
	if got := idx.resolve("file_search", "gpt-4.1"); got != 0.0025 {
		t.Errorf("file_search seed = %v, want 0.0025", got)
	}
}

// TestToolPriceIndex_PrefixLongestMatch 模型前缀最长匹配优先
func TestToolPriceIndex_PrefixLongestMatch(t *testing.T) {
	idx := buildIndexFromRaw(t, `{
		"web_search": 0.01,
		"web_search:claude-*": 0.012,
		"web_search:claude-sonnet*": 0.015
	}`)
	cases := []struct {
		model string
		want  float64
	}{
		{"claude-sonnet-4-5", 0.015}, // 最长前缀 claude-sonnet*
		{"claude-opus-4", 0.012},     // 次长前缀 claude-*
		{"gpt-4o", 0.01},             // 工具默认价
		{"claude-haiku-4", 0.012},    // 前缀无 * 后缀也按纯前缀匹配
	}
	for _, c := range cases {
		if got := idx.resolve("web_search", c.model); got != c.want {
			t.Errorf("resolve(web_search, %s) = %v, want %v", c.model, got, c.want)
		}
	}
	// 空模型名跳过前缀匹配直取默认价
	if got := idx.resolve("web_search", ""); got != 0.01 {
		t.Errorf("resolve(web_search, \"\") = %v, want 0.01", got)
	}
}

// TestToolPriceIndex_UnknownToolFree 未配置的工具 = 免费（非错误）
func TestToolPriceIndex_UnknownToolFree(t *testing.T) {
	idx := buildIndexFromRaw(t, `{}`)
	if got := idx.resolve("nonexistent_tool", "gpt-4o"); got != 0 {
		t.Errorf("unknown tool = %v, want 0 (free)", got)
	}
}

// TestToolPriceIndex_InvalidEntries 读侧容错：坏条目忽略、兄弟条目保留、整份非法退回种子
func TestToolPriceIndex_InvalidEntries(t *testing.T) {
	idx := buildIndexFromRaw(t, `{"web_search": 0.02, "BAD-KEY": 5, "file_search": -1}`)
	if got := idx.resolve("web_search", "m"); got != 0.02 {
		t.Errorf("valid sibling should survive: %v", got)
	}
	if got := idx.resolve("BAD-KEY", "m"); got != 0 {
		t.Errorf("invalid key should be ignored: %v", got)
	}
	if got := idx.resolve("file_search", "m"); got != 0.0025 {
		t.Errorf("negative price entry ignored, seed remains: %v", got)
	}
	// 整份非法 JSON：退回纯种子价
	idx2 := buildIndexFromRaw(t, `{not json`)
	if got := idx2.resolve("web_search", "m"); got != 0.01 {
		t.Errorf("broken json falls back to seed: %v", got)
	}
}

// TestValidateToolCallPricesJSON 保存校验：合法/非法 key、价格区间、非 JSON
func TestValidateToolCallPricesJSON(t *testing.T) {
	valid := []string{
		"",
		"{}",
		`{"web_search": 0.01}`,
		`{"web_search:claude-*": 0}`,
		`{"google_search:gemini-2.5*": 0.035}`,
	}
	for _, v := range valid {
		if err := ValidateToolCallPricesJSON(v); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", v, err)
		}
	}
	invalid := []string{
		"{not json",
		`{"WEB-SEARCH": 1}`,      // 大写非法
		`{"web search": 1}`,      // 空格非法
		`{"web_search": -0.01}`,  // 负价
		`{"web_search": 1000}`,   // 超上限
		`{"web_search": "0.01"}`, // 非数字
	}
	for _, v := range invalid {
		if err := ValidateToolCallPricesJSON(v); err == nil {
			t.Errorf("Validate(%q) = nil, want error", v)
		}
	}
}

// TestApplyToolSurcharge_TokenMode 附加费叠加：token 模式 + 乘数链 + 明细可复算
func TestApplyToolSurcharge_TokenMode(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		InputPrice:       3,
		OutputPrice:      15,
		TenantMultiplier: 0.8,
		TimeMultiplier:   1.5, // 综合倍率 1.2
		Currency:         "USD",
	}
	usage := &rcommon.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		ToolCalls:        map[string]int{"web_search": 3, "free_tool": 2},
	}
	resolve := func(tool string) float64 {
		if tool == "web_search" {
			return 0.01
		}
		return 0 // free_tool 未配价 → 免费
	}
	bd := computeCostWithEval(pricing, usage.PromptTokens, usage.CompletionTokens, usage, 1, nil, resolve)

	// token 费用：(1000/1M×3 + 500/1M×15) × 0.8 × 1.5 = (0.003+0.0075)×1.2 = 0.0126
	tokenCost := 0.0126
	// 工具费独立计算：固定单价 × 次数 = 3 × 0.01 = 0.03（租户 0.8 / 时段 1.5 均不作用）
	wantSurcharge := 0.03
	if bd.ToolSurcharge != wantSurcharge {
		t.Errorf("ToolSurcharge = %v, want %v", bd.ToolSurcharge, wantSurcharge)
	}
	wantTotal := tokenCost + wantSurcharge
	if bd.TotalCost < wantTotal-1e-9 || bd.TotalCost > wantTotal+1e-9 {
		t.Errorf("TotalCost = %v, want %v", bd.TotalCost, wantTotal)
	}
	// 仅计价工具进明细（free_tool 跳过），按名排序
	if len(bd.ToolItems) != 1 || bd.ToolItems[0].Name != "web_search" {
		t.Fatalf("ToolItems = %+v, want single web_search", bd.ToolItems)
	}
	item := bd.ToolItems[0]
	if item.Count != 3 || item.UnitPrice != 0.01 {
		t.Errorf("item = %+v, want {count:3, unit_price:0.01}", item)
	}
	if item.Cost != wantSurcharge {
		t.Errorf("item.Cost = %v, want %v", item.Cost, wantSurcharge)
	}
	// BaseCost 含工具小计（固定价无倍率差异）：0.0105 + 0.03
	wantBase := 0.0105 + 0.03
	if bd.BaseCost < wantBase-1e-9 || bd.BaseCost > wantBase+1e-9 {
		t.Errorf("BaseCost = %v, want %v", bd.BaseCost, wantBase)
	}
}

// TestApplyToolSurcharge_Clamp 次数钳制：超出 MaxBillableToolCallCount 的部分免费
func TestApplyToolSurcharge_Clamp(t *testing.T) {
	pricing := &PricingResult{BillingMode: "token", InputPrice: 3, OutputPrice: 15, TenantMultiplier: 1, TimeMultiplier: 1}
	usage := &rcommon.Usage{ToolCalls: map[string]int{"web_search": MaxBillableToolCallCount + 50000}}
	resolve := func(tool string) float64 { return 0.01 }
	bd := computeCostWithEval(pricing, 0, 0, usage, 1, nil, resolve)
	if bd.ToolItems[0].Count != MaxBillableToolCallCount {
		t.Errorf("clamped count = %d, want %d", bd.ToolItems[0].Count, MaxBillableToolCallCount)
	}
	if bd.ToolSurcharge != 100.0 { // 10000 × 0.01 × 1
		t.Errorf("clamped surcharge = %v, want 100", bd.ToolSurcharge)
	}
}

// TestApplyToolSurcharge_NoCountOrResolver 无工具计数 / 无 resolver：计费结果与旧口径完全一致
func TestApplyToolSurcharge_NoCountOrResolver(t *testing.T) {
	pricing := &PricingResult{BillingMode: "token", InputPrice: 3, OutputPrice: 15, TenantMultiplier: 1, TimeMultiplier: 1}
	usage := &rcommon.Usage{PromptTokens: 1000, CompletionTokens: 500}

	plain := computeCost(pricing, 1000, 500, usage, 1)
	noResolver := computeCostWithEval(pricing, 1000, 500, usage, 1, nil, nil)
	if plain.TotalCost != noResolver.TotalCost || noResolver.ToolSurcharge != 0 || len(noResolver.ToolItems) != 0 {
		t.Errorf("nil resolver must be a no-op: plain=%v got=%+v", plain.TotalCost, noResolver)
	}

	usage.ToolCalls = map[string]int{"web_search": 3}
	withCountsNilResolver := computeCostWithEval(pricing, 1000, 500, usage, 1, nil, nil)
	if withCountsNilResolver.ToolSurcharge != 0 {
		t.Errorf("nil resolver with counts must not bill: %v", withCountsNilResolver.ToolSurcharge)
	}
}

// TestApplyToolSurcharge_PerRequestMode 按次模式同样叠加工具附加费（跨模式通用）
func TestApplyToolSurcharge_PerRequestMode(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "per_request",
		PerRequestPrice:  0.5,
		TenantMultiplier: 0.5,
		TimeMultiplier:   1.5, // 时段乘数作用于模型费但不作用于工具费
	}
	usage := &rcommon.Usage{ToolCalls: map[string]int{"web_search": 2}}
	resolve := func(tool string) float64 { return 0.01 }
	bd := computeCostWithEval(pricing, 0, 0, usage, 1, nil, resolve)
	// 按次模型费 0.5×1×0.5×1.5 = 0.375；工具费固定价：2×0.01 = 0.02（租户/时段均不作用）
	want := 0.395
	if bd.TotalCost < want-1e-9 || bd.TotalCost > want+1e-9 {
		t.Errorf("per_request TotalCost = %v, want %v", bd.TotalCost, want)
	}
	if bd.ToolSurcharge != 0.02 {
		t.Errorf("ToolSurcharge = %v, want 0.02", bd.ToolSurcharge)
	}
}

// TestSnapshot_ToolSurchargesSection 快照与摘要：工具附加费明细写入
// billing_snapshot.tool_surcharges，摘要含可复算行「次数 × 单价 × 倍率 = 费用」
func TestSnapshot_ToolSurchargesSection(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		InputPrice:       3,
		OutputPrice:      15,
		TenantMultiplier: 0.8,
		TimeMultiplier:   1.5,
		Currency:         "USD",
	}
	usage := &rcommon.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		ToolCalls:        map[string]int{"web_search": 3},
	}
	resolve := func(tool string) float64 { return 0.01 }
	breakdown := computeCostWithEval(pricing, 1000, 500, usage, 1, nil, resolve)

	settlement := &SettlementResult{
		PreDeductAmount:   0.0126,
		ActualCost:        breakdown.TotalCost,
		ToolSurchargeCost: breakdown.ToolSurcharge,
	}
	snapshot := GenerateBillingSnapshot(pricing, breakdown, usage, settlement, nil, nil)
	if len(snapshot.ToolSurcharges) != 1 {
		t.Fatalf("ToolSurcharges = %+v, want 1 item", snapshot.ToolSurcharges)
	}
	item := snapshot.ToolSurcharges[0]
	if item.Name != "web_search" || item.Count != 3 || item.UnitPrice != 0.01 || item.Cost != 0.03 {
		t.Errorf("snapshot item = %+v, want {web_search 3 0.01 0.03}（固定价，无倍率）", item)
	}

	// 摘要含工具计费行（倍率显式进入算式）
	summary := GenerateBillingSummary(context.Background(), snapshot)
	if !strings.Contains(summary, "联网搜索(web_search)") || !strings.Contains(summary, "3 次") {
		t.Errorf("summary missing tool surcharge line:\n%s", summary)
	}

	// 无工具计费：快照省略该节（旧快照兼容）
	plain := computeCost(pricing, 1000, 500, &rcommon.Usage{PromptTokens: 1000, CompletionTokens: 500}, 1)
	snapshot2 := GenerateBillingSnapshot(pricing, plain, nil, nil, nil, nil)
	if snapshot2.ToolSurcharges != nil {
		t.Errorf("ToolSurcharges should be omitted without tool billing, got %+v", snapshot2.ToolSurcharges)
	}
}

// TestToolPriceIndex_CNYConversion 本位币=CNY：USD 种子按折算率换算，
// 管理端覆盖值（本位币直配）不二次折算，0 值种子折算后仍为 0（免费语义不变）
func TestToolPriceIndex_CNYConversion(t *testing.T) {
	rate := 7.142857142857143 // 1/0.14，与 billing.defaultUSDToCNY 一致
	idx := buildToolPriceIndex(`{"web_search:claude-*": 0.07}`, rate)

	// 种子 web_search 0.01 USD → ≈0.0714286 本位币
	got := idx.resolve("web_search", "gpt-4o")
	want := 0.01 * rate
	if got < want-1e-9 || got > want+1e-9 {
		t.Errorf("web_search CNY = %v, want %v", got, want)
	}
	// 管理端覆盖值 0.07 原样生效（不折算）
	if got := idx.resolve("web_search", "claude-opus-4"); got != 0.07 {
		t.Errorf("admin override in base currency = %v, want 0.07", got)
	}
	// 0 值种子：折算后仍为 0（gemini ≤2.5 按查询免费的终结语义）
	if got := idx.resolve("google_search", "gemini-2.5-pro"); got != 0 {
		t.Errorf("zero seed stays zero after conversion = %v", got)
	}
	// 未覆盖工具按折算后种子
	got = idx.resolve("file_search", "gpt-4.1")
	want = 0.0025 * rate
	if got < want-1e-9 || got > want+1e-9 {
		t.Errorf("file_search CNY = %v, want %v", got, want)
	}
}

// TestToolPriceIndex_InvalidRate 非法折算率按 1 兜底（防御）
func TestToolPriceIndex_InvalidRate(t *testing.T) {
	for _, bad := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		idx := buildToolPriceIndex("", bad)
		if got := idx.resolve("web_search", "m"); got != 0.01 {
			t.Errorf("rate %v: web_search = %v, want 0.01 (fallback to 1)", bad, got)
		}
	}
}
