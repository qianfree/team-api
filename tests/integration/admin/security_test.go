//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestLoginHistory(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/security/login-history", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestLoginHistoryWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by success status
	resp := client.Get("/api/admin/security/login-history", map[string]string{
		"page":      "1",
		"page_size": "10",
		"success":   "true",
	})
	resp.AssertSuccess(t)

	// Filter by login method
	resp = client.Get("/api/admin/security/login-history", map[string]string{
		"page":         "1",
		"page_size":    "10",
		"login_method": "password",
	})
	resp.AssertSuccess(t)

	// Filter by username
	resp = client.Get("/api/admin/security/login-history", map[string]string{
		"page":      "1",
		"page_size": "10",
		"username":  "admin",
	})
	resp.AssertSuccess(t)

	t.Logf("Login history filters applied successfully")
}

func TestTenantLoginHistory(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create a tenant to filter by
	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	resp := client.Get("/api/admin/security/tenant-login-history", map[string]string{
		"page":      "1",
		"page_size": "10",
		"tenant_id": fmt.Sprintf("%d", tenantID),
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestTenantLoginHistoryWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by IP address
	resp := client.Get("/api/admin/security/tenant-login-history", map[string]string{
		"page":       "1",
		"page_size":  "10",
		"ip_address": "127.0.0.1",
	})
	resp.AssertSuccess(t)

	// Filter by success
	resp = client.Get("/api/admin/security/tenant-login-history", map[string]string{
		"page":      "1",
		"page_size": "10",
		"success":   "false",
	})
	resp.AssertSuccess(t)

	t.Logf("Tenant login history filters applied successfully")
}
