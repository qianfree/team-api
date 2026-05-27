//go:build integration

package tenant_test

import (
	"testing"

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
