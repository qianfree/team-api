//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestWalletList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/wallets", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)

	t.Logf("Wallet list retrieved successfully")
}

func TestWalletInfo(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	resp := client.Get(fmt.Sprintf("/api/admin/wallets/%d", tenantID), nil)
	resp.AssertSuccess(t)

	var data struct {
		Balance          float64  `json:"balance"`
		FrozenBalance    float64  `json:"frozen_balance"`
		WarningThreshold *float64 `json:"warning_threshold"`
	}
	resp.DecodeData(t, &data)

	if data.Balance < 0 {
		t.Fatalf("expected balance >= 0, got %f", data.Balance)
	}
	if data.FrozenBalance < 0 {
		t.Fatalf("expected frozen_balance >= 0, got %f", data.FrozenBalance)
	}

	t.Logf("Wallet info for tenant %d: balance=%f, frozen=%f",
		tenantID, data.Balance, data.FrozenBalance)
}

func TestWalletAdjust(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	// Add balance
	resp := client.Post(fmt.Sprintf("/api/admin/wallets/%d/adjust", tenantID), map[string]any{
		"amount":      10.5,
		"description": "integration test adjustment",
	})
	resp.AssertSuccess(t)

	// Verify balance increased
	infoResp := client.Get(fmt.Sprintf("/api/admin/wallets/%d", tenantID), nil)
	infoResp.AssertSuccess(t)

	var data struct {
		Balance float64 `json:"balance"`
	}
	infoResp.DecodeData(t, &data)

	if data.Balance < 10.0 {
		t.Fatalf("expected balance >= 10.0 after adjustment, got %f", data.Balance)
	}

	t.Logf("Wallet adjust succeeded, new balance: %f", data.Balance)
}

func TestWalletTransactions(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	// First adjust wallet to create a transaction
	adjustResp := client.Post(fmt.Sprintf("/api/admin/wallets/%d/adjust", tenantID), map[string]any{
		"amount":      5.0,
		"description": "test transaction for listing",
	})
	adjustResp.AssertSuccess(t)

	// Then list transactions
	resp := client.Get(fmt.Sprintf("/api/admin/wallets/%d/transactions", tenantID), map[string]string{
		"page":      "1",
		"page_size": "20",
	})
	testinfra.AssertPaginatedList(t, resp, 1)

	total := resp.GetTotal(t)
	t.Logf("Wallet transactions for tenant %d: total=%d", tenantID, total)
}

func TestWalletWarningThreshold(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	// Set warning threshold
	threshold := 1.5
	resp := client.Put(fmt.Sprintf("/api/admin/wallets/%d/warning-threshold", tenantID), map[string]any{
		"threshold": threshold,
	})
	resp.AssertSuccess(t)

	// Verify threshold was set
	infoResp := client.Get(fmt.Sprintf("/api/admin/wallets/%d", tenantID), nil)
	infoResp.AssertSuccess(t)

	var data struct {
		WarningThreshold *float64 `json:"warning_threshold"`
	}
	infoResp.DecodeData(t, &data)

	if data.WarningThreshold == nil {
		t.Fatal("expected warning_threshold to be set, got nil")
	}

	t.Logf("Warning threshold set to %f for tenant %d", *data.WarningThreshold, tenantID)
}
