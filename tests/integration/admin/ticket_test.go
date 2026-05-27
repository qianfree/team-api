//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestTicketList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/tickets", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestTicketListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by status
	resp := client.Get("/api/admin/tickets", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "open",
	})
	resp.AssertSuccess(t)

	// Filter by category
	resp = client.Get("/api/admin/tickets", map[string]string{
		"page":      "1",
		"page_size": "10",
		"category":  "billing",
	})
	resp.AssertSuccess(t)

	// Create tenant and filter by tenant_id
	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	resp = client.Get("/api/admin/tickets", map[string]string{
		"page":      "1",
		"page_size": "10",
		"tenant_id": fmt.Sprintf("%d", tenantID),
	})
	resp.AssertSuccess(t)

	t.Logf("Ticket list filters applied successfully")
}
