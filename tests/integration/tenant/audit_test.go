//go:build integration

package tenant_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestAuditConfigGet(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/audit/config", nil)
	resp.AssertSuccess(t)

	var data struct {
		AuditLevel string `json:"audit_level"`
	}
	resp.DecodeData(t, &data)
}

func TestAuditConfigUpdate(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	// Get current level
	getResp := client.Get("/api/tenant/audit/config", nil)
	getResp.AssertSuccess(t)

	var getData struct {
		AuditLevel string `json:"audit_level"`
	}
	getResp.DecodeData(t, &getData)

	// Update to a different level
	newLevel := "full_text"
	if getData.AuditLevel == "full_text" {
		newLevel = "masked"
	}

	updateResp := client.Put("/api/tenant/audit/config", map[string]any{
		"audit_level": newLevel,
	})
	updateResp.AssertSuccess(t)

	// Restore original level
	client.Put("/api/tenant/audit/config", map[string]any{
		"audit_level": getData.AuditLevel,
	})
}

func TestAuditLogs(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/audit/logs", map[string]string{
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

func TestRequestAuditLogs(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/audit/request-logs", map[string]string{
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

func TestRequestAuditLogsWithFilters(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	// Filter by username
	resp := client.Get("/api/tenant/audit/request-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"username":  "owner",
	})
	resp.AssertSuccess(t)

	// Filter by date range
	resp = client.Get("/api/tenant/audit/request-logs", map[string]string{
		"page":       "1",
		"page_size":  "10",
		"start_date": "2025-01-01",
		"end_date":   "2026-12-31",
	})
	resp.AssertSuccess(t)
}
