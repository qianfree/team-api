//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestUsageLogList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/usage-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)

	t.Logf("Usage log list: total=%d", resp.GetTotal(t))
}

func TestUsageLogListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Date range filter
	resp := client.Get("/api/admin/usage-logs", map[string]string{
		"page":       "1",
		"page_size":  "10",
		"start_date": "2025-01-01",
		"end_date":   "2030-12-31",
	})
	testinfra.AssertPaginatedList(t, resp, 0)

	// Status filter
	resp2 := client.Get("/api/admin/usage-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "success",
	})
	testinfra.AssertPaginatedList(t, resp2, 0)

	// Verify status filter correctness
	if resp2.GetTotal(t) > 0 {
		var data struct {
			List []struct {
				Id     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"list"`
		}
		resp2.DecodeData(t, &data)

		for _, item := range data.List {
			if item.Status != "success" {
				t.Fatalf("filter status=success returned item with status=%q (id=%d)", item.Status, item.Id)
			}
		}
	}

	// Narrow date range (should have fewer results)
	narrowResp := client.Get("/api/admin/usage-logs", map[string]string{
		"page":       "1",
		"page_size":  "10",
		"start_date": "2026-05-01",
		"end_date":   "2026-05-27",
	})
	narrowResp.AssertSuccess(t)

	// Tenant filter
	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	tenantResp := client.Get("/api/admin/usage-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"tenant_id": fmt.Sprintf("%d", tenantID),
	})
	tenantResp.AssertSuccess(t)

	// New tenant should have no usage
	var tenantData struct {
		Total int `json:"total"`
	}
	tenantResp.DecodeData(t, &tenantData)
	if tenantData.Total != 0 {
		t.Fatalf("new tenant should have 0 usage logs, got %d", tenantData.Total)
	}
}

func TestUsageLogCleanupCreate(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create a cleanup task with dry_run=true (non-destructive)
	createResp := client.Post("/api/admin/usage-logs/cleanup", map[string]any{
		"start_time": "2025-01-01T00:00:00Z",
		"end_time":   "2025-12-31T23:59:59Z",
		"batch_size": 5000,
		"dry_run":    true,
	})
	createResp.AssertSuccess(t)

	var createData struct {
		TaskID int64 `json:"task_id"`
	}
	createResp.DecodeData(t, &createData)

	if createData.TaskID <= 0 {
		t.Fatalf("expected valid task_id, got %d", createData.TaskID)
	}

	t.Logf("Cleanup task created: task_id=%d", createData.TaskID)
}

func TestUsageLogCleanupList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create a cleanup task first
	createResp := client.Post("/api/admin/usage-logs/cleanup", map[string]any{
		"start_time": "2025-01-01T00:00:00Z",
		"end_time":   "2025-12-31T23:59:59Z",
		"batch_size": 5000,
		"dry_run":    true,
	})
	createResp.AssertSuccess(t)

	// List cleanup tasks
	listResp := client.Get("/api/admin/usage-logs/cleanup/tasks", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, listResp, 1)

	var listData struct {
		List []struct {
			ID     int64  `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) > 0 {
		task := listData.List[0]
		if task.ID <= 0 {
			t.Fatalf("expected valid task ID, got %d", task.ID)
		}
		if task.Status == "" {
			t.Fatal("task should have non-empty status")
		}
		t.Logf("Cleanup task: id=%d, name=%q, status=%q", task.ID, task.Name, task.Status)
	}
}

func TestUsageLogCleanupCancel(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create a cleanup task
	createResp := client.Post("/api/admin/usage-logs/cleanup", map[string]any{
		"start_time": "2025-01-01T00:00:00Z",
		"end_time":   "2025-12-31T23:59:59Z",
		"batch_size": 5000,
		"dry_run":    true,
	})
	createResp.AssertSuccess(t)

	var createData struct {
		TaskID int64 `json:"task_id"`
	}
	createResp.DecodeData(t, &createData)

	if createData.TaskID <= 0 {
		t.Skip("Failed to create cleanup task for cancel test")
	}

	// Cancel the cleanup task
	cancelResp := client.Post(fmt.Sprintf("/api/admin/usage-logs/cleanup/tasks/%d/cancel", createData.TaskID), nil)
	cancelResp.AssertSuccess(t)

	// Verify the task is cancelled via list
	listResp := client.Get("/api/admin/usage-logs/cleanup/tasks", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	for _, task := range listData.List {
		if task.ID == createData.TaskID {
			if task.Status != "cancelled" {
				t.Fatalf("expected status=cancelled after cancel, got %q", task.Status)
			}
			return
		}
	}

	t.Logf("Cleanup task %d cancelled", createData.TaskID)
}

func TestUsageLogExport(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// CSV export
	csvResp := client.Get("/api/admin/usage-logs/export", map[string]string{
		"format": "csv",
	})
	csvResp.AssertSuccess(t)

	// XLSX export
	xlsxResp := client.Get("/api/admin/usage-logs/export", map[string]string{
		"format": "xlsx",
	})
	xlsxResp.AssertSuccess(t)

	// Export with filters
	filteredResp := client.Get("/api/admin/usage-logs/export", map[string]string{
		"format":     "csv",
		"status":     "success",
		"start_date": "2025-01-01",
		"end_date":   "2030-12-31",
	})
	filteredResp.AssertSuccess(t)
}

func TestUsageLogNegative(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Cleanup with missing required fields
	missingResp := client.Post("/api/admin/usage-logs/cleanup", map[string]any{})
	if missingResp.Code == 0 {
		t.Fatal("expected error for missing cleanup fields, got success")
	}

	// Cleanup with invalid batch_size (too small)
	invalidBatchResp := client.Post("/api/admin/usage-logs/cleanup", map[string]any{
		"start_time": "2025-01-01T00:00:00Z",
		"end_time":   "2025-12-31T23:59:59Z",
		"batch_size": 10,
	})
	if invalidBatchResp.Code == 0 {
		t.Fatal("expected error for batch_size < 100, got success")
	}

	// Cancel non-existent cleanup task
	cancelNonExistResp := client.Post("/api/admin/usage-logs/cleanup/tasks/999999999/cancel", nil)
	if cancelNonExistResp.Code == 0 {
		t.Fatal("expected error when cancelling non-existent cleanup task, got success")
	}

	// start_time > end_time should fail
	invalidTimeResp := client.Post("/api/admin/usage-logs/cleanup", map[string]any{
		"start_time": "2025-12-31T23:59:59Z",
		"end_time":   "2025-01-01T00:00:00Z",
	})
	if invalidTimeResp.Code == 0 {
		t.Fatal("expected error for start_time > end_time, got success")
	}
}
