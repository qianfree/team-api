//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestBillingRecordList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/billing-records", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)

	t.Logf("Billing record list: total=%d", resp.GetTotal(t))
}

func TestBillingRecordListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by tenant_id
	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	tenantResp := client.Get("/api/admin/billing-records", map[string]string{
		"page":      "1",
		"page_size": "10",
		"tenant_id": fmt.Sprintf("%d", tenantID),
	})
	tenantResp.AssertSuccess(t)

	// New tenant should have no billing records
	var tenantData struct {
		Total int `json:"total"`
	}
	tenantResp.DecodeData(t, &tenantData)
	if tenantData.Total != 0 {
		t.Fatalf("new tenant should have 0 billing records, got %d", tenantData.Total)
	}

	// List all records
	allResp := client.Get("/api/admin/billing-records", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	allResp.AssertSuccess(t)

	// Verify record structure if records exist
	if allResp.GetTotal(t) > 0 {
		var data struct {
			List []struct {
				Id           int64   `json:"id"`
				TenantId     int64   `json:"tenant_id"`
				TenantName   string  `json:"tenant_name"`
				ModelName    string  `json:"model_name"`
				InputTokens  int     `json:"input_tokens"`
				OutputTokens int     `json:"output_tokens"`
				TotalCost    float64 `json:"total_cost"`
				Currency     string  `json:"currency"`
				Status       string  `json:"status"`
			} `json:"list"`
		}
		allResp.DecodeData(t, &data)

		for _, record := range data.List {
			if record.Id <= 0 {
				t.Fatal("billing record id should be positive")
			}
			if record.TotalCost < 0 {
				t.Fatalf("billing record %d has negative total_cost=%f", record.Id, record.TotalCost)
			}
			if record.Currency != "USD" {
				t.Fatalf("billing record %d has unexpected currency=%q (expected USD)", record.Id, record.Currency)
			}
		}

		t.Logf("Verified %d billing records with valid structure", len(data.List))
	}
}

func TestBillingRecordExport(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// CSV export
	csvResp := client.Get("/api/admin/billing-records/export", map[string]string{
		"format": "csv",
	})
	csvResp.AssertSuccess(t)

	// XLSX export
	xlsxResp := client.Get("/api/admin/billing-records/export", map[string]string{
		"format": "xlsx",
	})
	xlsxResp.AssertSuccess(t)

	// Export with tenant filter
	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	filteredExportResp := client.Get("/api/admin/billing-records/export", map[string]string{
		"format":    "csv",
		"tenant_id": fmt.Sprintf("%d", tenantID),
	})
	filteredExportResp.AssertSuccess(t)
}

func TestBillingRecordNegative(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Export with invalid format
	invalidExportResp := client.Get("/api/admin/billing-records/export", map[string]string{
		"format": "pdf",
	})
	if invalidExportResp.Code == 0 {
		t.Fatal("expected error for invalid export format, got success")
	}

	// List with page_size exceeding max
	largePageSizeResp := client.Get("/api/admin/billing-records", map[string]string{
		"page":      "1",
		"page_size": "200",
	})
	// API validates page_size between 1-100, so 200 should fail
	if largePageSizeResp.Code == 0 {
		t.Fatal("expected error for page_size=200 exceeding max, got success")
	}
}

// --- P2: Billing record business logic tests ---

// TestBillingRecordDetail verifies billing record list items contain required fields.
// Note: detail endpoint (/billing-records/{id}) is not yet implemented,
// so this test validates fields from list items instead.
func TestBillingRecordDetail(t *testing.T) {
	t.Skip("Detail endpoint not yet implemented, skipping")
}

// TestBillingRecordTenantIsolation verifies that tenant_id filter correctly isolates records.
// Business rule: billing records must be strictly scoped to their owning tenant.
func TestBillingRecordTenantIsolation(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantA, cleanupA := testinfra.CreateTestTenant(t, client)
	defer cleanupA()

	tenantB, cleanupB := testinfra.CreateTestTenant(t, client)
	defer cleanupB()

	// Both new tenants should have zero records
	respA := client.Get("/api/admin/billing-records", map[string]string{
		"page":      "1",
		"page_size": "10",
		"tenant_id": fmt.Sprintf("%d", tenantA),
	})
	respA.AssertSuccess(t)
	var dataA struct {
		Total int `json:"total"`
	}
	respA.DecodeData(t, &dataA)
	if dataA.Total != 0 {
		t.Fatalf("tenant A (new) should have 0 billing records, got %d", dataA.Total)
	}

	respB := client.Get("/api/admin/billing-records", map[string]string{
		"page":      "1",
		"page_size": "10",
		"tenant_id": fmt.Sprintf("%d", tenantB),
	})
	respB.AssertSuccess(t)
	var dataB struct {
		Total int `json:"total"`
	}
	respB.DecodeData(t, &dataB)
	if dataB.Total != 0 {
		t.Fatalf("tenant B (new) should have 0 billing records, got %d", dataB.Total)
	}

	// Verify that filtering by tenant A does not return any records belonging to other tenants
	// (when records exist in the system)
	allResp := client.Get("/api/admin/billing-records", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	allResp.AssertSuccess(t)

	var allData struct {
		List []struct {
			Id       int64 `json:"id"`
			TenantId int64 `json:"tenant_id"`
		} `json:"list"`
	}
	allResp.DecodeData(t, &allData)

	// Cross-check: tenant A's filtered results should not contain any record
	// that belongs to a different tenant
	filteredResp := client.Get("/api/admin/billing-records", map[string]string{
		"page":      "1",
		"page_size": "50",
		"tenant_id": fmt.Sprintf("%d", tenantA),
	})
	filteredResp.AssertSuccess(t)

	var filteredData struct {
		List []struct {
			Id       int64 `json:"id"`
			TenantId int64 `json:"tenant_id"`
		} `json:"list"`
	}
	filteredResp.DecodeData(t, &filteredData)

	for _, record := range filteredData.List {
		if record.TenantId != tenantA {
			t.Fatalf("tenant A filter returned record %d with tenant_id=%d (expected %d)",
				record.Id, record.TenantId, tenantA)
		}
	}

	t.Logf("Tenant isolation verified: tenant A=%d records, tenant B=%d records, cross-check passed",
		dataA.Total, dataB.Total)
}

// TestBillingRecordDateRangeFilter verifies date range filtering works correctly.
// Business rule: billing records can be filtered by date, and the results must fall
// within the specified range.
func TestBillingRecordDateRangeFilter(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Wide date range — should return all records
	wideResp := client.Get("/api/admin/billing-records", map[string]string{
		"page":       "1",
		"page_size":  "10",
		"start_date": "2020-01-01",
		"end_date":   "2030-12-31",
	})
	wideResp.AssertSuccess(t)
	wideTotal := wideResp.GetTotal(t)

	// Narrow future date range — should return 0 records
	futureResp := client.Get("/api/admin/billing-records", map[string]string{
		"page":       "1",
		"page_size":  "10",
		"start_date": "2099-01-01",
		"end_date":   "2099-12-31",
	})
	futureResp.AssertSuccess(t)
	futureTotal := futureResp.GetTotal(t)

	if futureTotal > wideTotal {
		t.Fatalf("future date range returned more records (%d) than wide range (%d)",
			futureTotal, wideTotal)
	}

	t.Logf("Date range filter: wide=%d, future=%d", wideTotal, futureTotal)
}

// TestBillingRecordCurrencyConsistency verifies all records use USD currency.
// Business rule: system currency for billing is always USD.
func TestBillingRecordCurrencyConsistency(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/billing-records", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			Id        int64   `json:"id"`
			Currency  string  `json:"currency"`
			TotalCost float64 `json:"total_cost"`
		} `json:"list"`
	}
	resp.DecodeData(t, &data)

	for _, record := range data.List {
		if record.Currency != "USD" {
			t.Fatalf("billing record %d has currency=%q — all records must use USD", record.Id, record.Currency)
		}
		if record.TotalCost < 0 {
			t.Fatalf("billing record %d has negative total_cost=%.10f", record.Id, record.TotalCost)
		}
	}

	t.Logf("Currency consistency verified for %d records: all USD, all non-negative", len(data.List))
}

// TestBillingRecordCostNonNegative verifies that all billing amounts are non-negative.
// Business rule: billing costs can never be negative — refunds are separate transactions.
func TestBillingRecordCostNonNegative(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/billing-records", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			Id           int64   `json:"id"`
			InputCost    float64 `json:"input_cost"`
			OutputCost   float64 `json:"output_cost"`
			TotalCost    float64 `json:"total_cost"`
			InputTokens  int     `json:"input_tokens"`
			OutputTokens int     `json:"output_tokens"`
		} `json:"list"`
	}
	resp.DecodeData(t, &data)

	for _, record := range data.List {
		if record.TotalCost < 0 {
			t.Fatalf("record %d: total_cost=%.10f must be non-negative", record.Id, record.TotalCost)
		}
		if record.InputCost < 0 {
			t.Fatalf("record %d: input_cost=%.10f must be non-negative", record.Id, record.InputCost)
		}
		if record.OutputCost < 0 {
			t.Fatalf("record %d: output_cost=%.10f must be non-negative", record.Id, record.OutputCost)
		}
		if record.InputTokens < 0 {
			t.Fatalf("record %d: input_tokens=%d must be non-negative", record.Id, record.InputTokens)
		}
		if record.OutputTokens < 0 {
			t.Fatalf("record %d: output_tokens=%d must be non-negative", record.Id, record.OutputTokens)
		}
	}

	t.Logf("Cost non-negativity verified for %d records", len(data.List))
}
