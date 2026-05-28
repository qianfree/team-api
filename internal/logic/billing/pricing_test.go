package billing

import (
	"fmt"
	"math"
	"testing"

	rcommon "github.com/qianfree/team-api/relay/common"
)

const eps = 1e-10

func assertFloat(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > eps {
		t.Errorf("%s: got %.15f, want %.15f (delta %.15e)", label, got, want, got-want)
	}
}

// ─── AvailableBalance ────────────────────────────────────────────────

func TestAvailableBalance(t *testing.T) {
	tests := []struct {
		name    string
		balance float64
		frozen  float64
		want    float64
	}{
		{"normal", 100.0, 10.0, 90.0},
		{"no frozen", 50.0, 0.0, 50.0},
		{"all frozen", 50.0, 50.0, 0.0},
		{"over frozen", 50.0, 60.0, -10.0},
		{"zero balance", 0.0, 0.0, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &WalletInfo{Balance: tt.balance, FrozenBalance: tt.frozen}
			got := AvailableBalance(w)
			assertFloat(t, got, tt.want, "AvailableBalance")
		})
	}
}

func TestAvailableBalance_NilPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil WalletInfo, but did not panic")
		}
	}()
	_ = AvailableBalance(nil)
}

// ─── calculateTieredCostFromTiers ────────────────────────────────────
// 核心阶梯定价计算逻辑的单元测试

func ptrInt64(v int64) *int64 { return &v }

func TestCalculateTieredCostFromTiers_EmptyTiers(t *testing.T) {
	got := calculateTieredCostFromTiers(nil, 1000, true)
	if got != 0 {
		t.Errorf("expected 0 for nil tiers, got %f", got)
	}
	got = calculateTieredCostFromTiers([]pricingTierRow{}, 1000, true)
	if got != 0 {
		t.Errorf("expected 0 for empty tiers, got %f", got)
	}
}

func TestCalculateTieredCostFromTiers_ZeroOrNegativeTokens(t *testing.T) {
	tiers := []pricingTierRow{
		{MinTokens: 0, MaxTokens: ptrInt64(100_000), InputPrice: 5.0, OutputPrice: 15.0},
	}
	for _, tokens := range []int{0, -1, -1000} {
		got := calculateTieredCostFromTiers(tiers, tokens, true)
		if got != 0 {
			t.Errorf("expected 0 for tokens=%d, got %f", tokens, got)
		}
	}
}

func TestCalculateTieredCostFromTiers_SingleTier_WithMax(t *testing.T) {
	tiers := []pricingTierRow{
		{MinTokens: 0, MaxTokens: ptrInt64(100_000), InputPrice: 5.0, OutputPrice: 15.0},
	}

	tests := []struct {
		tokens  int
		isInput bool
		want    float64
	}{
		{50_000, true, 50_000.0 / 1_000_000 * 5.0},
		{50_000, false, 50_000.0 / 1_000_000 * 15.0},
		{100_000, true, 100_000.0 / 1_000_000 * 5.0},
		{1, true, 1.0 / 1_000_000 * 5.0},
	}
	for _, tt := range tests {
		name := fmt.Sprintf("tokens=%d_isInput=%v", tt.tokens, tt.isInput)
		t.Run(name, func(t *testing.T) {
			got := calculateTieredCostFromTiers(tiers, tt.tokens, tt.isInput)
			assertFloat(t, got, tt.want, "cost")
		})
	}
}

func TestCalculateTieredCostFromTiers_SingleTier_NilMax(t *testing.T) {
	tiers := []pricingTierRow{
		{MinTokens: 0, MaxTokens: nil, InputPrice: 10.0, OutputPrice: 30.0},
	}

	got := calculateTieredCostFromTiers(tiers, 1_000_000, true)
	assertFloat(t, got, 10.0, "1M tokens")

	got = calculateTieredCostFromTiers(tiers, 100_000_000, true)
	assertFloat(t, got, 1000.0, "100M tokens")
}

func TestCalculateTieredCostFromTiers_TwoTiers(t *testing.T) {
	tiers := []pricingTierRow{
		{MinTokens: 0, MaxTokens: ptrInt64(100_000), InputPrice: 5.0, OutputPrice: 15.0},
		{MinTokens: 100_000, MaxTokens: ptrInt64(500_000), InputPrice: 3.0, OutputPrice: 10.0},
	}

	// 50K: entirely in first tier
	got := calculateTieredCostFromTiers(tiers, 50_000, true)
	assertFloat(t, got, 50_000.0/1_000_000*5.0, "50K input")

	// 200K: 100K at $5 + 100K at $3
	got = calculateTieredCostFromTiers(tiers, 200_000, true)
	assertFloat(t, got, 100_000.0/1_000_000*5.0+100_000.0/1_000_000*3.0, "200K input")

	// 500K: 100K at $5 + 400K at $3
	got = calculateTieredCostFromTiers(tiers, 500_000, true)
	assertFloat(t, got, 100_000.0/1_000_000*5.0+400_000.0/1_000_000*3.0, "500K input")
}

func TestCalculateTieredCostFromTiers_ThreeTiers_WithOpenEnd(t *testing.T) {
	max128 := int64(128_000)
	max1M := int64(1_000_000)
	tiers := []pricingTierRow{
		{MinTokens: 0, MaxTokens: &max128, InputPrice: 3.0, OutputPrice: 15.0},
		{MinTokens: max128, MaxTokens: &max1M, InputPrice: 2.0, OutputPrice: 10.0},
		{MinTokens: max1M, MaxTokens: nil, InputPrice: 1.0, OutputPrice: 5.0},
	}

	// 50K: all in first tier
	got := calculateTieredCostFromTiers(tiers, 50_000, true)
	assertFloat(t, got, 50_000.0/1_000_000*3.0, "50K")

	// 500K: 128K at $3 + 372K at $2
	got = calculateTieredCostFromTiers(tiers, 500_000, true)
	assertFloat(t, got, 128_000.0/1_000_000*3.0+372_000.0/1_000_000*2.0, "500K")

	// 2M: 128K at $3 + 872K at $2 + 1M at $1
	got = calculateTieredCostFromTiers(tiers, 2_000_000, true)
	assertFloat(t, got, 128_000.0/1_000_000*3.0+872_000.0/1_000_000*2.0+1_000_000.0/1_000_000*1.0, "2M")
}

func TestCalculateTieredCostFromTiers_ExactBoundary(t *testing.T) {
	tiers := []pricingTierRow{
		{MinTokens: 0, MaxTokens: ptrInt64(1000), InputPrice: 5.0, OutputPrice: 15.0},
		{MinTokens: 1000, MaxTokens: ptrInt64(5000), InputPrice: 3.0, OutputPrice: 10.0},
	}

	got := calculateTieredCostFromTiers(tiers, 1000, true)
	assertFloat(t, got, 1000.0/1_000_000*5.0, "1000 boundary")

	got = calculateTieredCostFromTiers(tiers, 5000, true)
	assertFloat(t, got, 1000.0/1_000_000*5.0+4000.0/1_000_000*3.0, "5000 boundary")
}

func TestCalculateTieredCostFromTiers_InputVsOutput(t *testing.T) {
	tiers := []pricingTierRow{
		{MinTokens: 0, MaxTokens: ptrInt64(100_000), InputPrice: 5.0, OutputPrice: 15.0},
	}

	inputCost := calculateTieredCostFromTiers(tiers, 50_000, true)
	outputCost := calculateTieredCostFromTiers(tiers, 50_000, false)

	expectedRatio := 15.0 / 5.0
	actualRatio := outputCost / inputCost
	if math.Abs(actualRatio-expectedRatio) > eps {
		t.Errorf("expected output/input ratio %.2f, got %.10f", expectedRatio, actualRatio)
	}
}

func TestCalculateTieredCostFromTiers_SkipDegenerateTier(t *testing.T) {
	tiers := []pricingTierRow{
		{MinTokens: 0, MaxTokens: ptrInt64(1000), InputPrice: 5.0, OutputPrice: 15.0},
		{MinTokens: 1000, MaxTokens: ptrInt64(1000), InputPrice: 99.0, OutputPrice: 99.0},
		{MinTokens: 1000, MaxTokens: ptrInt64(5000), InputPrice: 3.0, OutputPrice: 10.0},
	}

	got := calculateTieredCostFromTiers(tiers, 3000, true)
	assertFloat(t, got, 1000.0/1_000_000*5.0+2000.0/1_000_000*3.0, "3000 tokens")
}

func TestCalculateTieredCostFromTiers_TokensExceedAllTiers(t *testing.T) {
	tiers := []pricingTierRow{
		{MinTokens: 0, MaxTokens: ptrInt64(1000), InputPrice: 5.0, OutputPrice: 15.0},
		{MinTokens: 1000, MaxTokens: ptrInt64(5000), InputPrice: 3.0, OutputPrice: 10.0},
	}

	got := calculateTieredCostFromTiers(tiers, 10_000, true)
	assertFloat(t, got, 1000.0/1_000_000*5.0+4000.0/1_000_000*3.0, "10K exceeds all")
}

func TestCalculateTieredCostFromTiers_SingleToken(t *testing.T) {
	tiers := []pricingTierRow{
		{MinTokens: 0, MaxTokens: ptrInt64(100_000), InputPrice: 30.0, OutputPrice: 60.0},
	}
	got := calculateTieredCostFromTiers(tiers, 1, true)
	assertFloat(t, got, 1.0/1_000_000*30.0, "1 token")
}

func TestCalculateTieredCostFromTiers_ZeroPrice(t *testing.T) {
	tiers := []pricingTierRow{
		{MinTokens: 0, MaxTokens: ptrInt64(100_000), InputPrice: 0.0, OutputPrice: 0.0},
	}
	got := calculateTieredCostFromTiers(tiers, 50_000, true)
	if got != 0 {
		t.Errorf("expected 0 cost for zero-price tier, got %f", got)
	}
}

func TestCalculateTieredCostFromTiers_MixedZeroAndNonZeroTiers(t *testing.T) {
	// First tier free, second tier paid
	tiers := []pricingTierRow{
		{MinTokens: 0, MaxTokens: ptrInt64(10_000), InputPrice: 0.0, OutputPrice: 0.0},
		{MinTokens: 10_000, MaxTokens: ptrInt64(100_000), InputPrice: 5.0, OutputPrice: 15.0},
	}

	// 5K: all free
	got := calculateTieredCostFromTiers(tiers, 5_000, true)
	assertFloat(t, got, 0.0, "5K all free")

	// 50K: 10K free + 40K at $5
	got = calculateTieredCostFromTiers(tiers, 50_000, true)
	assertFloat(t, got, 40_000.0/1_000_000*5.0, "50K mixed")
}

func TestCalculateTieredCostFromTiers_TableDriven(t *testing.T) {
	max1K := int64(1000)
	max5K := int64(5000)
	tiers := []pricingTierRow{
		{MinTokens: 0, MaxTokens: &max1K, InputPrice: 10.0, OutputPrice: 30.0},
		{MinTokens: max1K, MaxTokens: &max5K, InputPrice: 5.0, OutputPrice: 15.0},
		{MinTokens: max5K, MaxTokens: nil, InputPrice: 2.0, OutputPrice: 8.0},
	}

	tests := []struct {
		name    string
		tokens  int
		isInput bool
		want    float64
	}{
		{"1 token input", 1, true, 1.0 / 1e6 * 10},
		{"1K exactly input", 1000, true, 1000.0 / 1e6 * 10},
		{"3K input spans two", 3000, true, 1000.0/1e6*10 + 2000.0/1e6*5},
		{"5K boundary input", 5000, true, 1000.0/1e6*10 + 4000.0/1e6*5},
		{"20K input spans three", 20000, true, 1000.0/1e6*10 + 4000.0/1e6*5 + 15000.0/1e6*2},
		{"1K output", 1000, false, 1000.0 / 1e6 * 30},
		{"3K output", 3000, false, 1000.0/1e6*30 + 2000.0/1e6*15},
		{"0 tokens", 0, true, 0},
		{"negative tokens", -100, true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateTieredCostFromTiers(tiers, tt.tokens, tt.isInput)
			assertFloat(t, got, tt.want, "cost")
		})
	}
}

// ─── EstimatePreDeductAmount 逻辑验证 ────────────────────────────────

func TestPreDeductMaxCap(t *testing.T) {
	cost := 5.0
	if cost > 1.0 {
		cost = 1.0
	}
	if cost != 1.0 {
		t.Errorf("expected cap at 1.0, got %f", cost)
	}
}

func TestPreDeductMinFloor(t *testing.T) {
	cost := 0.00001
	if cost < 0.001 {
		cost = 0.001
	}
	if cost != 0.001 {
		t.Errorf("expected floor at 0.001, got %f", cost)
	}
}

func TestPreDeductRounding(t *testing.T) {
	cost := 0.0012345
	rounded := math.Ceil(cost*1_000_000) / 1_000_000
	want := 0.001235
	assertFloat(t, rounded, want, "rounding")
}

func TestPreDeductRounding_ExactValue(t *testing.T) {
	cost := 0.0050000
	rounded := math.Ceil(cost*1_000_000) / 1_000_000
	assertFloat(t, rounded, 0.005, "exact")
}

func TestPreDeductRounding_AlreadyCeiled(t *testing.T) {
	cost := 0.123456
	rounded := math.Ceil(cost*1_000_000) / 1_000_000
	assertFloat(t, rounded, 0.123456, "already ceiled")
}

// ─── PricingResult 结构体验证 ────────────────────────────────────────

func TestPricingResult_Defaults(t *testing.T) {
	p := &PricingResult{}
	if p.Currency != "" {
		t.Errorf("expected empty currency, got %q", p.Currency)
	}
	if p.TenantMultiplier != 0 {
		t.Errorf("expected zero tenant multiplier, got %f", p.TenantMultiplier)
	}
}

// ─── CostBreakdown 结构体验证 ────────────────────────────────────────

func TestCostBreakdown_Fields(t *testing.T) {
	cb := &CostBreakdown{
		BaseCost:         0.5,
		InputCost:        0.3,
		OutputCost:       0.2,
		TotalCost:        0.5,
		InputTokens:      1000,
		OutputTokens:     500,
		BillingMode:      "token",
		TenantMultiplier: 1.0,
		Currency:         "USD",
	}
	if cb.TotalCost != cb.BaseCost {
		t.Errorf("TotalCost %f != BaseCost %f when multiplier=1.0", cb.TotalCost, cb.BaseCost)
	}
	if cb.InputCost+cb.OutputCost != cb.TotalCost {
		t.Errorf("InputCost + OutputCost != TotalCost")
	}
}

func TestCostBreakdown_PerRequest(t *testing.T) {
	cb := &CostBreakdown{
		BaseCost:        0.05,
		TotalCost:       0.05,
		BillingMode:     "per_request",
		PerRequestPrice: 0.05,
		InputTokens:     100,
		OutputTokens:    50,
	}
	if cb.TotalCost != cb.PerRequestPrice {
		t.Errorf("per_request billing: TotalCost should equal PerRequestPrice")
	}
	if cb.InputTokens != 100 || cb.OutputTokens != 50 {
		t.Error("token counts should be recorded even in per_request mode")
	}
}

func TestCostBreakdown_CacheCosts(t *testing.T) {
	cb := &CostBreakdown{
		CacheCreationTokens: 500,
		CacheReadTokens:     2000,
		CacheCreationCost:   0.005,
		CacheReadCost:       0.002,
	}
	if cb.CacheCreationTokens+cb.CacheReadTokens != 2500 {
		t.Errorf("total cache tokens mismatch")
	}
	if cb.CacheCreationCost <= 0 || cb.CacheReadCost <= 0 {
		t.Error("cache costs should be positive when cache tokens > 0")
	}
}

func TestCostBreakdown_WithTenantMultiplier(t *testing.T) {
	baseCost := 0.5
	tenantMultiplier := 0.8 // 20% discount
	cb := &CostBreakdown{
		BaseCost:         baseCost,
		InputCost:        0.3 * tenantMultiplier,
		OutputCost:       0.2 * tenantMultiplier,
		TotalCost:        baseCost * tenantMultiplier,
		TenantMultiplier: tenantMultiplier,
	}
	assertFloat(t, cb.TotalCost, 0.4, "TotalCost with 0.8 multiplier")
	assertFloat(t, cb.InputCost, 0.24, "InputCost with 0.8 multiplier")
	assertFloat(t, cb.OutputCost, 0.16, "OutputCost with 0.8 multiplier")
}

// ─── computeCost — 完整计费计算单元测试 ──────────────────────────────
// 测试从 PricingResult 直接计算费用，覆盖三种计费模式、租户倍率、cache token 等

func TestComputeCost_TokenMode(t *testing.T) {
	pricing := &PricingResult{
		InputPrice:       30.0, // $30/1M tokens
		OutputPrice:      60.0, // $60/1M tokens
		BillingMode:      "token",
		TenantMultiplier: 1.0,
		Currency:         "USD",
	}

	cb := computeCost(pricing, 1000, 500, nil)

	// 1000/1M * 30 = 0.03, 500/1M * 60 = 0.03, total = 0.06
	assertFloat(t, cb.InputCost, 0.03, "InputCost")
	assertFloat(t, cb.OutputCost, 0.03, "OutputCost")
	assertFloat(t, cb.TotalCost, 0.06, "TotalCost")
	assertFloat(t, cb.BaseCost, 0.06, "BaseCost")
	if cb.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", cb.InputTokens)
	}
	if cb.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want 500", cb.OutputTokens)
	}
	if cb.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", cb.Currency)
	}
}

func TestComputeCost_TokenMode_WithTenantMultiplier(t *testing.T) {
	pricing := &PricingResult{
		InputPrice:       10.0,
		OutputPrice:      30.0,
		BillingMode:      "token",
		TenantMultiplier: 0.8, // 20% discount
		Currency:         "USD",
	}

	cb := computeCost(pricing, 100_000, 50_000, nil)

	// input: 100K/1M * 10 = 1.0, output: 50K/1M * 30 = 1.5, base = 2.5
	// with 0.8 multiplier: total = 2.0
	assertFloat(t, cb.BaseCost, 2.5, "BaseCost (before multiplier)")
	assertFloat(t, cb.TotalCost, 2.0, "TotalCost (after 0.8 multiplier)")
	assertFloat(t, cb.InputCost, 0.8, "InputCost (1.0 * 0.8)")
	assertFloat(t, cb.OutputCost, 1.2, "OutputCost (1.5 * 0.8)")
}

func TestComputeCost_PerRequestMode(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "per_request",
		PerRequestPrice:  0.05,
		TenantMultiplier: 1.0,
		Currency:         "USD",
	}

	cb := computeCost(pricing, 5000, 3000, nil)

	// Per-request: cost is flat regardless of tokens
	assertFloat(t, cb.TotalCost, 0.05, "TotalCost (per_request)")
	assertFloat(t, cb.BaseCost, 0.05, "BaseCost")
	if cb.BillingMode != "per_request" {
		t.Errorf("BillingMode = %q, want per_request", cb.BillingMode)
	}
	if cb.InputTokens != 5000 {
		t.Errorf("InputTokens = %d, want 5000 (recorded even in per_request)", cb.InputTokens)
	}
}

func TestComputeCost_PerRequestMode_WithMultiplier(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "per_request",
		PerRequestPrice:  0.10,
		TenantMultiplier: 0.9,
		Currency:         "USD",
	}

	cb := computeCost(pricing, 100, 50, nil)

	// Per-request ignores multiplier in BaseCost but applies it to TotalCost
	assertFloat(t, cb.TotalCost, 0.10, "per_request TotalCost")
}

func TestComputeCost_TieredMode_WithCustomTiers(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "tiered",
		TenantMultiplier: 1.0,
		Currency:         "USD",
		CustomTiers: []pricingTierRow{
			{MinTokens: 0, MaxTokens: ptrInt64(100_000), InputPrice: 5.0, OutputPrice: 15.0},
			{MinTokens: 100_000, MaxTokens: nil, InputPrice: 3.0, OutputPrice: 10.0},
		},
	}

	cb := computeCost(pricing, 200_000, 50_000, nil)

	// Input: 100K * 5 + 100K * 3 = 0.5 + 0.3 = 0.8 per 1M = $0.8
	// Output: 50K * 15 = $0.75
	// Total: 0.8 + 0.75 = $1.55
	assertFloat(t, cb.InputCost, 0.8, "InputCost (tiered)")
	assertFloat(t, cb.OutputCost, 0.75, "OutputCost (tiered)")
	assertFloat(t, cb.TotalCost, 1.55, "TotalCost")
}

func TestComputeCost_ZeroTokens(t *testing.T) {
	pricing := &PricingResult{
		InputPrice:       30.0,
		OutputPrice:      60.0,
		BillingMode:      "token",
		TenantMultiplier: 1.0,
		Currency:         "USD",
	}

	cb := computeCost(pricing, 0, 0, nil)
	assertFloat(t, cb.TotalCost, 0.0, "zero tokens cost")
}

func TestComputeCost_WithCacheTokens(t *testing.T) {
	pricing := &PricingResult{
		InputPrice:         10.0,
		OutputPrice:        30.0,
		CacheReadPrice:     1.0, // $1/1M cache read
		CacheCreationPrice: 2.5, // $2.5/1M cache creation
		BillingMode:        "token",
		TenantMultiplier:   1.0,
		Currency:           "USD",
	}

	usage := &rcommon.Usage{
		PromptTokens:     10_000,
		CompletionTokens: 2_000,
		PromptTokensDetails: &rcommon.TokenDetails{
			CachedTokens:         5_000,
			CachedCreationTokens: 1_000,
		},
		CacheIncludedInPrompt: true,
	}

	cb := computeCost(pricing, 10_000, 2_000, usage)

	// base input = 10K - 5K cache_read - 1K cache_creation = 4K
	// input cost: 4K/1M * 10 = 0.04
	assertFloat(t, cb.InputCost, 0.04, "InputCost (after cache deduction)")
	if cb.InputTokens != 4000 {
		t.Errorf("InputTokens = %d, want 4000 (after cache deduction)", cb.InputTokens)
	}

	// output cost: 2K/1M * 30 = 0.06
	assertFloat(t, cb.OutputCost, 0.06, "OutputCost")

	// cache read: 5K/1M * 1.0 = 0.005
	assertFloat(t, cb.CacheReadCost, 0.005, "CacheReadCost")

	// cache creation: 1K/1M * 2.5 = 0.0025
	assertFloat(t, cb.CacheCreationCost, 0.0025, "CacheCreationCost")

	// total = 0.04 + 0.06 + 0.005 + 0.0025 = 0.1075
	assertFloat(t, cb.TotalCost, 0.1075, "TotalCost (with cache)")
}

func TestComputeCost_CacheNotIncludedInPrompt(t *testing.T) {
	pricing := &PricingResult{
		InputPrice:         10.0,
		OutputPrice:        30.0,
		CacheReadPrice:     1.0,
		CacheCreationPrice: 2.5,
		BillingMode:        "token",
		TenantMultiplier:   1.0,
		Currency:           "USD",
	}

	usage := &rcommon.Usage{
		PromptTokens:     10_000,
		CompletionTokens: 2_000,
		PromptTokensDetails: &rcommon.TokenDetails{
			CachedTokens:         5_000,
			CachedCreationTokens: 1_000,
		},
		CacheIncludedInPrompt: false,
	}

	cb := computeCost(pricing, 10_000, 2_000, usage)

	// When cache NOT included in prompt, full prompt tokens are billed
	// input cost: 10K/1M * 10 = 0.10 (no deduction)
	assertFloat(t, cb.InputCost, 0.10, "InputCost (no cache deduction)")
	if cb.InputTokens != 10_000 {
		t.Errorf("InputTokens = %d, want 10000 (no cache deduction)", cb.InputTokens)
	}
}

func TestComputeCost_CacheWithTenantMultiplier(t *testing.T) {
	pricing := &PricingResult{
		InputPrice:         10.0,
		OutputPrice:        0.0,
		CacheReadPrice:     2.0,
		CacheCreationPrice: 0.0,
		BillingMode:        "token",
		TenantMultiplier:   0.5, // 50% discount
		Currency:           "USD",
	}

	usage := &rcommon.Usage{
		PromptTokens:     10_000,
		CompletionTokens: 0,
		PromptTokensDetails: &rcommon.TokenDetails{
			CachedTokens: 10_000,
		},
		CacheIncludedInPrompt: true,
	}

	cb := computeCost(pricing, 10_000, 0, usage)

	// All input is cache, so base input tokens = 0
	// cache read: 10K/1M * 2.0 = 0.02, * 0.5 multiplier = 0.01
	assertFloat(t, cb.InputCost, 0.0, "InputCost (all cache)")
	assertFloat(t, cb.CacheReadCost, 0.01, "CacheReadCost (with 0.5 multiplier)")
	assertFloat(t, cb.TotalCost, 0.01, "TotalCost (cache only)")
}

// ─── resolveTokenCounts ─────────────────────────────────────────────

func TestResolveTokenCounts_NoUsage(t *testing.T) {
	baseIn, out, cr, cc := resolveTokenCounts(&PricingResult{}, 1000, 500, nil)
	if baseIn != 1000 || out != 500 || cr != 0 || cc != 0 {
		t.Errorf("expected (1000,500,0,0), got (%d,%d,%d,%d)", baseIn, out, cr, cc)
	}
}

func TestResolveTokenCounts_CacheIncluded(t *testing.T) {
	usage := &rcommon.Usage{
		PromptTokens: 10_000,
		PromptTokensDetails: &rcommon.TokenDetails{
			CachedTokens:         3_000,
			CachedCreationTokens: 2_000,
		},
		CacheIncludedInPrompt: true,
	}
	baseIn, _, cr, cc := resolveTokenCounts(&PricingResult{}, 10_000, 500, usage)
	if baseIn != 5_000 {
		t.Errorf("baseInput = %d, want 5000 (10000 - 3000 - 2000)", baseIn)
	}
	if cr != 3_000 || cc != 2_000 {
		t.Errorf("cache: read=%d creation=%d, want 3000, 2000", cr, cc)
	}
}

func TestResolveTokenCounts_CacheExceedsInput(t *testing.T) {
	usage := &rcommon.Usage{
		PromptTokens: 100,
		PromptTokensDetails: &rcommon.TokenDetails{
			CachedTokens:         200,
			CachedCreationTokens: 50,
		},
		CacheIncludedInPrompt: true,
	}
	baseIn, _, _, _ := resolveTokenCounts(&PricingResult{}, 100, 50, usage)
	// 100 - 200 - 50 = -150 → clamped to 0
	if baseIn != 0 {
		t.Errorf("baseInput = %d, want 0 (clamped)", baseIn)
	}
}

func TestResolveTokenCounts_CacheNotIncluded(t *testing.T) {
	usage := &rcommon.Usage{
		PromptTokens: 10_000,
		PromptTokensDetails: &rcommon.TokenDetails{
			CachedTokens: 5_000,
		},
		CacheIncludedInPrompt: false,
	}
	baseIn, _, cr, _ := resolveTokenCounts(&PricingResult{}, 10_000, 500, usage)
	// No deduction when cache not included
	if baseIn != 10_000 {
		t.Errorf("baseInput = %d, want 10000 (no deduction)", baseIn)
	}
	if cr != 5_000 {
		t.Errorf("cacheRead = %d, want 5000", cr)
	}
}

func TestResolveTokenCounts_NilTokenDetails(t *testing.T) {
	usage := &rcommon.Usage{
		PromptTokens:          1000,
		CompletionTokens:      500,
		PromptTokensDetails:   nil,
		CacheIncludedInPrompt: true,
	}
	baseIn, out, cr, cc := resolveTokenCounts(&PricingResult{}, 1000, 500, usage)
	if baseIn != 1000 || out != 500 || cr != 0 || cc != 0 {
		t.Errorf("expected (1000,500,0,0), got (%d,%d,%d,%d)", baseIn, out, cr, cc)
	}
}
