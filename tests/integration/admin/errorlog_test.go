//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestErrorLogList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestErrorLogListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by source
	resp := client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"source":    "relay",
	})
	resp.AssertSuccess(t)

	// Filter by resolved=false
	resp = client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"resolved":  "false",
	})
	resp.AssertSuccess(t)

	// Filter by resolved=true
	resp = client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"resolved":  "true",
	})
	resp.AssertSuccess(t)

	// Filter by error_code
	resp = client.Get("/api/admin/error-logs", map[string]string{
		"page":       "1",
		"page_size":  "10",
		"error_code": "500",
	})
	resp.AssertSuccess(t)

	// Filter by keyword
	resp = client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"keyword":   "timeout",
	})
	resp.AssertSuccess(t)

	// Combined filters: source + resolved
	resp = client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"source":    "relay",
		"resolved":  "false",
	})
	resp.AssertSuccess(t)

	// Date range filter
	resp = client.Get("/api/admin/error-logs", map[string]string{
		"page":       "1",
		"page_size":  "10",
		"start_date": "2025-01-01",
		"end_date":   "2030-12-31",
	})
	resp.AssertSuccess(t)
}

func TestErrorLogDetail(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// List first to find an error log
	listResp := client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
		Total int `json:"total"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) == 0 {
		t.Skip("No error logs found, skipping detail test")
	}

	errorID := listData.List[0].Id

	// Get detail
	detailResp := client.Get(fmt.Sprintf("/api/admin/error-logs/%d", errorID), nil)
	detailResp.AssertSuccess(t)

	var detail map[string]any
	detailResp.DecodeData(t, &detail)

	// Verify essential fields
	requiredFields := []string{"id"}
	for _, field := range requiredFields {
		if _, ok := detail[field]; !ok {
			t.Fatalf("error log detail missing required field: %s", field)
		}
	}

	if id, ok := detail["id"].(float64); !ok || int64(id) != errorID {
		t.Fatalf("expected id=%d in detail response", errorID)
	}

	t.Logf("Error log detail retrieved: id=%d", errorID)
}

func TestErrorLogResolve(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Find an unresolved error log
	listResp := client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"resolved":  "false",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) == 0 {
		t.Skip("No unresolved error logs found, skipping resolve test")
	}

	errorID := listData.List[0].Id

	// Resolve the error
	resolveResp := client.Put(fmt.Sprintf("/api/admin/error-logs/%d/resolve", errorID), nil)
	resolveResp.AssertSuccess(t)

	// Verify the error is now resolved — it should not appear in unresolved filter
	unresolvedResp := client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "50",
		"resolved":  "false",
	})
	unresolvedResp.AssertSuccess(t)

	var unresolvedData struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
	}
	unresolvedResp.DecodeData(t, &unresolvedData)

	for _, item := range unresolvedData.List {
		if item.Id == errorID {
			t.Fatal("resolved error log should not appear in unresolved list")
		}
	}

	// Verify it appears in resolved filter
	resolvedResp := client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "50",
		"resolved":  "true",
	})
	resolvedResp.AssertSuccess(t)

	var resolvedData struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
	}
	resolvedResp.DecodeData(t, &resolvedData)

	found := false
	for _, item := range resolvedData.List {
		if item.Id == errorID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("resolved error log should appear in resolved list")
	}

	t.Logf("Error log %d resolved successfully", errorID)
}

func TestErrorLogBatchResolve(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Find multiple unresolved error logs
	listResp := client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"resolved":  "false",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) < 2 {
		t.Skip("Need at least 2 unresolved error logs for batch resolve test")
	}

	// Pick up to 3 IDs to batch resolve
	ids := make([]int64, 0, 3)
	for i := 0; i < len(listData.List) && i < 3; i++ {
		ids = append(ids, listData.List[i].Id)
	}

	// Batch resolve
	idInterfaces := make([]any, len(ids))
	for i, id := range ids {
		idInterfaces[i] = id
	}
	batchResp := client.Put("/api/admin/error-logs/batch-resolve", map[string]any{
		"ids": idInterfaces,
	})
	batchResp.AssertSuccess(t)

	// Verify none appear in unresolved list
	unresolvedResp := client.Get("/api/admin/error-logs", map[string]string{
		"page":      "1",
		"page_size": "50",
		"resolved":  "false",
	})
	unresolvedResp.AssertSuccess(t)

	var unresolvedData struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
	}
	unresolvedResp.DecodeData(t, &unresolvedData)

	for _, item := range unresolvedData.List {
		for _, resolvedID := range ids {
			if item.Id == resolvedID {
				t.Fatalf("batch-resolved error %d should not appear in unresolved list", resolvedID)
			}
		}
	}

	t.Logf("Batch resolved %d error logs", len(ids))
}

func TestErrorLogStats(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/error-logs/stats", nil)
	resp.AssertSuccess(t)

	var data map[string]any
	resp.DecodeData(t, &data)

	// Stats should return a non-empty map
	if len(data) == 0 {
		t.Fatal("error log stats returned empty data")
	}

	t.Logf("Error log stats returned %d keys", len(data))
}

func TestErrorLogNegative(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Detail for non-existent error log
	detailNonExistResp := client.Get("/api/admin/error-logs/999999999", nil)
	if detailNonExistResp.Code == 0 {
		t.Fatal("expected error for non-existent error log detail, got success")
	}

	// Resolve non-existent error log
	resolveNonExistResp := client.Put("/api/admin/error-logs/999999999/resolve", nil)
	if resolveNonExistResp.Code == 0 {
		t.Fatal("expected error when resolving non-existent error log, got success")
	}

	// Batch resolve with empty IDs
	emptyBatchResp := client.Put("/api/admin/error-logs/batch-resolve", map[string]any{
		"ids": []int64{},
	})
	if emptyBatchResp.Code == 0 {
		t.Fatal("expected error for empty batch resolve IDs, got success")
	}
}
