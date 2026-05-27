//go:build integration

package admin_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestErrorLogList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestErrorLogListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by source
	resp := client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"source":    "relay",
	})
	resp.AssertSuccess(t)

	// Filter by resolved status
	resp = client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"resolved":  "false",
	})
	resp.AssertSuccess(t)

	// Filter by error_code
	resp = client.Get("/api/admin/error-logs", map[string]string{
		"page":       "1",
		"page_size":  "10",
		"error_code": "500",
	})
	resp.AssertSuccess(t)

	// Filter by keyword
	resp = client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"keyword":   "timeout",
	})
	resp.AssertSuccess(t)

	t.Logf("Error log filters applied successfully")
}

func TestErrorLogStats(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/error-logs/stats", nil)
	resp.AssertSuccess(t)

	var data map[string]any
	resp.DecodeData(t, &data)

	t.Logf("Error log stats returned %d keys", len(data))
}
