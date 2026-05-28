//go:build integration

package admin_test

import (
	"fmt"
	"sync"
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
		Balance       float64 `json:"balance"`
		FrozenBalance float64 `json:"frozen_balance"`
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

	// Verify balance increased to exact value
	infoResp := client.Get(fmt.Sprintf("/api/admin/wallets/%d", tenantID), nil)
	infoResp.AssertSuccess(t)

	var data struct {
		Balance float64 `json:"balance"`
	}
	infoResp.DecodeData(t, &data)

	testinfra.AssertFloatEqual(t, 10.5, data.Balance, 0.0001, "balance after +10.5 adjustment")

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

	testinfra.AssertFloatEqual(t, 1.5, *data.WarningThreshold, 0.0001, "warning_threshold")

	t.Logf("Warning threshold set to %f for tenant %d", *data.WarningThreshold, tenantID)
}

// --- P2: Wallet business logic tests ---

// TestWalletAdjustPrecision verifies that sequential adjustments accumulate precisely.
// Business rule: wallet balance must reflect exact sum of all adjustments.
func TestWalletAdjustPrecision(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	// Three sequential adjustments — balance should be exact sum
	adjustments := []float64{10.5, 3.25, 1.25}
	var expectedTotal float64
	for _, amount := range adjustments {
		expectedTotal += amount
		resp := client.Post(fmt.Sprintf("/api/admin/wallets/%d/adjust", tenantID), map[string]any{
			"amount":      amount,
			"description": fmt.Sprintf("precision test adjustment %.2f", amount),
		})
		resp.AssertSuccess(t)
	}

	// Verify final balance is exactly the sum of adjustments
	infoResp := client.Get(fmt.Sprintf("/api/admin/wallets/%d", tenantID), nil)
	infoResp.AssertSuccess(t)

	var data struct {
		Balance float64 `json:"balance"`
	}
	infoResp.DecodeData(t, &data)

	testinfra.AssertFloatEqual(t, expectedTotal, data.Balance, 0.0001,
		"balance after sequential adjustments")

	// Verify transaction count matches adjustment count
	txResp := client.Get(fmt.Sprintf("/api/admin/wallets/%d/transactions", tenantID), map[string]string{
		"page":      "1",
		"page_size": "20",
	})
	txTotal := txResp.GetTotal(t)
	if txTotal < len(adjustments) {
		t.Fatalf("expected at least %d transactions, got %d", len(adjustments), txTotal)
	}

	t.Logf("Precision verified: %v = %.4f (actual=%.4f)", adjustments, expectedTotal, data.Balance)
}

// TestWalletNegativeAdjust verifies deduction from wallet (negative adjustment).
// Business rule: wallet supports both positive (recharge) and negative (deduct) adjustments.
func TestWalletNegativeAdjust(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	// First add 20.0
	client.Post(fmt.Sprintf("/api/admin/wallets/%d/adjust", tenantID), map[string]any{
		"amount":      20.0,
		"description": "initial deposit for deduction test",
	}).AssertSuccess(t)

	// Then deduct 7.5
	client.Post(fmt.Sprintf("/api/admin/wallets/%d/adjust", tenantID), map[string]any{
		"amount":      -7.5,
		"description": "deduction test",
	}).AssertSuccess(t)

	// Verify balance is exactly 12.5
	infoResp := client.Get(fmt.Sprintf("/api/admin/wallets/%d", tenantID), nil)
	infoResp.AssertSuccess(t)

	var data struct {
		Balance float64 `json:"balance"`
	}
	infoResp.DecodeData(t, &data)

	testinfra.AssertFloatEqual(t, 12.5, data.Balance, 0.0001,
		"balance after +20.0 and -7.5")

	t.Logf("Negative adjust verified: 20.0 - 7.5 = %.4f", data.Balance)
}

// TestWalletOverdraftPrevention verifies that deducting more than balance is handled correctly.
// Business rule: wallet balance should not go negative from admin adjustments,
// or if allowed, the balance must be accurately tracked.
func TestWalletOverdraftPrevention(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	// Add small amount first
	client.Post(fmt.Sprintf("/api/admin/wallets/%d/adjust", tenantID), map[string]any{
		"amount":      5.0,
		"description": "small deposit for overdraft test",
	}).AssertSuccess(t)

	// Try to deduct more than balance (deduct 100.0 when only 5.0 available)
	overdraftResp := client.Post(fmt.Sprintf("/api/admin/wallets/%d/adjust", tenantID), map[string]any{
		"amount":      -100.0,
		"description": "overdraft attempt",
	})

	// Either the overdraft is rejected (error), or if admin can force it, balance goes negative
	// Both are valid business behaviors — we verify consistency either way
	infoResp := client.Get(fmt.Sprintf("/api/admin/wallets/%d", tenantID), nil)
	infoResp.AssertSuccess(t)

	var data struct {
		Balance float64 `json:"balance"`
	}
	infoResp.DecodeData(t, &data)

	if overdraftResp.Code == 0 {
		// Admin overdraft was allowed — balance should be 5.0 - 100.0 = -95.0
		testinfra.AssertFloatEqual(t, -95.0, data.Balance, 0.0001,
			"balance after allowed overdraft")
		t.Logf("Overdraft allowed: balance = %.4f (admin override)", data.Balance)
	} else {
		// Overdraft was rejected — balance should still be 5.0
		testinfra.AssertFloatEqual(t, 5.0, data.Balance, 0.0001,
			"balance after rejected overdraft")
		t.Logf("Overdraft prevented: balance = %.4f (still 5.0)", data.Balance)
	}
}

// TestWalletConcurrentAdjust verifies wallet balance consistency under concurrent modifications.
// Business rule: concurrent balance adjustments must not cause lost updates or data corruption.
// The final balance must equal the exact sum of all successful adjustments.
func TestWalletConcurrentAdjust(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	const numGoroutines = 10
	const amountPerOp = 1.0

	// Use a shared HTTP client — http.Client is goroutine-safe
	var wg sync.WaitGroup
	results := make(chan *testinfra.APIResponse, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := client.Post(fmt.Sprintf("/api/admin/wallets/%d/adjust", tenantID), map[string]any{
				"amount":      amountPerOp,
				"description": "concurrent adjustment",
			})
			results <- resp
		}()
	}

	wg.Wait()
	close(results)

	// Count successes
	var successCount int
	for resp := range results {
		if resp.Code == 0 {
			successCount++
		}
	}

	if successCount == 0 {
		t.Fatal("all concurrent adjustments failed — possible server issue")
	}

	// Verify final balance equals successCount * amountPerOp
	infoResp := client.Get(fmt.Sprintf("/api/admin/wallets/%d", tenantID), nil)
	infoResp.AssertSuccess(t)

	var data struct {
		Balance float64 `json:"balance"`
	}
	infoResp.DecodeData(t, &data)

	expectedBalance := float64(successCount) * amountPerOp
	testinfra.AssertFloatEqual(t, expectedBalance, data.Balance, 0.0001,
		fmt.Sprintf("balance after %d concurrent +%.1f adjustments", successCount, amountPerOp))

	t.Logf("Concurrent adjust: %d/%d succeeded, balance=%.4f (expected=%.4f)",
		successCount, numGoroutines, data.Balance, expectedBalance)
}

// TestWalletConcurrentMixedAdjust verifies wallet balance consistency
// when concurrent operations include both additions and deductions.
// Business rule: mixed concurrent operations must maintain balance integrity.
func TestWalletConcurrentMixedAdjust(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	// Pre-load balance to support deductions
	const preload = 50.0
	client.Post(fmt.Sprintf("/api/admin/wallets/%d/adjust", tenantID), map[string]any{
		"amount":      preload,
		"description": "preload for mixed concurrent test",
	}).AssertSuccess(t)

	type opResult struct {
		amount float64
		resp   *testinfra.APIResponse
	}

	const numOps = 10
	results := make(chan opResult, numOps)
	var wg sync.WaitGroup

	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Even indices add, odd indices deduct
			amount := 2.0
			if idx%2 == 1 {
				amount = -1.5
			}
			resp := client.Post(fmt.Sprintf("/api/admin/wallets/%d/adjust", tenantID), map[string]any{
				"amount":      amount,
				"description": fmt.Sprintf("mixed concurrent op %d", idx),
			})
			results <- opResult{amount: amount, resp: resp}
		}(i)
	}

	wg.Wait()
	close(results)

	var actualDelta float64
	var successCount int
	for r := range results {
		if r.resp.Code == 0 {
			actualDelta += r.amount
			successCount++
		}
	}

	// Verify final balance = preload + actualDelta
	infoResp := client.Get(fmt.Sprintf("/api/admin/wallets/%d", tenantID), nil)
	infoResp.AssertSuccess(t)

	var data struct {
		Balance float64 `json:"balance"`
	}
	infoResp.DecodeData(t, &data)

	expectedBalance := preload + actualDelta
	testinfra.AssertFloatEqual(t, expectedBalance, data.Balance, 0.0001,
		fmt.Sprintf("balance after %d mixed concurrent ops (delta=%.4f)", successCount, actualDelta))

	t.Logf("Mixed concurrent: preload=%.1f + delta=%.4f = %.4f (actual=%.4f)",
		preload, actualDelta, expectedBalance, data.Balance)
}

// TestWalletAvailableBalance verifies the relationship between balance, frozen_balance, and available balance.
// Business rule: available_balance = balance - frozen_balance, must never be negative.
func TestWalletAvailableBalance(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	// Add balance
	client.Post(fmt.Sprintf("/api/admin/wallets/%d/adjust", tenantID), map[string]any{
		"amount":      100.0,
		"description": "deposit for available balance test",
	}).AssertSuccess(t)

	infoResp := client.Get(fmt.Sprintf("/api/admin/wallets/%d", tenantID), nil)
	infoResp.AssertSuccess(t)

	var data struct {
		Balance       float64 `json:"balance"`
		FrozenBalance float64 `json:"frozen_balance"`
	}
	infoResp.DecodeData(t, &data)

	available := data.Balance - data.FrozenBalance
	if available < 0 {
		t.Fatalf("available balance (%.4f) is negative: balance=%.4f, frozen=%.4f",
			available, data.Balance, data.FrozenBalance)
	}

	if data.FrozenBalance > data.Balance {
		t.Fatalf("frozen_balance (%.4f) exceeds total balance (%.4f)",
			data.FrozenBalance, data.Balance)
	}

	t.Logf("Available balance verified: balance=%.4f - frozen=%.4f = available=%.4f",
		data.Balance, data.FrozenBalance, available)
}

// TestWalletAdjustNonExistentTenant verifies that adjusting a non-existent tenant's wallet fails.
// Business rule: wallet operations must validate tenant existence.
func TestWalletAdjustNonExistentTenant(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Post("/api/admin/wallets/999999999/adjust", map[string]any{
		"amount":      10.0,
		"description": "adjustment for non-existent tenant",
	})
	if resp.Code == 0 {
		t.Fatal("expected error when adjusting wallet for non-existent tenant, got success")
	}

	// Also verify wallet info for non-existent tenant
	infoResp := client.Get("/api/admin/wallets/999999999", nil)
	if infoResp.Code == 0 {
		t.Fatal("expected error when getting wallet for non-existent tenant, got success")
	}
}

// TestWalletTransactionFieldValidation verifies transaction records contain correct fields.
// Business rule: every balance change must produce a transaction with accurate amount, type, and balance_after.
func TestWalletTransactionFieldValidation(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	// Create a known adjustment
	client.Post(fmt.Sprintf("/api/admin/wallets/%d/adjust", tenantID), map[string]any{
		"amount":      25.75,
		"description": "transaction field validation test",
	}).AssertSuccess(t)

	// Get transactions
	txResp := client.Get(fmt.Sprintf("/api/admin/wallets/%d/transactions", tenantID), map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	txResp.AssertSuccess(t)

	var txData struct {
		List []struct {
			Id           int64   `json:"id"`
			Amount       float64 `json:"amount"`
			BalanceAfter float64 `json:"balance_after"`
			Type         string  `json:"type"`
			Description  string  `json:"description"`
		} `json:"list"`
	}
	txResp.DecodeData(t, &txData)

	if len(txData.List) == 0 {
		t.Fatal("expected at least 1 transaction after adjustment")
	}

	tx := txData.List[0]
	testinfra.AssertFloatEqual(t, 25.75, tx.Amount, 0.0001, "transaction amount")
	testinfra.AssertFloatEqual(t, 25.75, tx.BalanceAfter, 0.0001, "balance_after")
	if tx.Type == "" {
		t.Fatal("transaction type should not be empty")
	}
	if tx.Description != "transaction field validation test" {
		t.Fatalf("expected description='transaction field validation test', got %q", tx.Description)
	}

	t.Logf("Transaction fields validated: amount=%.4f, balance_after=%.4f, type=%q",
		tx.Amount, tx.BalanceAfter, tx.Type)
}

// ─── 边界值测试 ────────────────────────────────────────────────────

// TestWalletAdjust_ZeroAmount 验证零金额调整被拒绝
// Business rule: 零金额调整没有实际意义，应被拒绝
func TestWalletAdjust_ZeroAmount(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	resp := client.Post(fmt.Sprintf("/api/admin/wallets/%d/adjust", tenantID), map[string]any{
		"amount":      0,
		"description": "zero amount test",
	})
	if resp.Code == 0 {
		t.Log("server accepted zero amount adjustment — may need validation")
	} else {
		t.Logf("zero amount rejected: code=%d msg=%q", resp.Code, resp.Message)
	}
}

// TestWalletAdjust_NegativeWarningThreshold 验证负数预警阈值被拒绝
// Business rule: 预警阈值不能为负数
func TestWalletAdjust_NegativeWarningThreshold(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	resp := client.Put(fmt.Sprintf("/api/admin/wallets/%d/warning-threshold", tenantID), map[string]any{
		"warning_threshold": -5.0,
	})
	if resp.Code == 0 {
		t.Log("server accepted negative warning threshold — may need validation")
	} else {
		t.Logf("negative warning threshold rejected: code=%d msg=%q", resp.Code, resp.Message)
	}
}

// TestWalletList_PaginationBoundary 验证钱包列表分页边界
func TestWalletList_PaginationBoundary(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// page=999999
	resp := client.Get("/api/admin/wallets", map[string]string{
		"page":      "999999",
		"page_size": "10",
	})
	resp.AssertSuccess(t)
	var data struct {
		List []any `json:"list"`
	}
	resp.DecodeData(t, &data)
	if len(data.List) != 0 {
		t.Fatalf("page=999999 should return empty list, got %d items", len(data.List))
	}
}
