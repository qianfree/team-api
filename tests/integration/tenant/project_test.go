//go:build integration

package tenant_test

import (
	"fmt"
	"testing"

	admintest "github.com/qianfree/team-api/tests/integration/admin/testinfra"
	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestProjectList(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	_, cleanup := testinfra.CreateTestProject(t, client)
	defer cleanup()

	resp := client.Get("/api/tenant/projects", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	admintest.AssertPaginatedList(t, resp, 1)
}

func TestProjectCRUD(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	// --- Create ---
	suffix := testinfra.RandomSuffix()
	createResp := client.Post("/api/tenant/projects", map[string]any{
		"name":        fmt.Sprintf("crud-project-%s", suffix),
		"description": "Integration test project",
		"budget":      100.0,
	})
	createResp.AssertSuccess(t)
	projectID := createResp.GetID(t)
	defer func() {
		client.Post(fmt.Sprintf("/api/tenant/projects/%d/archive", projectID), nil)
	}()

	// --- Get detail ---
	detailResp := client.Get(fmt.Sprintf("/api/tenant/projects/%d", projectID), nil)
	detailResp.AssertSuccess(t)

	var detail struct {
		Data struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Budget      string `json:"budget"`
			Status      string `json:"status"`
		} `json:"data"`
	}
	detailResp.DecodeData(t, &detail)

	if detail.Data.ID != projectID {
		t.Fatalf("expected id=%d, got %d", projectID, detail.Data.ID)
	}

	// --- Update ---
	updateResp := client.Put(fmt.Sprintf("/api/tenant/projects/%d", projectID), map[string]any{
		"name":        fmt.Sprintf("updated-project-%s", suffix),
		"description": "Updated description",
		"budget":      200.0,
	})
	updateResp.AssertSuccess(t)

	// Verify update
	verifyResp := client.Get(fmt.Sprintf("/api/tenant/projects/%d", projectID), nil)
	verifyResp.AssertSuccess(t)
}

func TestProjectArchiveUnarchive(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	projectID, _ := testinfra.CreateTestProject(t, client)

	// Archive
	archiveResp := client.Post(fmt.Sprintf("/api/tenant/projects/%d/archive", projectID), nil)
	archiveResp.AssertSuccess(t)

	// Verify archived via list
	listResp := client.Get("/api/tenant/projects", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	listResp.AssertSuccess(t)

	// Unarchive
	unarchiveResp := client.Post(fmt.Sprintf("/api/tenant/projects/%d/unarchive", projectID), nil)
	unarchiveResp.AssertSuccess(t)
}

func TestProjectApiKeys(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	projectID, cleanup := testinfra.CreateTestProject(t, client)
	defer cleanup()

	// Create project API key
	createKeyResp := client.Post(fmt.Sprintf("/api/tenant/projects/%d/api-keys", projectID), map[string]any{
		"name": "project-test-key",
	})
	createKeyResp.AssertSuccess(t)

	// List project API keys
	listResp := client.Get(fmt.Sprintf("/api/tenant/projects/%d/api-keys", projectID), map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	admintest.AssertPaginatedList(t, listResp, 1)

	var listData struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)
	if len(listData.List) == 0 {
		t.Fatal("expected at least 1 project API key")
	}

	// Delete project API key
	keyID := listData.List[0].ID
	deleteResp := client.Delete(fmt.Sprintf("/api/tenant/projects/%d/api-keys/%d", projectID, keyID))
	deleteResp.AssertSuccess(t)
}

func TestProjectUsageStats(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	projectID, cleanup := testinfra.CreateTestProject(t, client)
	defer cleanup()

	resp := client.Get(fmt.Sprintf("/api/tenant/projects/%d/usage-stats", projectID), nil)
	resp.AssertSuccess(t)
}

func TestProjectUsageLogs(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	projectID, cleanup := testinfra.CreateTestProject(t, client)
	defer cleanup()

	resp := client.Get(fmt.Sprintf("/api/tenant/projects/%d/usage-logs", projectID), map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	resp.AssertSuccess(t)
}
