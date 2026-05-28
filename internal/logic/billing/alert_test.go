package billing

import (
	"testing"
)

func TestCheckBalanceWarningLogic_BelowThreshold(t *testing.T) {
	// 模拟预警判断逻辑：available <= warningThreshold
	w := &WalletInfo{
		Balance:          100.0,
		FrozenBalance:    99.5,
		WarningThreshold: 1.0,
	}
	available := AvailableBalance(w)
	shouldWarn := w.WarningThreshold > 0 && available <= w.WarningThreshold
	if !shouldWarn {
		t.Errorf("should warn: available=%.6f threshold=%.6f", available, w.WarningThreshold)
	}
}

func TestCheckBalanceWarningLogic_AboveThreshold(t *testing.T) {
	w := &WalletInfo{
		Balance:          100.0,
		FrozenBalance:    50.0,
		WarningThreshold: 1.0,
	}
	available := AvailableBalance(w)
	shouldWarn := w.WarningThreshold > 0 && available <= w.WarningThreshold
	if shouldWarn {
		t.Errorf("should not warn: available=%.6f threshold=%.6f", available, w.WarningThreshold)
	}
}

func TestCheckBalanceWarningLogic_ThresholdZero(t *testing.T) {
	w := &WalletInfo{
		Balance:          0.001,
		FrozenBalance:    0,
		WarningThreshold: 0,
	}
	shouldWarn := w.WarningThreshold > 0 && AvailableBalance(w) <= w.WarningThreshold
	if shouldWarn {
		t.Error("should not warn when threshold is 0")
	}
}

func TestCheckBalanceWarningLogic_ExactlyAtThreshold(t *testing.T) {
	w := &WalletInfo{
		Balance:          5.0,
		FrozenBalance:    4.0,
		WarningThreshold: 1.0,
	}
	available := AvailableBalance(w)
	// available = 1.0, threshold = 1.0, 1.0 <= 1.0 → should warn
	shouldWarn := w.WarningThreshold > 0 && available <= w.WarningThreshold
	if !shouldWarn {
		t.Error("should warn when available exactly equals threshold")
	}
}

func TestCheckBalanceWarningLogic_ZeroBalance(t *testing.T) {
	w := &WalletInfo{
		Balance:          0,
		FrozenBalance:    0,
		WarningThreshold: 5.0,
	}
	available := AvailableBalance(w)
	shouldWarn := w.WarningThreshold > 0 && available <= w.WarningThreshold
	if !shouldWarn {
		t.Error("should warn when balance is 0 and threshold > 0")
	}
}
