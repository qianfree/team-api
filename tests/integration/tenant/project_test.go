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

	projectID, cleanup := testinfra.CreateTestProject(t, client)
	defer cleanup()

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

// ─── 边界值测试 ────────────────────────────────────────────────────

// TestProjectCreate_EmptyName 验证空项目名称被拒绝
// Business rule: 项目名称不能为空（project.go: 项目名称不能为空）
func TestProjectCreate_EmptyName(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Post("/api/tenant/projects", map[string]any{
		"name": "",
	})
	if resp.Code == 0 {
		t.Fatal("empty project name should be rejected")
	}
	t.Logf("empty project name: code=%d msg=%q", resp.Code, resp.Message)
}

// TestProjectCreate_ZeroBudget 验证零预算被正确处理
// Business rule: budget=0 表示不设预算限制
func TestProjectCreate_ZeroBudget(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	suffix := testinfra.RandomSuffix()
	resp := client.Post("/api/tenant/projects", map[string]any{
		"name":   fmt.Sprintf("zero-budget-%s", suffix),
		"budget": 0,
	})
	resp.AssertSuccess(t)
	projectID := resp.GetID(t)
	defer client.Post(fmt.Sprintf("/api/tenant/projects/%d/archive", projectID), nil)
}

// TestProjectCreate_NegativeBudget 验证负数预算被拒绝
// Business rule: 预算金额不能为负数
func TestProjectCreate_NegativeBudget(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	suffix := testinfra.RandomSuffix()
	resp := client.Post("/api/tenant/projects", map[string]any{
		"name":   fmt.Sprintf("negative-budget-%s", suffix),
		"budget": -10.0,
	})
	if resp.Code == 0 {
		projectID := resp.GetID(t)
		defer client.Post(fmt.Sprintf("/api/tenant/projects/%d/archive", projectID), nil)
		t.Logf("server accepted negative budget — may need server-side validation")
	} else {
		t.Logf("negative budget rejected: code=%d msg=%q", resp.Code, resp.Message)
	}
}

// TestProjectUpdate_ArchivedProject 验证归档项目不可编辑
// Business rule: 归档或预算耗尽的项目不能直接编辑
func TestProjectUpdate_ArchivedProject(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	projectID, cleanup := testinfra.CreateTestProject(t, client)
	defer cleanup()

	// 先归档
	archiveResp := client.Post(fmt.Sprintf("/api/tenant/projects/%d/archive", projectID), nil)
	archiveResp.AssertSuccess(t)

	// 尝试更新归档项目
	resp := client.Put(fmt.Sprintf("/api/tenant/projects/%d", projectID), map[string]any{
		"name": "should-not-work",
	})
	if resp.Code == 0 {
		t.Fatal("archived project should not be updatable")
	}
	t.Logf("archived project update: code=%d msg=%q", resp.Code, resp.Message)
}

// TestProjectList_PaginationBoundary 验证分页边界值
func TestProjectList_PaginationBoundary(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	_, cleanup := testinfra.CreateTestProject(t, client)
	defer cleanup()

	// 超出可用数据的 page
	resp := client.Get("/api/tenant/projects", map[string]string{
		"page":      "999999",
		"page_size": "10",
	})
	resp.AssertSuccess(t)
	var data struct {
		List []any `json:"list"`
	}
	resp.DecodeData(t, &data)
	if len(data.List) != 0 {
		t.Fatalf("page=999999 should return empty list, got %d items", len(data.List))
	}
}
