//go:build integration

package admin_test

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func randomSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func TestTenantList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create a tenant to ensure at least one exists
	_, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	resp := client.Get("/api/admin/tenants", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 1)
}

func TestTenantListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	_, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	// Filter by status
	resp := client.Get("/api/admin/tenants", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "active",
	})
	resp.AssertSuccess(t)

	// Filter by keyword
	resp = client.Get("/api/admin/tenants", map[string]string{
		"page":      "1",
		"page_size": "10",
		"keyword":   "test-tenant",
	})
	resp.AssertSuccess(t)
}

func TestTenantCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// --- Create ---
	suffix := randomSuffix()
	createResp := client.Post("/api/admin/tenants", map[string]any{
		"tenant_name":     fmt.Sprintf("CRUD测试租户 %s", suffix),
		"tenant_code":     fmt.Sprintf("crud-tenant-%s", suffix),
		"username":        fmt.Sprintf("crudowner%s", suffix),
		"email":           fmt.Sprintf("crudowner-%s@test.com", suffix),
		"password":        "OwnerPass123!",
		"max_members":     20,
		"max_concurrency": 5,
	})
	createResp.AssertSuccess(t)
	tenantID := createResp.GetID(t)
	defer func() {
		client.Put(fmt.Sprintf("/api/admin/tenants/%d/status", tenantID), map[string]any{
			"status": "closed",
		})
	}()

	// --- Get detail ---
	detailResp := client.Get(fmt.Sprintf("/api/admin/tenants/%d", tenantID), nil)
	detailResp.AssertSuccess(t)

	var detail struct {
		ID             int64  `json:"id"`
		TenantName     string `json:"tenant_name"`
		TenantCode     string `json:"tenant_code"`
		MaxMembers     int    `json:"max_members"`
		MaxConcurrency int    `json:"max_concurrency"`
	}
	detailResp.DecodeData(t, &detail)

	if detail.ID != tenantID {
		t.Fatalf("expected tenant id %d, got %d", tenantID, detail.ID)
	}
	if detail.MaxMembers != 20 {
		t.Fatalf("expected max_members=20, got %d", detail.MaxMembers)
	}

	// --- Update ---
	updateResp := client.Put(fmt.Sprintf("/api/admin/tenants/%d", tenantID), map[string]any{
		"name":            fmt.Sprintf("更新租户名 %s", suffix),
		"max_members":     50,
		"max_concurrency": 10,
	})
	updateResp.AssertSuccess(t)

	// Verify update
	verifyResp := client.Get(fmt.Sprintf("/api/admin/tenants/%d", tenantID), nil)
	verifyResp.AssertSuccess(t)
	var updated struct {
		MaxMembers     int `json:"max_members"`
		MaxConcurrency int `json:"max_concurrency"`
	}
	verifyResp.DecodeData(t, &updated)
	if updated.MaxMembers != 50 {
		t.Fatalf("expected max_members=50 after update, got %d", updated.MaxMembers)
	}

	// --- List should contain the created tenant ---
	listResp := client.Get("/api/admin/tenants", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	testinfra.AssertPaginatedList(t, listResp, 1)
	total := listResp.GetTotal(t)
	if total < 1 {
		t.Fatalf("expected at least 1 tenant in list, got total=%d", total)
	}
}

func TestTenantStatusUpdate(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	// Suspend the tenant
	suspendResp := client.Put(fmt.Sprintf("/api/admin/tenants/%d/status", tenantID), map[string]any{
		"status": "suspended",
	})
	suspendResp.AssertSuccess(t)

	// Verify status
	detailResp := client.Get(fmt.Sprintf("/api/admin/tenants/%d", tenantID), nil)
	detailResp.AssertSuccess(t)
	var detail struct {
		Status string `json:"status"`
	}
	detailResp.DecodeData(t, &detail)
	if detail.Status != "suspended" {
		t.Fatalf("expected status=suspended, got %s", detail.Status)
	}

	// Reactivate
	activateResp := client.Put(fmt.Sprintf("/api/admin/tenants/%d/status", tenantID), map[string]any{
		"status": "active",
	})
	activateResp.AssertSuccess(t)

	// Verify reactivation
	detailResp2 := client.Get(fmt.Sprintf("/api/admin/tenants/%d", tenantID), nil)
	detailResp2.AssertSuccess(t)
	var detail2 struct {
		Status string `json:"status"`
	}
	detailResp2.DecodeData(t, &detail2)
	if detail2.Status != "active" {
		t.Fatalf("expected status=active after reactivation, got %s", detail2.Status)
	}
}

func TestTenantChannelScope(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	// Update channel scope
	scopeResp := client.Put(fmt.Sprintf("/api/admin/tenants/%d/channel-scope", tenantID), map[string]any{
		"default_channel_scope": "all",
	})
	scopeResp.AssertSuccess(t)
}

func TestTenantSelect(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	_, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	resp := client.Get("/api/admin/tenants/select", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	resp.AssertSuccess(t)

	var result struct {
		List []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			Code string `json:"code"`
		} `json:"list"`
		Total int `json:"total"`
	}
	resp.DecodeData(t, &result)
	if result.Total < 1 {
		t.Fatalf("expected at least 1 tenant in select, got total=%d", result.Total)
	}

	// Filter by keyword
	resp2 := client.Get("/api/admin/tenants/select", map[string]string{
		"keyword":   "test-tenant",
		"page":      "1",
		"page_size": "10",
	})
	resp2.AssertSuccess(t)
}

func TestMemberList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	_, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	resp := client.Get("/api/admin/members", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	resp.AssertSuccess(t)

	// List with filters
	resp = client.Get("/api/admin/members", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "active",
	})
	resp.AssertSuccess(t)
}
