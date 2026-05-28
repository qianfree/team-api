//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
	admintest "github.com/qianfree/team-api/tests/integration/admin/testinfra"
	tenanttest "github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestFeedbackList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/feedbacks", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestFeedbackListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by status
	resp := client.Get("/api/admin/feedbacks", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "pending",
	})
	resp.AssertSuccess(t)

	// Filter by category
	resp = client.Get("/api/admin/feedbacks", map[string]string{
		"page":      "1",
		"page_size": "10",
		"category":  "bug",
	})
	resp.AssertSuccess(t)

	// Filter by priority
	resp = client.Get("/api/admin/feedbacks", map[string]string{
		"page":      "1",
		"page_size": "10",
		"priority":  "high",
	})
	resp.AssertSuccess(t)

	// Filter by tenant_id
	tenantID, cleanup := createTestTenantWithFeedback(t, client)
	defer cleanup()

	resp = client.Get("/api/admin/feedbacks", map[string]string{
		"page":      "1",
		"page_size": "10",
		"tenant_id": fmt.Sprintf("%d", tenantID),
	})
	resp.AssertSuccess(t)
}

func TestFeedbackReply(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create test tenant with feedback
	feedbackID, cleanup := createTestFeedbackViaTenant(t, client)
	defer cleanup()

	// Reply and update status to acknowledged
	replyText := fmt.Sprintf("管理员回复（集成测试）%s", randomSuffix())
	replyResp := client.Post(fmt.Sprintf("/api/admin/feedbacks/%d/reply", feedbackID), map[string]any{
		"reply":  replyText,
		"status": "acknowledged",
	})
	replyResp.AssertSuccess(t)

	// Verify status changed via list
	verifyResp := client.Get("/api/admin/feedbacks", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	verifyResp.AssertSuccess(t)

	var verifyData struct {
		List []struct {
			Id         int64  `json:"id"`
			Status     string `json:"status"`
			AdminReply string `json:"admin_reply"`
		} `json:"list"`
	}
	verifyResp.DecodeData(t, &verifyData)

	for _, item := range verifyData.List {
		if item.Id == feedbackID {
			if item.Status != "acknowledged" {
				t.Fatalf("expected status=acknowledged after reply, got %q", item.Status)
			}
			if item.AdminReply != replyText {
				t.Fatalf("expected admin_reply=%q, got %q", replyText, item.AdminReply)
			}
			return
		}
	}

	t.Fatalf("feedback id=%d not found after reply", feedbackID)
}

func TestFeedbackStatusUpdate(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create test tenant with feedback
	feedbackID, cleanup := createTestFeedbackViaTenant(t, client)
	defer cleanup()

	// Update status to in_progress and priority to high
	updateResp := client.Put(fmt.Sprintf("/api/admin/feedbacks/%d/status", feedbackID), map[string]any{
		"status":   "in_progress",
		"priority": "high",
	})
	updateResp.AssertSuccess(t)

	// Verify status and priority changed
	verifyResp := client.Get("/api/admin/feedbacks", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	verifyResp.AssertSuccess(t)

	var verifyData struct {
		List []struct {
			Id       int64  `json:"id"`
			Status   string `json:"status"`
			Priority string `json:"priority"`
		} `json:"list"`
	}
	verifyResp.DecodeData(t, &verifyData)

	for _, item := range verifyData.List {
		if item.Id == feedbackID {
			if item.Status != "in_progress" {
				t.Fatalf("expected status=in_progress, got %q", item.Status)
			}
			if item.Priority != "high" {
				t.Fatalf("expected priority=high, got %q", item.Priority)
			}
			return
		}
	}

	t.Fatalf("feedback id=%d not found after status update", feedbackID)
}

func TestFeedbackStats(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/feedbacks/stats", nil)
	resp.AssertSuccess(t)

	var data struct {
		Total        int            `json:"total"`
		Pending      int            `json:"pending"`
		Acknowledged int            `json:"acknowledged"`
		InProgress   int            `json:"in_progress"`
		Resolved     int            `json:"resolved"`
		Closed       int            `json:"closed"`
		ByCategory   map[string]int `json:"by_category"`
		RecentTrend  []struct {
			Date  string `json:"date"`
			Count int    `json:"count"`
		} `json:"recent_trend"`
	}
	resp.DecodeData(t, &data)

	// Verify total is non-negative and consistent with status counts
	if data.Total < 0 {
		t.Fatalf("expected total >= 0, got %d", data.Total)
	}

	statusSum := data.Pending + data.Acknowledged + data.InProgress + data.Resolved + data.Closed
	if statusSum > data.Total {
		t.Fatalf("status sum (%d) exceeds total (%d)", statusSum, data.Total)
	}

	// Verify by_category values are non-negative
	for cat, count := range data.ByCategory {
		if count < 0 {
			t.Fatalf("by_category[%q] = %d, expected >= 0", cat, count)
		}
	}

	t.Logf("Feedback stats verified: total=%d, sum=%d, categories=%d",
		data.Total, statusSum, len(data.ByCategory))
}

func TestFeedbackNegative(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Reply to non-existent feedback
	replyNonExistResp := client.Post("/api/admin/feedbacks/999999999/reply", map[string]any{
		"reply":  "test",
		"status": "acknowledged",
	})
	if replyNonExistResp.Code == 0 {
		t.Fatal("expected error when replying to non-existent feedback, got success")
	}

	// Update non-existent feedback status
	statusNonExistResp := client.Put("/api/admin/feedbacks/999999999/status", map[string]any{
		"status": "resolved",
	})
	if statusNonExistResp.Code == 0 {
		t.Fatal("expected error when updating non-existent feedback status, got success")
	}

	// Reply with empty content
	replyEmptyResp := client.Post("/api/admin/feedbacks/999999999/reply", map[string]any{
		"reply":  "",
		"status": "acknowledged",
	})
	if replyEmptyResp.Code == 0 {
		t.Fatal("expected error for empty reply content, got success")
	}

	// Invalid status value
	listResp := client.Get("/api/admin/feedbacks", map[string]string{
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
		invalidStatusResp := client.Put(fmt.Sprintf("/api/admin/feedbacks/%d/status", listData.List[0].Id), map[string]any{
			"status": "invalid_status",
		})
		if invalidStatusResp.Code == 0 {
			t.Fatal("expected error for invalid feedback status, got success")
		}
	}
}

// createTestFeedbackViaTenant creates a test tenant, registers a tenant user,
// creates a feedback via the tenant API, and returns the feedback ID and cleanup.
func createTestFeedbackViaTenant(t *testing.T, adminClient *admintest.APIClient) (feedbackID int64, cleanup func()) {
	t.Helper()

	result := tenanttest.RegisterTestTenant(t)
	tenantClient := admintest.NewAPIClient(tenanttest.DefaultBaseURL).WithToken(result.AccessToken)

	suffix := tenanttest.RandomSuffix()
	resp := tenantClient.Post("/api/tenant/feedbacks", map[string]any{
		"category":    "bug_report",
		"title":       fmt.Sprintf("管理员测试反馈 %s", suffix),
		"description": "管理员集成测试自动创建的反馈",
	})
	resp.AssertSuccess(t)

	var data struct {
		ID int64 `json:"id"`
	}
	resp.DecodeData(t, &data)

	return data.ID, func() {
		// Feedback and tenant cleaned up by hardDeleteTenant in RegisterTestTenant
	}
}

// createTestTenantWithFeedback creates a test tenant with a feedback for filter tests.
func createTestTenantWithFeedback(t *testing.T, adminClient *admintest.APIClient) (tenantID int64, cleanup func()) {
	t.Helper()

	// Register a test tenant
	result := tenanttest.RegisterTestTenant(t)
	tenantClient := admintest.NewAPIClient(tenanttest.DefaultBaseURL).WithToken(result.AccessToken)

	// Create feedback via tenant API
	suffix := tenanttest.RandomSuffix()
	resp := tenantClient.Post("/api/tenant/feedbacks", map[string]any{
		"category":    "bug_report",
		"title":       fmt.Sprintf("过滤测试反馈 %s", suffix),
		"description": "过滤测试自动创建的反馈",
	})
	resp.AssertSuccess(t)

	return result.Tenant.ID, func() {
		// Tenant cleanup is handled by t.Cleanup in RegisterTestTenant
	}
}
