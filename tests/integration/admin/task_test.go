//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestTaskList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/tasks", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestTaskListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by status
	resp := client.Get("/api/admin/tasks", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "completed",
	})
	resp.AssertSuccess(t)

	// Filter by platform=suno
	resp = client.Get("/api/admin/tasks", map[string]string{
		"page":      "1",
		"page_size": "10",
		"platform":  "suno",
	})
	resp.AssertSuccess(t)

	// Filter by platform=kling
	resp = client.Get("/api/admin/tasks", map[string]string{
		"page":      "1",
		"page_size": "10",
		"platform":  "kling",
	})
	resp.AssertSuccess(t)

	// Filter by non-existent public_task_id
	resp = client.Get("/api/admin/tasks", map[string]string{
		"page":           "1",
		"page_size":      "10",
		"public_task_id": "nonexistent-task-id-12345",
	})
	resp.AssertSuccess(t)

	// Verify empty result for non-existent ID
	var emptyData struct {
		Total int `json:"total"`
	}
	resp.DecodeData(t, &emptyData)
	if emptyData.Total != 0 {
		t.Fatalf("expected 0 results for non-existent public_task_id, got %d", emptyData.Total)
	}

	// Combined filters: status + platform
	resp = client.Get("/api/admin/tasks", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "completed",
		"platform":  "suno",
	})
	resp.AssertSuccess(t)

	// Verify platform filter correctness
	if resp.GetTotal(t) > 0 {
		var filteredData struct {
			List []struct {
				Id       int64  `json:"id"`
				Platform string `json:"platform"`
			} `json:"list"`
		}
		resp.DecodeData(t, &filteredData)

		for _, task := range filteredData.List {
			if task.Platform != "suno" {
				t.Fatalf("filter platform=suno returned task with platform=%q (id=%d)", task.Platform, task.Id)
			}
		}
	}
}

func TestTaskDetail(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// List first to find a task
	listResp := client.Get("/api/admin/tasks", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			ID              int64   `json:"id"`
			PublicTaskID    string  `json:"public_task_id"`
			Platform        string  `json:"platform"`
			Status          string  `json:"status"`
			ModelName       string  `json:"model_name"`
			TenantID        int64   `json:"tenant_id"`
			PreDeductAmount float64 `json:"pre_deduct_amount"`
			ActualCost      float64 `json:"actual_cost"`
			BillingSettled  bool    `json:"billing_settled"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) == 0 {
		t.Skip("No tasks found, skipping detail test")
	}

	taskID := listData.List[0].ID
	expectedPlatform := listData.List[0].Platform
	expectedStatus := listData.List[0].Status

	// Get task detail
	detailResp := client.Get(fmt.Sprintf("/api/admin/tasks/%d", taskID), nil)
	detailResp.AssertSuccess(t)

	var detailData struct {
		Task struct {
			ID        int64  `json:"id"`
			Platform  string `json:"platform"`
			Status    string `json:"status"`
			ModelName string `json:"model_name"`
			TenantID  int64  `json:"tenant_id"`
		} `json:"task"`
	}
	detailResp.DecodeData(t, &detailData)

	if detailData.Task.ID != taskID {
		t.Fatalf("expected task id=%d, got %d", taskID, detailData.Task.ID)
	}
	if detailData.Task.Platform != expectedPlatform {
		t.Fatalf("expected platform=%q, got %q", expectedPlatform, detailData.Task.Platform)
	}
	if detailData.Task.Status != expectedStatus {
		t.Fatalf("expected status=%q, got %q", expectedStatus, detailData.Task.Status)
	}

	t.Logf("Task detail verified: id=%d, platform=%s, status=%s", taskID, expectedPlatform, expectedStatus)
}

func TestTaskCancel(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Find a pending/processing task to cancel
	listResp := client.Get("/api/admin/tasks", map[string]string{
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
		// Try processing status
		listResp2 := client.Get("/api/admin/tasks", map[string]string{
			"page":      "1",
			"page_size": "10",
			"status":    "processing",
		})
		listResp2.AssertSuccess(t)
		listResp2.DecodeData(t, &listData)
	}

	if len(listData.List) == 0 {
		t.Skip("No pending/processing tasks found, skipping cancel test")
	}

	taskID := listData.List[0].Id

	// Cancel the task
	cancelResp := client.Post(fmt.Sprintf("/api/admin/tasks/%d/cancel", taskID), nil)
	cancelResp.AssertSuccess(t)

	// Verify task is now cancelled via detail
	detailResp := client.Get(fmt.Sprintf("/api/admin/tasks/%d", taskID), nil)
	detailResp.AssertSuccess(t)

	var detailData struct {
		Task struct {
			Status string `json:"status"`
		} `json:"task"`
	}
	detailResp.DecodeData(t, &detailData)

	if detailData.Task.Status != "cancelled" {
		t.Fatalf("expected status=cancelled after cancel, got %q", detailData.Task.Status)
	}

	t.Logf("Task %d cancelled successfully", taskID)
}

func TestTaskBillingFields(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// List completed tasks to verify billing fields
	listResp := client.Get("/api/admin/tasks", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "completed",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			ID              int64   `json:"id"`
			PreDeductAmount float64 `json:"pre_deduct_amount"`
			ActualCost      float64 `json:"actual_cost"`
			BillingSettled  bool    `json:"billing_settled"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) == 0 {
		t.Skip("No completed tasks found, skipping billing fields test")
	}

	// Verify completed tasks have valid billing data
	for _, task := range listData.List {
		if task.PreDeductAmount < 0 {
			t.Fatalf("task %d has negative pre_deduct_amount=%f", task.ID, task.PreDeductAmount)
		}
		if task.ActualCost < 0 {
			t.Fatalf("task %d has negative actual_cost=%f", task.ID, task.ActualCost)
		}
	}

	t.Logf("Verified billing fields for %d completed tasks", len(listData.List))
}

func TestTaskNegative(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Detail for non-existent task
	detailNonExistResp := client.Get("/api/admin/tasks/999999999", nil)
	if detailNonExistResp.Code == 0 {
		t.Fatal("expected error for non-existent task detail, got success")
	}

	// Cancel non-existent task
	cancelNonExistResp := client.Post("/api/admin/tasks/999999999/cancel", nil)
	if cancelNonExistResp.Code == 0 {
		t.Fatal("expected error when cancelling non-existent task, got success")
	}

	// Invalid platform filter
	invalidPlatformResp := client.Get("/api/admin/tasks", map[string]string{
		"page":      "1",
		"page_size": "10",
		"platform":  "nonexistent_platform",
	})
	// Platform filter may or may not reject invalid values — just verify it doesn't crash
	invalidPlatformResp.AssertSuccess(t)
}
