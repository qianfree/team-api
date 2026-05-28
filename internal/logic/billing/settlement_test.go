package billing

import (
	"testing"
)

func TestSettlementResult_Fields(t *testing.T) {
	r := &SettlementResult{
		PreDeductAmount:  0.01,
		BaseCost:         0.008,
		ActualCost:       0.008,
		RefundAmount:     0.002,
		SupplementAmount: 0,
		BillingRecordID:  123,
	}
	assertFloat(t, r.PreDeductAmount, 0.01, "PreDeductAmount")
	assertFloat(t, r.ActualCost, 0.008, "ActualCost")
	assertFloat(t, r.RefundAmount, 0.002, "RefundAmount")
	if r.BillingRecordID != 123 {
		t.Errorf("BillingRecordID = %d, want 123", r.BillingRecordID)
	}
}

func TestSettlementResult_RefundCalculation(t *testing.T) {
	tests := []struct {
		name           string
		preDeduct      float64
		actual         float64
		wantRefund     float64
		wantSupplement float64
	}{
		{
			"exact match",
			0.01, 0.01,
			0, 0,
		},
		{
			"refund needed",
			0.01, 0.005,
			0.005, 0,
		},
		{
			"supplement needed",
			0.005, 0.01,
			0, 0.005,
		},
		{
			"both zero",
			0, 0,
			0, 0,
		},
		{
			"preDeduct zero actual positive",
			0, 0.01,
			0, 0.01,
		},
		{
			"large refund",
			1.0, 0.0001,
			0.9999, 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var refundAmt, supplementAmt float64
			if tt.preDeduct > tt.actual {
				refundAmt = tt.preDeduct - tt.actual
			} else if tt.actual > tt.preDeduct {
				supplementAmt = tt.actual - tt.preDeduct
			}

			assertFloat(t, refundAmt, tt.wantRefund, "refund")
			assertFloat(t, supplementAmt, tt.wantSupplement, "supplement")
		})
	}
}

func TestSettlementResult_Defaults(t *testing.T) {
	r := &SettlementResult{}
	assertFloat(t, r.PreDeductAmount, 0, "PreDeductAmount")
	assertFloat(t, r.ActualCost, 0, "ActualCost")
	assertFloat(t, r.RefundAmount, 0, "RefundAmount")
	assertFloat(t, r.SupplementAmount, 0, "SupplementAmount")
	if r.BillingRecordID != 0 {
		t.Errorf("BillingRecordID = %d, want 0", r.BillingRecordID)
	}
	if r.BillingMode != "" {
		t.Errorf("BillingMode = %q, want empty", r.BillingMode)
	}
	if r.BillingSource != "" {
		t.Errorf("BillingSource = %q, want empty", r.BillingSource)
	}
}

func TestSettlementResult_WithCostBreakdown(t *testing.T) {
	r := &SettlementResult{
		PreDeductAmount: 0.01,
		ActualCost:      0.008,
		RefundAmount:    0.002,
		CostBreakdown: &CostBreakdown{
			InputTokens:  1000,
			OutputTokens: 500,
			TotalCost:    0.008,
			Currency:     "USD",
		},
		BillingMode:    "token",
		BillingSource:  "base",
		RateMultiplier: 1.0,
	}

	if r.CostBreakdown == nil {
		t.Fatal("CostBreakdown should not be nil")
	}
	if r.CostBreakdown.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", r.CostBreakdown.InputTokens)
	}
	if r.BillingMode != "token" {
		t.Errorf("BillingMode = %q, want token", r.BillingMode)
	}
	if r.BillingSource != "base" {
		t.Errorf("BillingSource = %q, want base", r.BillingSource)
	}
}
