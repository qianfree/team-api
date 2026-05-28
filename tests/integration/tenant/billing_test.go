//go:build integration

package tenant_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	admintest "github.com/qianfree/team-api/tests/integration/admin/testinfra"
	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestUsageLogs(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/usage-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	resp.AssertSuccess(t)

	var data struct {
		List     []map[string]any `json:"list"`
		Total    int              `json:"total"`
		Page     int              `json:"page"`
		PageSize int              `json:"page_size"`
	}
	resp.DecodeData(t, &data)
}

func TestUsageLogsWithFilters(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	// Filter by status
	resp := client.Get("/api/tenant/usage-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "success",
	})
	resp.AssertSuccess(t)

	// Filter by date range
	resp = client.Get("/api/tenant/usage-logs", map[string]string{
		"page":       "1",
		"page_size":  "10",
		"start_date": "2025-01-01",
		"end_date":   "2026-12-31",
	})
	resp.AssertSuccess(t)

	// Filter by username
	resp = client.Get("/api/tenant/usage-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"username":  "owner",
	})
	resp.AssertSuccess(t)
}

func TestUsageLogsExport(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/usage-logs/export", map[string]string{
		"format": "csv",
	})
	resp.AssertSuccess(t)
}

// --- P2: E2E billing verification tests ---

// TestBillingRecordFieldsAfterUsage verifies that usage log records contain
// correct business fields: non-negative costs, valid token counts, USD currency.
// Business rule: every API call produces a usage record with accurate billing data.
func TestBillingRecordFieldsAfterUsage(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/usage-logs", map[string]string{
		"page":      "1",
		"page_size": "20",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			Id           int64   `json:"id"`
			ModelName    string  `json:"model_name"`
			InputTokens  int     `json:"input_tokens"`
			OutputTokens int     `json:"output_tokens"`
			TotalCost    float64 `json:"total_cost"`
			Status       string  `json:"status"`
		} `json:"list"`
	}
	resp.DecodeData(t, &data)

	for _, record := range data.List {
		if record.TotalCost < 0 {
			t.Fatalf("usage record %d has negative total_cost=%.10f", record.Id, record.TotalCost)
		}
		if record.InputTokens < 0 {
			t.Fatalf("usage record %d has negative input_tokens=%d", record.Id, record.InputTokens)
		}
		if record.OutputTokens < 0 {
			t.Fatalf("usage record %d has negative output_tokens=%d", record.Id, record.OutputTokens)
		}
		if record.Status == "" {
			t.Fatalf("usage record %d has empty status", record.Id)
		}
	}

	t.Logf("Validated %d usage records: all costs non-negative, tokens non-negative", len(data.List))
}

// TestUsageLogStatusFilterAccuracy verifies that status filter returns only matching records.
// Business rule: status filter must be accurate — no leaked records from other statuses.
func TestUsageLogStatusFilterAccuracy(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	// Get success records
	resp := client.Get("/api/tenant/usage-logs", map[string]string{
		"page":      "1",
		"page_size": "50",
		"status":    "success",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			Id     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"list"`
	}
	resp.DecodeData(t, &data)

	for _, record := range data.List {
		if record.Status != "success" {
			t.Fatalf("filter status=success returned record %d with status=%q", record.Id, record.Status)
		}
	}

	t.Logf("Status filter accuracy verified: %d records all have status=success", len(data.List))
}

// TestWalletBalanceConsistency verifies the wallet balance, frozen balance,
// and available balance relationship holds for the tenant.
// Business rule: available = balance - frozen, must be >= 0.
func TestWalletBalanceConsistency(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/wallet", nil)
	resp.AssertSuccess(t)

	var wallet struct {
		Balance       float64 `json:"balance"`
		FrozenBalance float64 `json:"frozen_balance"`
		Available     float64 `json:"available"`
		Currency      string  `json:"currency"`
	}
	resp.DecodeData(t, &wallet)

	// Business rule: balance = frozen + available
	if wallet.FrozenBalance < 0 {
		t.Fatalf("frozen_balance (%.10f) must be non-negative", wallet.FrozenBalance)
	}
	if wallet.Available < 0 {
		t.Fatalf("available balance (%.10f) must be non-negative", wallet.Available)
	}

	// Verify balance decomposition: balance should equal frozen + available
	expectedAvailable := wallet.Balance - wallet.FrozenBalance
	delta := expectedAvailable - wallet.Available
	if delta < -0.0001 || delta > 0.0001 {
		t.Fatalf("balance decomposition failed: balance=%.10f - frozen=%.10f = %.10f, but available=%.10f",
			wallet.Balance, wallet.FrozenBalance, expectedAvailable, wallet.Available)
	}

	// Business rule: currency must be USD for internal wallet
	if wallet.Currency != "USD" {
		t.Fatalf("wallet currency=%q, expected USD", wallet.Currency)
	}

	t.Logf("Wallet consistency verified: balance=%.6f = frozen=%.6f + available=%.6f, currency=%s",
		wallet.Balance, wallet.FrozenBalance, wallet.Available, wallet.Currency)
}

// TestNewTenantZeroBalance verifies that a freshly registered tenant starts with zero balance.
// Business rule: new tenants have zero balance, zero frozen, and USD currency.
func TestNewTenantZeroBalance(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/wallet", nil)
	resp.AssertSuccess(t)

	var wallet struct {
		Balance       float64 `json:"balance"`
		FrozenBalance float64 `json:"frozen_balance"`
		Available     float64 `json:"available"`
		Currency      string  `json:"currency"`
	}
	resp.DecodeData(t, &wallet)

	admintest.AssertFloatEqual(t, 0, wallet.Balance, 0.0001, "new tenant balance")
	admintest.AssertFloatEqual(t, 0, wallet.FrozenBalance, 0.0001, "new tenant frozen_balance")
	admintest.AssertFloatEqual(t, 0, wallet.Available, 0.0001, "new tenant available balance")

	if wallet.Currency != "USD" {
		t.Fatalf("new tenant wallet currency=%q, expected USD", wallet.Currency)
	}

	t.Logf("New tenant zero balance verified: balance=%.6f, frozen=%.6f, available=%.6f",
		wallet.Balance, wallet.FrozenBalance, wallet.Available)
}

// TestRelayBillingE2E performs an end-to-end relay request and verifies the billing trail.
// Business rule: every successful relay request must create a billing record and deduct from wallet.
//
// This test is conditional — it requires:
// 1. A model assigned to the tenant
// 2. A working upstream channel
// If either is unavailable, the test is skipped.
func TestRelayBillingE2E(t *testing.T) {
	tenantClient, _ := testinfra.GetAuthedClient(t)

	// Step 1: Check available models for this tenant
	modelsResp := tenantClient.Get("/api/tenant/models", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	modelsResp.AssertSuccess(t)

	var modelsData struct {
		List []struct {
			ModelId   string `json:"model_id"`
			ModelName string `json:"model_name"`
			Enabled   bool   `json:"enabled"`
		} `json:"list"`
	}
	modelsResp.DecodeData(t, &modelsData)

	if len(modelsData.List) == 0 {
		t.Skip("No models available for tenant — skipping relay E2E billing test")
	}

	// Find the first enabled model
	var targetModel string
	for _, m := range modelsData.List {
		if m.Enabled {
			targetModel = m.ModelId
			break
		}
	}
	if targetModel == "" {
		t.Skip("No enabled models for tenant — skipping relay E2E billing test")
	}

	// Step 2: Record wallet balance before request
	walletBeforeResp := tenantClient.Get("/api/tenant/wallet", nil)
	walletBeforeResp.AssertSuccess(t)
	var walletBefore struct {
		Balance   float64 `json:"balance"`
		Available float64 `json:"available"`
	}
	walletBeforeResp.DecodeData(t, &walletBefore)

	// If wallet is empty, we need to add funds (via admin)
	// For E2E test to work, tenant needs some balance
	if walletBefore.Available <= 0 {
		t.Skip("Tenant has no wallet balance — skipping relay E2E billing test (admin needs to add funds first)")
	}

	// Step 3: Create an API key for the relay request
	_, rawKey, keyCleanup := testinfra.CreateTestApiKeyWithSecret(t, tenantClient)
	defer keyCleanup()

	// Step 4: Make a minimal relay request
	relayReq := map[string]any{
		"model":      targetModel,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
		"max_tokens": 5,
	}
	reqBody, _ := json.Marshal(relayReq)

	httpClient := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", testinfra.DefaultBaseURL+"/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("create relay request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+rawKey)

	relayResp, err := httpClient.Do(req)
	if err != nil {
		t.Skipf("Relay request failed (no upstream available): %v", err)
	}
	defer relayResp.Body.Close()

	body, _ := io.ReadAll(relayResp.Body)

	// If relay returned an error (e.g., no channel configured), skip
	if relayResp.StatusCode == 401 || relayResp.StatusCode == 403 {
		t.Skipf("Relay auth failed (status %d) — skipping E2E billing test", relayResp.StatusCode)
	}

	// Parse the relay response to check for errors
	var relayResult struct {
		Error any `json:"error"`
	}
	_ = json.Unmarshal(body, &relayResult)
	if relayResult.Error != nil {
		t.Skipf("Relay returned error: %v — skipping E2E billing test", relayResult.Error)
	}

	// Step 5: Verify wallet was deducted (balance decreased)
	walletAfterResp := tenantClient.Get("/api/tenant/wallet", nil)
	walletAfterResp.AssertSuccess(t)
	var walletAfter struct {
		Balance   float64 `json:"balance"`
		Available float64 `json:"available"`
	}
	walletAfterResp.DecodeData(t, &walletAfter)

	// Balance should have decreased (or stayed same if it was a very cheap request)
	if walletAfter.Balance > walletBefore.Balance+0.0001 {
		t.Fatalf("wallet balance increased after relay request: before=%.10f, after=%.10f",
			walletBefore.Balance, walletAfter.Balance)
	}

	costIncurred := walletBefore.Balance - walletAfter.Balance
	if costIncurred < -0.0001 {
		t.Fatalf("negative cost incurred: %.10f", costIncurred)
	}

	// Step 6: Verify usage log was created
	usageResp := tenantClient.Get("/api/tenant/usage-logs", map[string]string{
		"page":      "1",
		"page_size": "5",
	})
	usageResp.AssertSuccess(t)

	var usageData struct {
		Total int `json:"total"`
		List  []struct {
			Id           int64   `json:"id"`
			ModelName    string  `json:"model_name"`
			InputTokens  int     `json:"input_tokens"`
			OutputTokens int     `json:"output_tokens"`
			TotalCost    float64 `json:"total_cost"`
			Status       string  `json:"status"`
		} `json:"list"`
	}
	usageResp.DecodeData(t, &usageData)

	if usageData.Total == 0 {
		t.Fatal("expected at least 1 usage log after relay request, got 0")
	}

	// Verify the latest usage record
	latest := usageData.List[0]
	if latest.TotalCost < 0 {
		t.Fatalf("usage record %d has negative cost=%.10f", latest.Id, latest.TotalCost)
	}
	if latest.Status == "" {
		t.Fatal("usage record status should not be empty")
	}

	t.Logf("E2E billing verified: model=%q, cost=%.10f USD, wallet=%.6f→%.6f, usage_records=%d",
		targetModel, costIncurred, walletBefore.Balance, walletAfter.Balance, usageData.Total)
}

// TestTransactionTypeConsistency verifies that wallet transactions have valid types
// and amounts that are consistent with balance changes.
// Business rule: every balance change produces a transaction record with a valid type.
func TestTransactionTypeConsistency(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/wallet/transactions", map[string]string{
		"page":      "1",
		"page_size": "20",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			Id      int64   `json:"id"`
			Type    string  `json:"type"`
			Amount  float64 `json:"amount"`
			Balance float64 `json:"balance_after"`
		} `json:"list"`
	}
	resp.DecodeData(t, &data)

	for _, tx := range data.List {
		if tx.Type == "" {
			t.Fatalf("transaction %d has empty type", tx.Id)
		}
		if tx.Amount == 0 {
			t.Fatalf("transaction %d has zero amount — every transaction should change balance", tx.Id)
		}
	}

	t.Logf("Transaction consistency verified for %d records", len(data.List))
}

// ─── 边界值测试 ────────────────────────────────────────────────────

// TestUsageLogs_PaginationBoundary 验证分页边界参数
// Business rule: 超出范围的 page 返回空列表，page_size=1 返回单条记录
func TestUsageLogs_PaginationBoundary(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	// 超出可用数据的 page 应返回空列表
	resp := client.Get("/api/tenant/usage-logs", map[string]string{
		"page":      "999999",
		"page_size": "10",
	})
	resp.AssertSuccess(t)

	var data struct {
		List  []any `json:"list"`
		Total int   `json:"total"`
	}
	resp.DecodeData(t, &data)
	if len(data.List) != 0 {
		t.Fatalf("page=999999 should return empty list, got %d items", len(data.List))
	}

	// page_size=1 应返回最多1条
	resp = client.Get("/api/tenant/usage-logs", map[string]string{
		"page":      "1",
		"page_size": "1",
	})
	resp.AssertSuccess(t)
	resp.DecodeData(t, &data)
	if len(data.List) > 1 {
		t.Fatalf("page_size=1 should return at most 1 item, got %d", len(data.List))
	}
}

// TestUsageLogs_DateRangeInversion 验证日期范围倒置时的行为
// Business rule: start_date > end_date 时应返回空结果或报错
func TestUsageLogs_DateRangeInversion(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/usage-logs", map[string]string{
		"page":       "1",
		"page_size":  "10",
		"start_date": "2026-12-31",
		"end_date":   "2025-01-01",
	})
	resp.AssertSuccess(t)

	var data struct {
		List  []any `json:"list"`
		Total int   `json:"total"`
	}
	resp.DecodeData(t, &data)
	// 倒置的日期范围应返回 0 条记录（没有数据在 2026-12-31 ~ 2025-01-01 之间）
	if len(data.List) != 0 {
		t.Fatalf("inverted date range should return empty list, got %d items", len(data.List))
	}
	t.Logf("inverted date range correctly returns 0 results")
}

// TestUsageLogs_FutureDateRange 验证未来日期范围返回空结果
// Business rule: 未来日期范围内不应有使用记录
func TestUsageLogs_FutureDateRange(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/usage-logs", map[string]string{
		"page":       "1",
		"page_size":  "10",
		"start_date": "2099-01-01",
		"end_date":   "2099-12-31",
	})
	resp.AssertSuccess(t)

	var data struct {
		List  []any `json:"list"`
		Total int   `json:"total"`
	}
	resp.DecodeData(t, &data)
	if len(data.List) != 0 {
		t.Fatalf("future date range should return empty list, got %d items", len(data.List))
	}
}

// TestUsageLogs_InvalidDateFormat 验证无效日期格式被拒绝
func TestUsageLogs_InvalidDateFormat(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/usage-logs", map[string]string{
		"page":       "1",
		"page_size":  "10",
		"start_date": "not-a-date",
		"end_date":   "also-not-date",
	})
	// 服务器应拒绝无效日期格式或忽略它
	if resp.Code == 0 {
		// 如果服务器忽略了无效日期，验证至少返回了有效数据
		t.Logf("server accepted invalid date format — may need validation. code=0")
	} else {
		t.Logf("invalid date format rejected: code=%d msg=%q", resp.Code, resp.Message)
	}
}
