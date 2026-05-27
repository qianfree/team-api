//go:build integration

package tenant_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestWalletInfo(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/wallet", nil)
	resp.AssertSuccess(t)

	var data struct {
		Balance            float64 `json:"balance"`
		FrozenBalance      float64 `json:"frozen_balance"`
		AvailableBalance   float64 `json:"available_balance"`
		Currency           string  `json:"currency"`
		Level              int     `json:"level"`
		LevelName          string  `json:"level_name"`
		CumulativeRecharge float64 `json:"cumulative_recharge"`
	}
	resp.DecodeData(t, &data)

	// New tenant should have zero balance
	if data.Currency == "" {
		t.Fatal("expected non-empty currency")
	}
}

func TestWalletTransactions(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/wallet/transactions", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	resp.AssertSuccess(t)

	// New tenant likely has no transactions, just verify structure
	var data struct {
		List     []map[string]any `json:"list"`
		Total    int              `json:"total"`
		Page     int              `json:"page"`
		PageSize int              `json:"page_size"`
	}
	resp.DecodeData(t, &data)
}

func TestWalletFrozenItems(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/wallet/frozen-items", nil)
	resp.AssertSuccess(t)

	var data struct {
		Items []map[string]any `json:"items"`
	}
	resp.DecodeData(t, &data)
}

func TestWalletTransactionsExport(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/wallet/transactions/export", map[string]string{
		"format": "csv",
	})
	resp.AssertSuccess(t)
}
