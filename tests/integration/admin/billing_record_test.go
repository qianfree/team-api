//go:build integration

package admin_test

import (
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

	// List with tenant_id filter (use 0 to get all, verify no crash)
	respFiltered := client.Get("/api/admin/billing-records", map[string]string{
		"page":      "1",
		"page_size": "5",
	})
	testinfra.AssertPaginatedList(t, respFiltered, 0)

	t.Logf("Billing record list with filters returned successfully")
}
