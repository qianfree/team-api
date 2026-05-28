package billing

import (
	"math"
	"testing"
)

func TestDailyReconciliationResult_Fields(t *testing.T) {
	r := &DailyReconciliationResult{
		Date:              "2025-05-27",
		TotalSettled:      100.50,
		TotalWalletDeduct: 100.45,
		Difference:        0.05,
		DifferencePct:     0.05,
		RecordCount:       1500,
	}
	if r.Date != "2025-05-27" {
		t.Errorf("Date = %q, want 2025-05-27", r.Date)
	}
	assertFloat(t, r.TotalSettled, 100.50, "TotalSettled")
	assertFloat(t, r.TotalWalletDeduct, 100.45, "TotalWalletDeduct")
	assertFloat(t, r.Difference, 0.05, "Difference")
	assertFloat(t, r.DifferencePct, 0.05, "DifferencePct")
	if r.RecordCount != 1500 {
		t.Errorf("RecordCount = %d, want 1500", r.RecordCount)
	}
}

func TestDailyReconciliationResult_DifferencePctCalc(t *testing.T) {
	tests := []struct {
		name        string
		settled     float64
		deduct      float64
		wantDiffPct float64
	}{
		{"exact match", 100, 100, 0},
		{"0.05% diff", 100.05, 100, 0.05},
		{"1% diff", 101, 100, 1.0},
		{"negative diff", 99, 100, 1.0}, // abs(-1%)
		{"zero deduct", 100, 0, 0},      // avoid div by zero
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff := tt.settled - tt.deduct
			var diffPct float64
			if tt.deduct > 0 {
				diffPct = math.Abs((diff / tt.deduct) * 100)
			}
			assertFloat(t, diffPct, tt.wantDiffPct, "DifferencePct")
		})
	}
}

func TestDailyReconciliationResult_ThresholdCheck(t *testing.T) {
	// 差异 > 0.1% 时告警
	tests := []struct {
		name      string
		diffPct   float64
		shouldLog bool
	}{
		{"below threshold", 0.05, false},
		{"at threshold", 0.1, false},
		{"above threshold", 0.11, true},
		{"well above threshold", 1.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldLog := tt.diffPct > 0.1
			if shouldLog != tt.shouldLog {
				t.Errorf("diffPct=%.2f: shouldLog=%v, want %v", tt.diffPct, shouldLog, tt.shouldLog)
			}
		})
	}
}

func TestDailyReconciliationResult_Defaults(t *testing.T) {
	r := &DailyReconciliationResult{}
	assertFloat(t, r.TotalSettled, 0, "TotalSettled")
	assertFloat(t, r.TotalWalletDeduct, 0, "TotalWalletDeduct")
	assertFloat(t, r.Difference, 0, "Difference")
	assertFloat(t, r.DifferencePct, 0, "DifferencePct")
	if r.RecordCount != 0 {
		t.Errorf("RecordCount = %d, want 0", r.RecordCount)
	}
}
