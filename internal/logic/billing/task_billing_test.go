package billing

import (
	"testing"
)

func TestBuildTaskCostBreakdown_NilPricing(t *testing.T) {
	bd := buildTaskCostBreakdown(nil, 0.5, 1000, 500)
	if bd == nil {
		t.Fatal("expected non-nil breakdown")
	}
	assertFloat(t, bd.TotalCost, 0.5, "TotalCost")
	assertFloat(t, bd.BaseCost, 0.5, "BaseCost")
	if bd.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", bd.Currency)
	}
}

func TestBuildTaskCostBreakdown_PerRequestMode(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "per_request",
		PerRequestPrice:  0.10,
		DiscountRatio:    0.9,
		TenantMultiplier: 1.0,
		Currency:         "USD",
	}

	bd := buildTaskCostBreakdown(pricing, 0.10, 0, 0)
	assertFloat(t, bd.BaseCost, 0.10, "BaseCost")
	assertFloat(t, bd.TotalCost, 0.10, "TotalCost")
	assertFloat(t, bd.PerRequestPrice, 0.10, "PerRequestPrice")
	if bd.BillingMode != "per_request" {
		t.Errorf("BillingMode = %q, want per_request", bd.BillingMode)
	}
}

func TestBuildTaskCostBreakdown_TokenMode(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		OutputPrice:      3.0,
		TenantMultiplier: 0.8,
		DiscountRatio:    0.8,
		Currency:         "USD",
	}

	bd := buildTaskCostBreakdown(pricing, 0.024, 10000, 5000)
	if bd.OutputTokens != 10000 {
		t.Errorf("OutputTokens = %d, want 10000", bd.OutputTokens)
	}
	assertFloat(t, bd.OutputCost, 0.024, "OutputCost")
	assertFloat(t, bd.TotalCost, 0.024, "TotalCost")
	// BaseCost = actualCost / tenantMultiplier = 0.024 / 0.8 = 0.03
	assertFloat(t, bd.BaseCost, 0.03, "BaseCost")
}

func TestBuildTaskCostBreakdown_TokenMode_ZeroTenantMultiplier(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		OutputPrice:      3.0,
		TenantMultiplier: 0,
		Currency:         "USD",
	}

	bd := buildTaskCostBreakdown(pricing, 0.01, 1000, 500)
	// tenantMul == 0 → BaseCost = actualCost (no division)
	assertFloat(t, bd.BaseCost, 0.01, "BaseCost")
	assertFloat(t, bd.TotalCost, 0.01, "TotalCost")
}

func TestBuildTaskCostBreakdown_TokenMode_TotalTokens(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		OutputPrice:      5.0,
		TenantMultiplier: 1.0,
		Currency:         "USD",
	}

	bd := buildTaskCostBreakdown(pricing, 0.05, 10000, 3000)
	if bd.OutputTokens != 10000 {
		t.Errorf("OutputTokens = %d, want 10000", bd.OutputTokens)
	}
}

func TestBuildTaskCostBreakdown_CarriesPricingFields(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "per_request",
		PerRequestPrice:  0.50,
		DiscountRatio:    0.85,
		TenantMultiplier: 0.85,
		Currency:         "USD",
	}

	bd := buildTaskCostBreakdown(pricing, 0.50, 0, 0)
	assertFloat(t, bd.DiscountRatio, 0.85, "DiscountRatio")
	assertFloat(t, bd.TenantMultiplier, 0.85, "TenantMultiplier")
	if bd.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", bd.Currency)
	}
}
