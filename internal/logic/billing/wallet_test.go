package billing

import (
	"testing"
)

func TestAvailableBalance_Normal(t *testing.T) {
	w := &WalletInfo{Balance: 100.0, FrozenBalance: 30.0}
	assertFloat(t, AvailableBalance(w), 70.0, "available balance")
}

func TestAvailableBalance_ZeroFrozen(t *testing.T) {
	w := &WalletInfo{Balance: 50.0, FrozenBalance: 0}
	assertFloat(t, AvailableBalance(w), 50.0, "available balance")
}

func TestAvailableBalance_AllFrozen(t *testing.T) {
	w := &WalletInfo{Balance: 100.0, FrozenBalance: 100.0}
	assertFloat(t, AvailableBalance(w), 0.0, "available balance")
}

func TestAvailableBalance_ZeroBalance(t *testing.T) {
	w := &WalletInfo{Balance: 0, FrozenBalance: 0}
	assertFloat(t, AvailableBalance(w), 0.0, "available balance")
}

func TestAvailableBalance_Precision(t *testing.T) {
	w := &WalletInfo{Balance: 100.123456, FrozenBalance: 33.333333}
	expected := 66.790123
	got := AvailableBalance(w)
	if got < expected-0.000001 || got > expected+0.000001 {
		t.Errorf("AvailableBalance = %f, want ~%f", got, expected)
	}
}

func TestWalletInfo_Fields(t *testing.T) {
	w := &WalletInfo{
		ID:               1,
		TenantID:         100,
		Balance:          200.0,
		FrozenBalance:    50.0,
		WarningThreshold: 10.0,
		Currency:         "USD",
	}
	if w.ID != 1 {
		t.Errorf("ID = %d, want 1", w.ID)
	}
	if w.TenantID != 100 {
		t.Errorf("TenantID = %d, want 100", w.TenantID)
	}
	if w.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", w.Currency)
	}
}

func TestPreDeductConstants(t *testing.T) {
	if PreDeductRedisKeyPrefix != "prededuct:" {
		t.Errorf("PreDeductRedisKeyPrefix = %q, want %q", PreDeductRedisKeyPrefix, "prededuct:")
	}
	if PreDeductMaxAge != 1800 {
		t.Errorf("PreDeductMaxAge = %d, want 1800", PreDeductMaxAge)
	}
}

func TestFrozenItem_Fields(t *testing.T) {
	item := FrozenItem{
		RequestID: "req-001",
		ModelName: "gpt-4o",
		Amount:    0.015,
		CreatedAt: 1710000000,
		Remaining: 1200,
	}
	if item.RequestID != "req-001" {
		t.Errorf("RequestID = %q, want req-001", item.RequestID)
	}
	if item.ModelName != "gpt-4o" {
		t.Errorf("ModelName = %q, want gpt-4o", item.ModelName)
	}
	assertFloat(t, item.Amount, 0.015, "Amount")
}

func TestWalletInfo_WarningThresholdZero(t *testing.T) {
	w := &WalletInfo{Balance: 100.0, FrozenBalance: 50.0, WarningThreshold: 0}
	available := AvailableBalance(w)
	assertFloat(t, available, 50.0, "available balance")
}
