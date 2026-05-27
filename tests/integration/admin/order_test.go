//go:build integration

package admin_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestOrderList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Basic list with default pagination
	resp := client.Get("/api/admin/orders", map[string]string{
		"page":      "1",
		"page_size": "20",
	})
	testinfra.AssertPaginatedList(t, resp, 0)

	total := resp.GetTotal(t)
	t.Logf("Order list: total=%d", total)

	// Filter by status
	respByStatus := client.Get("/api/admin/orders", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "paid",
	})
	testinfra.AssertPaginatedList(t, respByStatus, 0)

	t.Logf("Order list filtered by status=paid: total=%d", respByStatus.GetTotal(t))
}
