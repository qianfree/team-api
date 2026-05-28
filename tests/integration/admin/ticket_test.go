//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
	admintest "github.com/qianfree/team-api/tests/integration/admin/testinfra"
	tenanttest "github.com/qianfree/team-api/tests/integration/tenant/testinfra"
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
		"status":    "pending",
	})
	resp.AssertSuccess(t)

	// Filter by category
	resp = client.Get("/api/admin/tickets", map[string]string{
		"page":      "1",
		"page_size": "10",
		"category":  "billing",
	})
	resp.AssertSuccess(t)

	// Filter by tenant_id
	tenantID, cleanup := createTestTenantWithTicket(t, client)
	defer cleanup()

	resp = client.Get("/api/admin/tickets", map[string]string{
		"page":      "1",
		"page_size": "10",
		"tenant_id": fmt.Sprintf("%d", tenantID),
	})
	resp.AssertSuccess(t)

	// Multiple filters combined
	resp = client.Get("/api/admin/tickets", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "pending",
		"category":  "technical",
	})
	resp.AssertSuccess(t)
}

func TestTicketDetail(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create test ticket via tenant
	ticketID, cleanup := createTestTicketViaTenant(t, client)
	defer cleanup()

	// Get ticket detail
	detailResp := client.Get(fmt.Sprintf("/api/admin/tickets/%d", ticketID), nil)
	detailResp.AssertSuccess(t)

	var detail map[string]any
	detailResp.DecodeData(t, &detail)

	// Verify essential fields exist
	requiredFields := []string{"id", "title", "status", "category"}
	for _, field := range requiredFields {
		if _, ok := detail[field]; !ok {
			t.Fatalf("ticket detail missing required field: %s", field)
		}
	}

	// Verify id matches
	if id, ok := detail["id"].(float64); !ok || int64(id) != ticketID {
		t.Fatalf("expected ticket id=%d in detail response", ticketID)
	}

	t.Logf("Ticket detail retrieved: id=%d, status=%v", ticketID, detail["status"])
}

func TestTicketReply(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create test ticket via tenant
	ticketID, cleanup := createTestTicketViaTenant(t, client)
	defer cleanup()

	// Reply to the ticket
	replyContent := fmt.Sprintf("管理员回复（集成测试）%s", randomSuffix())
	replyResp := client.Post(fmt.Sprintf("/api/admin/tickets/%d/reply", ticketID), map[string]any{
		"content": replyContent,
	})
	replyResp.AssertSuccess(t)

	t.Logf("Replied to ticket %d", ticketID)
}

func TestTicketAssign(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create test ticket via tenant
	ticketID, cleanup := createTestTicketViaTenant(t, client)
	defer cleanup()

	// Get current admin user list to find an admin ID
	adminListResp := client.Get("/api/admin/users", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	adminListResp.AssertSuccess(t)

	var adminData struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
	}
	adminListResp.DecodeData(t, &adminData)

	if len(adminData.List) == 0 {
		t.Skip("No admin users found, skipping assign test")
	}

	adminID := adminData.List[0].Id

	// Assign the ticket
	assignResp := client.Put(fmt.Sprintf("/api/admin/tickets/%d/assign", ticketID), map[string]any{
		"admin_id": adminID,
	})
	assignResp.AssertSuccess(t)

	// Verify assignment via detail
	detailResp := client.Get(fmt.Sprintf("/api/admin/tickets/%d", ticketID), nil)
	detailResp.AssertSuccess(t)

	var detail map[string]any
	detailResp.DecodeData(t, &detail)

	if assignedAdmin, ok := detail["assigned_admin_id"]; ok {
		if adminIDFloat, ok := assignedAdmin.(float64); ok && int64(adminIDFloat) != adminID {
			t.Fatalf("expected assigned_admin_id=%d, got %v", adminID, assignedAdmin)
		}
	}

	t.Logf("Ticket %d assigned to admin %d", ticketID, adminID)
}

func TestTicketStatusUpdate(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create test ticket via tenant
	ticketID, cleanup := createTestTicketViaTenant(t, client)
	defer cleanup()

	// Update status to processing
	updateResp := client.Put(fmt.Sprintf("/api/admin/tickets/%d/status", ticketID), map[string]any{
		"status": "processing",
	})
	updateResp.AssertSuccess(t)

	// Verify status change via detail
	detailResp := client.Get(fmt.Sprintf("/api/admin/tickets/%d", ticketID), nil)
	detailResp.AssertSuccess(t)

	var detail struct {
		Status string `json:"status"`
	}
	detailResp.DecodeData(t, &detail)

	if detail.Status != "processing" {
		t.Fatalf("expected status=processing after update, got %q", detail.Status)
	}

	// Update status to closed
	closeResp := client.Put(fmt.Sprintf("/api/admin/tickets/%d/status", ticketID), map[string]any{
		"status": "closed",
	})
	closeResp.AssertSuccess(t)

	// Verify closed status
	verifyResp := client.Get(fmt.Sprintf("/api/admin/tickets/%d", ticketID), nil)
	verifyResp.AssertSuccess(t)

	var verify struct {
		Status string `json:"status"`
	}
	verifyResp.DecodeData(t, &verify)

	if verify.Status != "closed" {
		t.Fatalf("expected status=closed, got %q", verify.Status)
	}

	t.Logf("Ticket %d status updated: pending -> processing -> closed", ticketID)
}

func TestTicketNegative(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Detail for non-existent ticket
	detailNonExistResp := client.Get("/api/admin/tickets/999999999", nil)
	if detailNonExistResp.Code == 0 {
		t.Fatal("expected error for non-existent ticket detail, got success")
	}

	// Reply to non-existent ticket
	replyNonExistResp := client.Post("/api/admin/tickets/999999999/reply", map[string]any{
		"content": "test",
	})
	if replyNonExistResp.Code == 0 {
		t.Fatal("expected error when replying to non-existent ticket, got success")
	}

	// Reply with empty content — test against a real ticket
	ticketID, cleanup := createTestTicketViaTenant(t, client)
	defer cleanup()

	emptyReplyResp := client.Post(fmt.Sprintf("/api/admin/tickets/%d/reply", ticketID), map[string]any{
		"content": "",
	})
	if emptyReplyResp.Code == 0 {
		t.Fatal("expected error for empty reply content, got success")
	}

	// Invalid status value
	invalidStatusResp := client.Put(fmt.Sprintf("/api/admin/tickets/%d/status", ticketID), map[string]any{
		"status": "invalid_status",
	})
	if invalidStatusResp.Code == 0 {
		t.Fatal("expected error for invalid ticket status, got success")
	}
}

// createTestTicketViaTenant creates a test tenant, creates a ticket via the tenant API,
// and returns the ticket ID and cleanup.
func createTestTicketViaTenant(t *testing.T, adminClient *admintest.APIClient) (ticketID int64, cleanup func()) {
	t.Helper()

	result := tenanttest.RegisterTestTenant(t)
	tenantClient := admintest.NewAPIClient(tenanttest.DefaultBaseURL).WithToken(result.AccessToken)

	suffix := tenanttest.RandomSuffix()
	resp := tenantClient.Post("/api/tenant/tickets", map[string]any{
		"category":    "technical",
		"title":       fmt.Sprintf("测试工单 %s", suffix),
		"description": "集成测试自动创建的工单",
		"urgency":     "normal",
	})
	resp.AssertSuccess(t)

	var data struct {
		ID int64 `json:"id"`
	}
	resp.DecodeData(t, &data)

	return data.ID, func() {
		// Tenant cleanup is handled by t.Cleanup in RegisterTestTenant
	}
}

// createTestTenantWithTicket creates a test tenant with a ticket for filter tests.
func createTestTenantWithTicket(t *testing.T, adminClient *admintest.APIClient) (tenantID int64, cleanup func()) {
	t.Helper()

	result := tenanttest.RegisterTestTenant(t)
	tenantClient := admintest.NewAPIClient(tenanttest.DefaultBaseURL).WithToken(result.AccessToken)

	suffix := tenanttest.RandomSuffix()
	resp := tenantClient.Post("/api/tenant/tickets", map[string]any{
		"category":    "billing",
		"title":       fmt.Sprintf("过滤测试工单 %s", suffix),
		"description": "过滤测试自动创建的工单",
		"urgency":     "normal",
	})
	resp.AssertSuccess(t)

	return result.Tenant.ID, func() {
		// Tenant cleanup is handled by t.Cleanup in RegisterTestTenant
	}
}
