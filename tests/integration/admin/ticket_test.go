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
	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
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

// Note: Ticket creation happens through the tenant console, not the admin console.
// The admin can only view, reply, assign, and update ticket status.
// These tests verify the admin-side operations assuming tickets exist in the system.

func TestTicketDetail(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// List tickets first to find one
	listResp := client.Get("/api/admin/tickets", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			Id     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"list"`
		Total int `json:"total"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) == 0 {
		t.Skip("No tickets found, skipping detail test")
	}

	ticketID := listData.List[0].Id

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

	// Find a ticket to reply to
	listResp := client.Get("/api/admin/tickets", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) == 0 {
		t.Skip("No tickets found, skipping reply test")
	}

	ticketID := listData.List[0].Id

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

	// Find a ticket to assign
	listResp := client.Get("/api/admin/tickets", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) == 0 {
		t.Skip("No tickets found, skipping assign test")
	}

	ticketID := listData.List[0].Id

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

	// Find a ticket
	listResp := client.Get("/api/admin/tickets", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "pending",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			Id     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) == 0 {
		t.Skip("No pending tickets found, skipping status update test")
	}

	ticketID := listData.List[0].Id

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

	// Reply with empty content
	listResp := client.Get("/api/admin/tickets", map[string]string{
		"page":      "1",
		"page_size": "1",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) > 0 {
		emptyReplyResp := client.Post(fmt.Sprintf("/api/admin/tickets/%d/reply", listData.List[0].Id), map[string]any{
			"content": "",
		})
		if emptyReplyResp.Code == 0 {
			t.Fatal("expected error for empty reply content, got success")
		}
	}

	// Invalid status value
	if len(listData.List) > 0 {
		invalidStatusResp := client.Put(fmt.Sprintf("/api/admin/tickets/%d/status", listData.List[0].Id), map[string]any{
			"status": "invalid_status",
		})
		if invalidStatusResp.Code == 0 {
			t.Fatal("expected error for invalid ticket status, got success")
		}
	}
}
