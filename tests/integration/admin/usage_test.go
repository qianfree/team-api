//go:build integration

package admin_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestUsageLogList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Basic list with default pagination
	resp := client.Get("/api/admin/usage-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)

	t.Logf("Usage log list: total=%d", resp.GetTotal(t))

	// List with date filters
	respFiltered := client.Get("/api/admin/usage-logs", map[string]string{
		"page":       "1",
		"page_size":  "5",
		"start_date": "2025-01-01",
		"end_date":   "2030-12-31",
	})
	testinfra.AssertPaginatedList(t, respFiltered, 0)

	// List with status filter
	respByStatus := client.Get("/api/admin/usage-logs", map[string]string{
		"page":      "1",
		"page_size": "5",
		"status":    "success",
	})
	testinfra.AssertPaginatedList(t, respByStatus, 0)

	t.Logf("Usage log list with filters returned successfully")
}
