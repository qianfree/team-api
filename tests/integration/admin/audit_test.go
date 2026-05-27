//go:build integration

package admin_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestAuditConfigGet(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/audit/config", nil)
	resp.AssertSuccess(t)

	var data struct {
		AuditLevel string `json:"audit_level"`
	}
	resp.DecodeData(t, &data)

	validLevels := map[string]bool{
		"full": true, "full_text": true, "masked": true,
		"question_only": true, "none": true,
	}
	if !validLevels[data.AuditLevel] {
		t.Fatalf("unexpected audit_level %q", data.AuditLevel)
	}

	t.Logf("Audit config: level=%s", data.AuditLevel)
}

func TestAuditConfigUpdate(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// First get current config
	getResp := client.Get("/api/admin/audit/config", nil)
	getResp.AssertSuccess(t)

	var current struct {
		AuditLevel string `json:"audit_level"`
	}
	getResp.DecodeData(t, &current)

	// Update with same value (non-destructive)
	updateResp := client.Put("/api/admin/audit/config", map[string]any{
		"audit_level": current.AuditLevel,
	})
	updateResp.AssertSuccess(t)

	// Verify it stayed the same
	verifyResp := client.Get("/api/admin/audit/config", nil)
	verifyResp.AssertSuccess(t)

	var after struct {
		AuditLevel string `json:"audit_level"`
	}
	verifyResp.DecodeData(t, &after)

	if after.AuditLevel != current.AuditLevel {
		t.Fatalf("audit_level changed: %s -> %s", current.AuditLevel, after.AuditLevel)
	}

	t.Logf("Audit config update verified: level=%s", after.AuditLevel)
}

func TestOperationLogList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/audit/operation-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestOperationLogListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by user_type
	resp := client.Get("/api/admin/audit/operation-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"user_type": "admin",
	})
	resp.AssertSuccess(t)

	// Filter by action
	resp = client.Get("/api/admin/audit/operation-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"action":    "create",
	})
	resp.AssertSuccess(t)

	t.Logf("Operation log filters applied successfully")
}

func TestSensitiveLogList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/audit/sensitive-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestSensitiveLogListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by resource_type
	resp := client.Get("/api/admin/audit/sensitive-logs", map[string]string{
		"page":          "1",
		"page_size":     "10",
		"resource_type": "api_key",
	})
	resp.AssertSuccess(t)

	t.Logf("Sensitive log filters applied successfully")
}

func TestRequestAuditLogList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/audit/request-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestRequestAuditLogListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by method
	resp := client.Get("/api/admin/audit/request-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"method":    "GET",
	})
	resp.AssertSuccess(t)

	// Filter by status_code
	resp = client.Get("/api/admin/audit/request-logs", map[string]string{
		"page":        "1",
		"page_size":   "10",
		"status_code": "200",
	})
	resp.AssertSuccess(t)

	t.Logf("Request audit log filters applied successfully")
}

func TestContentFilterLogList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/audit/content-filter-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestContentFilterLogListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by blocked
	resp := client.Get("/api/admin/audit/content-filter-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"blocked":   "true",
	})
	resp.AssertSuccess(t)

	// Filter by mode
	resp = client.Get("/api/admin/audit/content-filter-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"mode":      "input",
	})
	resp.AssertSuccess(t)

	t.Logf("Content filter log filters applied successfully")
}
