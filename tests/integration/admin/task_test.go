//go:build integration

package admin_test

import (
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

	// Filter by platform
	resp = client.Get("/api/admin/tasks", map[string]string{
		"page":      "1",
		"page_size": "10",
		"platform":  "suno",
	})
	resp.AssertSuccess(t)

	// Filter by public_task_id
	resp = client.Get("/api/admin/tasks", map[string]string{
		"page":           "1",
		"page_size":      "10",
		"public_task_id": "nonexistent",
	})
	resp.AssertSuccess(t)

	t.Logf("Task list filters applied successfully")
}
