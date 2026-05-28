package billing

import (
	"encoding/json"
	"strings"
	"testing"

	rcommon "github.com/qianfree/team-api/relay/common"
)

func TestSnapshotToJSON_Nil(t *testing.T) {
	got := SnapshotToJSON(nil)
	if got != "null" {
		t.Errorf("SnapshotToJSON(nil) = %q, want %q", got, "null")
	}
}

func TestSnapshotToJSON_ValidSnapshot(t *testing.T) {
	snapshot := &BillingSnapshot{
		Pricing: BillingSnapshotPricing{
			BaseInputPrice:  0.5,
			BaseOutputPrice: 1.5,
			BillingMode:     "token",
			BillingSource:   "base",
		},
		Multipliers: BillingSnapshotMultipliers{
			ModelMultiplier:  1.0,
			TenantMultiplier: 0.85,
		},
		Settlement: BillingSnapshotSettlement{
			PreDeductAmount: 0.01,
			ActualCost:      0.008,
			RefundAmount:    0.002,
		},
	}
	got := SnapshotToJSON(snapshot)
	if got == "null" {
		t.Fatal("expected valid JSON, got null")
	}
	if !strings.Contains(got, "billing_mode") {
		t.Error("JSON should contain billing_mode field")
	}

	// Verify it's valid JSON
	var parsed BillingSnapshot
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	assertFloat(t, parsed.Pricing.BaseInputPrice, 0.5, "BaseInputPrice")
	assertFloat(t, parsed.Multipliers.TenantMultiplier, 0.85, "TenantMultiplier")
}

func TestGenerateBillingSnapshot_NilPricing(t *testing.T) {
	result := GenerateBillingSnapshot(nil, &CostBreakdown{}, nil, nil, nil)
	if result != nil {
		t.Error("expected nil when pricing is nil")
	}
}

func TestGenerateBillingSnapshot_NilBreakdown(t *testing.T) {
	result := GenerateBillingSnapshot(&PricingResult{}, nil, nil, nil, nil)
	if result != nil {
		t.Error("expected nil when breakdown is nil")
	}
}

func TestGenerateBillingSnapshot_BasicFields(t *testing.T) {
	pricing := &PricingResult{
		InputPrice:         0.5,
		OutputPrice:        1.5,
		BaseInputPrice:     0.5,
		BaseOutputPrice:    1.5,
		BillingMode:        "token",
		BillingSource:      "tenant_custom",
		ModelMultiplier:    1.2,
		TenantMultiplier:   0.85,
		DiscountRatio:      0.85,
		Currency:           "USD",
		CacheReadPrice:     0.1,
		CacheCreationPrice: 0.3,
	}
	breakdown := &CostBreakdown{
		InputTokens:  1000,
		OutputTokens: 500,
		InputCost:    0.0005,
		OutputCost:   0.00075,
		TotalCost:    0.00125,
		Currency:     "USD",
	}
	settlement := &SettlementResult{
		PreDeductAmount: 0.01,
		ActualCost:      0.00125,
		RefundAmount:    0.00875,
	}

	snapshot := GenerateBillingSnapshot(pricing, breakdown, nil, settlement, nil)
	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}

	assertFloat(t, snapshot.Pricing.BaseInputPrice, 0.5, "BaseInputPrice")
	assertFloat(t, snapshot.Pricing.EffectiveOutputPrice, 1.5, "EffectiveOutputPrice")
	if snapshot.Pricing.BillingSource != "tenant_custom" {
		t.Errorf("BillingSource = %q, want tenant_custom", snapshot.Pricing.BillingSource)
	}
	assertFloat(t, snapshot.Multipliers.TenantMultiplier, 0.85, "TenantMultiplier")
	assertFloat(t, snapshot.Settlement.PreDeductAmount, 0.01, "PreDeductAmount")
	assertFloat(t, snapshot.Settlement.RefundAmount, 0.00875, "RefundAmount")
}

func TestGenerateBillingSnapshot_CachePrices(t *testing.T) {
	pricing := &PricingResult{
		InputPrice:         1.0,
		OutputPrice:        2.0,
		BaseInputPrice:     1.0,
		BaseOutputPrice:    2.0,
		BillingMode:        "token",
		CacheReadPrice:     0.1,
		CacheCreationPrice: 0.3,
		TenantMultiplier:   1.0,
		ModelMultiplier:    1.0,
		DiscountRatio:      1.0,
	}

	// With cache tokens → should populate CachePrices
	breakdown := &CostBreakdown{
		InputTokens:         1000,
		OutputTokens:        500,
		InputCost:           0.001,
		OutputCost:          0.001,
		CacheReadTokens:     200,
		CacheCreationTokens: 100,
		CacheReadCost:       0.00002,
		CacheCreationCost:   0.00003,
	}

	snapshot := GenerateBillingSnapshot(pricing, breakdown, nil, nil, nil)
	if snapshot.CachePrices == nil {
		t.Fatal("expected CachePrices to be populated when cache tokens > 0")
	}
	assertFloat(t, snapshot.CachePrices.CacheReadPrice, 0.1, "CacheReadPrice")
	assertFloat(t, snapshot.CachePrices.CacheCreationPrice, 0.3, "CacheCreationPrice")
}

func TestGenerateBillingSnapshot_NoCachePrices(t *testing.T) {
	pricing := &PricingResult{
		InputPrice:       1.0,
		OutputPrice:      2.0,
		BaseInputPrice:   1.0,
		BaseOutputPrice:  2.0,
		BillingMode:      "token",
		TenantMultiplier: 1.0,
		ModelMultiplier:  1.0,
		DiscountRatio:    1.0,
	}
	breakdown := &CostBreakdown{
		InputTokens:  1000,
		OutputTokens: 500,
	}

	snapshot := GenerateBillingSnapshot(pricing, breakdown, nil, nil, nil)
	if snapshot.CachePrices != nil {
		t.Error("expected CachePrices to be nil when no cache tokens")
	}
}

func TestBuildTokenCosts(t *testing.T) {
	pricing := &PricingResult{
		InputPrice:         0.5,
		OutputPrice:        1.5,
		CacheReadPrice:     0.1,
		CacheCreationPrice: 0.3,
	}
	breakdown := &CostBreakdown{
		InputTokens:         2000,
		OutputTokens:        1000,
		InputCost:           0.001,
		OutputCost:          0.0015,
		CacheReadTokens:     500,
		CacheCreationTokens: 300,
		CacheReadCost:       0.00005,
		CacheCreationCost:   0.00009,
	}

	costs := buildTokenCosts(pricing, breakdown)

	if _, ok := costs["input"]; !ok {
		t.Error("expected input token cost")
	}
	if _, ok := costs["output"]; !ok {
		t.Error("expected output token cost")
	}
	if _, ok := costs["cache_read"]; !ok {
		t.Error("expected cache_read token cost")
	}
	if _, ok := costs["cache_creation"]; !ok {
		t.Error("expected cache_creation token cost")
	}

	if costs["input"].Tokens != 2000 {
		t.Errorf("input tokens = %d, want 2000", costs["input"].Tokens)
	}
	assertFloat(t, costs["output"].Cost, 0.0015, "output cost")
	if costs["cache_read"].Tokens != 500 {
		t.Errorf("cache_read tokens = %d, want 500", costs["cache_read"].Tokens)
	}
	if costs["cache_creation"].Tokens != 300 {
		t.Errorf("cache_creation tokens = %d, want 300", costs["cache_creation"].Tokens)
	}
}

func TestBuildTokenCosts_NoCache(t *testing.T) {
	pricing := &PricingResult{
		InputPrice:  0.5,
		OutputPrice: 1.5,
	}
	breakdown := &CostBreakdown{
		InputTokens:  1000,
		OutputTokens: 500,
		InputCost:    0.0005,
		OutputCost:   0.00075,
	}

	costs := buildTokenCosts(pricing, breakdown)

	if _, ok := costs["cache_read"]; ok {
		t.Error("cache_read should not be present when tokens = 0")
	}
	if _, ok := costs["cache_creation"]; ok {
		t.Error("cache_creation should not be present when tokens = 0")
	}
}

func TestGenerateBillingSnapshot_WithRelayInfo(t *testing.T) {
	pricing := &PricingResult{
		InputPrice:       1.0,
		OutputPrice:      2.0,
		BaseInputPrice:   1.0,
		BaseOutputPrice:  2.0,
		BillingMode:      "token",
		TenantMultiplier: 1.0,
		ModelMultiplier:  1.0,
		DiscountRatio:    1.0,
	}
	breakdown := &CostBreakdown{InputTokens: 100, OutputTokens: 50}
	info := &rcommon.RelayInfo{
		OriginModelName: "gpt-4o",
		IsStream:        true,
		ChannelMeta: &rcommon.ChannelMeta{
			UpstreamModelName: "gpt-4o-2024-08-06",
			IsModelMapped:     true,
		},
	}

	snapshot := GenerateBillingSnapshot(pricing, breakdown, nil, nil, info)
	if snapshot.RequestMeta.RequestedModel != "gpt-4o" {
		t.Errorf("RequestedModel = %q, want gpt-4o", snapshot.RequestMeta.RequestedModel)
	}
	if snapshot.RequestMeta.UpstreamModel != "gpt-4o-2024-08-06" {
		t.Errorf("UpstreamModel = %q, want gpt-4o-2024-08-06", snapshot.RequestMeta.UpstreamModel)
	}
	if !snapshot.RequestMeta.IsModelMapped {
		t.Error("IsModelMapped should be true")
	}
	if !snapshot.RequestMeta.IsStream {
		t.Error("IsStream should be true")
	}
}

func TestGenerateBillingSummary_Nil(t *testing.T) {
	got := GenerateBillingSummary(nil)
	if got != "" {
		t.Errorf("GenerateBillingSummary(nil) = %q, want empty", got)
	}
}

func TestGenerateBillingSummary_PerRequest(t *testing.T) {
	snapshot := &BillingSnapshot{
		Pricing: BillingSnapshotPricing{
			BillingMode:         "per_request",
			EffectiveInputPrice: 0.50,
			BillingSource:       "base",
		},
		Multipliers: BillingSnapshotMultipliers{
			TenantMultiplier: 1.0,
		},
		Settlement: BillingSnapshotSettlement{
			PreDeductAmount: 0.50,
			ActualCost:      0.50,
		},
		RequestMeta: BillingSnapshotRequestMeta{
			RequestedModel: "dall-e-3",
		},
	}

	text := GenerateBillingSummary(snapshot)
	if !strings.Contains(text, "按次计费") {
		t.Error("summary should contain 按次计费")
	}
	if !strings.Contains(text, "dall-e-3") {
		t.Error("summary should contain model name")
	}
	if !strings.Contains(text, "0.50") {
		t.Error("summary should contain price")
	}
}

func TestGenerateBillingSummary_TokenMode(t *testing.T) {
	snapshot := &BillingSnapshot{
		Pricing: BillingSnapshotPricing{
			BillingMode:          "token",
			BillingSource:        "tenant_custom",
			EffectiveInputPrice:  0.50,
			EffectiveOutputPrice: 1.50,
		},
		Multipliers: BillingSnapshotMultipliers{
			TenantMultiplier: 0.85,
		},
		TokenCosts: map[string]TokenCostDetail{
			"input":  {Tokens: 1000, UnitPrice: 0.50, Cost: 0.0005},
			"output": {Tokens: 500, UnitPrice: 1.50, Cost: 0.00075},
		},
		Settlement: BillingSnapshotSettlement{
			PreDeductAmount: 0.01,
			ActualCost:      0.0010625,
			RefundAmount:    0.0089375,
		},
		RequestMeta: BillingSnapshotRequestMeta{
			RequestedModel: "gpt-4o",
		},
	}

	text := GenerateBillingSummary(snapshot)
	if !strings.Contains(text, "按量计费") {
		t.Error("summary should contain 按量计费")
	}
	if !strings.Contains(text, "租户独立价") {
		t.Error("summary should contain 租户独立价")
	}
	if !strings.Contains(text, "输入") {
		t.Error("summary should contain 输入")
	}
	if !strings.Contains(text, "输出") {
		t.Error("summary should contain 输出")
	}
	if !strings.Contains(text, "退还") {
		t.Error("summary should contain 退还")
	}
}

func TestGenerateBillingSummary_SupplementAmount(t *testing.T) {
	snapshot := &BillingSnapshot{
		Pricing: BillingSnapshotPricing{
			BillingMode:   "token",
			BillingSource: "base",
		},
		Multipliers: BillingSnapshotMultipliers{
			TenantMultiplier: 1.0,
		},
		TokenCosts: map[string]TokenCostDetail{
			"input":  {Tokens: 1000, UnitPrice: 1.0, Cost: 0.001},
			"output": {Tokens: 500, UnitPrice: 2.0, Cost: 0.001},
		},
		Settlement: BillingSnapshotSettlement{
			PreDeductAmount:  0.001,
			ActualCost:       0.003,
			SupplementAmount: 0.002,
		},
		RequestMeta: BillingSnapshotRequestMeta{
			RequestedModel: "gpt-4o",
		},
	}

	text := GenerateBillingSummary(snapshot)
	if !strings.Contains(text, "补扣") {
		t.Error("summary should contain 补扣")
	}
}

func TestGenerateBillingSummary_NoDiff(t *testing.T) {
	snapshot := &BillingSnapshot{
		Pricing: BillingSnapshotPricing{
			BillingMode:   "token",
			BillingSource: "base",
		},
		Multipliers: BillingSnapshotMultipliers{
			TenantMultiplier: 1.0,
		},
		TokenCosts: map[string]TokenCostDetail{
			"input":  {Tokens: 100, UnitPrice: 1.0, Cost: 0.0001},
			"output": {Tokens: 50, UnitPrice: 2.0, Cost: 0.0001},
		},
		Settlement: BillingSnapshotSettlement{
			PreDeductAmount: 0.0002,
			ActualCost:      0.0002,
		},
		RequestMeta: BillingSnapshotRequestMeta{
			RequestedModel: "gpt-4o",
		},
	}

	text := GenerateBillingSummary(snapshot)
	if !strings.Contains(text, "无差额") {
		t.Error("summary should contain 无差额")
	}
}

func TestGenerateBillingSummary_UnknownSourcePassthrough(t *testing.T) {
	snapshot := &BillingSnapshot{
		Pricing: BillingSnapshotPricing{
			BillingMode:   "unknown_mode",
			BillingSource: "custom_source",
		},
		Multipliers: BillingSnapshotMultipliers{},
		Settlement:  BillingSnapshotSettlement{},
		RequestMeta: BillingSnapshotRequestMeta{},
	}

	text := GenerateBillingSummary(snapshot)
	if !strings.Contains(text, "custom_source") {
		t.Error("unknown source should be passed through as-is")
	}
	if !strings.Contains(text, "unknown_mode") {
		t.Error("unknown mode should be passed through as-is")
	}
}

func TestFormatPrice(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, "0.000000"},
		{0.001, "0.001000"},
		{1.5, "1.500000"},
		{0.123456, "0.123456"},
	}

	for _, tt := range tests {
		got := formatPrice(tt.input)
		if got != tt.expected {
			t.Errorf("formatPrice(%f) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
