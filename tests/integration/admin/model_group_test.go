//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestModelGroupCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// --- Create ---
	suffix := randomSuffix()
	createResp := client.Post("/api/admin/model-groups", map[string]any{
		"name":        fmt.Sprintf("CRUD测试分组 %s", suffix),
		"code":        fmt.Sprintf("crud-group-%s", suffix),
		"description": "Integration test model group",
	})
	createResp.AssertSuccess(t)
	groupID := createResp.GetID(t)
	defer func() {
		client.Delete(fmt.Sprintf("/api/admin/model-groups/%d", groupID))
	}()

	// --- List ---
	listResp := client.Get("/api/admin/model-groups", map[string]string{
		"page":      "1",
		"page_size": "100",
	})
	testinfra.AssertPaginatedList(t, listResp, 1)

	var listResult struct {
		List []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			Code string `json:"code"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listResult)

	found := false
	for _, g := range listResult.List {
		if g.ID == groupID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created group id=%d not found in list", groupID)
	}

	// --- List with filters ---
	filterResp := client.Get("/api/admin/model-groups", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "active",
	})
	filterResp.AssertSuccess(t)

	searchResp := client.Get("/api/admin/model-groups", map[string]string{
		"page":      "1",
		"page_size": "10",
		"search":    fmt.Sprintf("crud-group-%s", suffix),
	})
	searchResp.AssertSuccess(t)

	// --- Update ---
	updateResp := client.Put(fmt.Sprintf("/api/admin/model-groups/%d", groupID), map[string]any{
		"name":        fmt.Sprintf("更新分组名 %s", suffix),
		"description": "Updated integration test model group",
		"status":      "active",
	})
	updateResp.AssertSuccess(t)

	// --- Delete ---
	deleteResp := client.Delete(fmt.Sprintf("/api/admin/model-groups/%d", groupID))
	deleteResp.AssertSuccess(t)
}

func TestModelGroupModels(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create prerequisites: a group and two models
	groupID, groupCleanup := testinfra.CreateTestModelGroup(t, client)
	defer groupCleanup()

	model1ID, model1Cleanup := testinfra.CreateTestModel(t, client)
	defer model1Cleanup()

	model2ID, model2Cleanup := testinfra.CreateTestModel(t, client)
	defer model2Cleanup()

	// --- Assign models to group ---
	assignResp := client.Put(fmt.Sprintf("/api/admin/model-groups/%d/models", groupID), map[string]any{
		"model_ids": []int64{model1ID, model2ID},
	})
	assignResp.AssertSuccess(t)

	// --- Get models in group ---
	getModelsResp := client.Get(fmt.Sprintf("/api/admin/model-groups/%d/models", groupID), nil)
	getModelsResp.AssertSuccess(t)

	var modelsResult struct {
		List []struct {
			ModelID   string `json:"model_id"`
			ModelName string `json:"model_name"`
			Category  string `json:"category"`
			Status    string `json:"status"`
		} `json:"list"`
	}
	getModelsResp.DecodeData(t, &modelsResult)
	if len(modelsResult.List) < 2 {
		t.Fatalf("expected at least 2 models in group, got %d", len(modelsResult.List))
	}

	// --- Replace models (only model1) ---
	replaceResp := client.Put(fmt.Sprintf("/api/admin/model-groups/%d/models", groupID), map[string]any{
		"model_ids": []int64{model1ID},
	})
	replaceResp.AssertSuccess(t)

	// Verify only model1 remains
	verifyResp := client.Get(fmt.Sprintf("/api/admin/model-groups/%d/models", groupID), nil)
	verifyResp.AssertSuccess(t)

	var verifyResult struct {
		List []struct {
			ModelID string `json:"model_id"`
		} `json:"list"`
	}
	verifyResp.DecodeData(t, &verifyResult)
	if len(verifyResult.List) != 1 {
		t.Fatalf("expected 1 model after replacement, got %d", len(verifyResult.List))
	}
}

func TestModelGroupOptions(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	_, cleanup := testinfra.CreateTestModelGroup(t, client)
	defer cleanup()

	resp := client.Get("/api/admin/model-groups/options", nil)
	resp.AssertSuccess(t)

	var result struct {
		List []struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			Code       string `json:"code"`
			ModelCount int    `json:"model_count"`
		} `json:"list"`
	}
	resp.DecodeData(t, &result)
	if len(result.List) < 1 {
		t.Fatal("expected at least 1 model group option")
	}
}

func TestTenantGroupAssignment(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create prerequisites
	tenantID, tenantCleanup := testinfra.CreateTestTenant(t, client)
	defer tenantCleanup()

	group1ID, group1Cleanup := testinfra.CreateTestModelGroup(t, client)
	defer group1Cleanup()

	group2ID, group2Cleanup := testinfra.CreateTestModelGroup(t, client)
	defer group2Cleanup()

	// --- Assign groups to tenant ---
	assignResp := client.Put(fmt.Sprintf("/api/admin/tenants/%d/groups", tenantID), map[string]any{
		"group_ids": []int64{group1ID, group2ID},
	})
	assignResp.AssertSuccess(t)

	// --- Get tenant groups ---
	getGroupsResp := client.Get(fmt.Sprintf("/api/admin/tenants/%d/groups", tenantID), nil)
	getGroupsResp.AssertSuccess(t)

	var groupsResult struct {
		List []struct {
			GroupID    int64  `json:"group_id"`
			Name       string `json:"name"`
			Code       string `json:"code"`
			Status     string `json:"status"`
			ModelCount int    `json:"model_count"`
		} `json:"list"`
	}
	getGroupsResp.DecodeData(t, &groupsResult)
	if len(groupsResult.List) != 2 {
		t.Fatalf("expected 2 groups assigned to tenant, got %d", len(groupsResult.List))
	}

	// --- Replace with single group ---
	replaceResp := client.Put(fmt.Sprintf("/api/admin/tenants/%d/groups", tenantID), map[string]any{
		"group_ids": []int64{group1ID},
	})
	replaceResp.AssertSuccess(t)

	// Verify only one group remains
	verifyResp := client.Get(fmt.Sprintf("/api/admin/tenants/%d/groups", tenantID), nil)
	verifyResp.AssertSuccess(t)

	var verifyResult struct {
		List []struct {
			GroupID int64 `json:"group_id"`
		} `json:"list"`
	}
	verifyResp.DecodeData(t, &verifyResult)
	if len(verifyResult.List) != 1 {
		t.Fatalf("expected 1 group after replacement, got %d", len(verifyResult.List))
	}
	if verifyResult.List[0].GroupID != group1ID {
		t.Fatalf("expected group_id=%d, got %d", group1ID, verifyResult.List[0].GroupID)
	}
}

// ==================== 边界 / 错误场景 ====================

func TestModelGroupDuplicateCode(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	groupID, cleanup := testinfra.CreateTestModelGroup(t, client)
	defer cleanup()

	// 获取已创建分组的 code
	listResp := client.Get("/api/admin/model-groups", map[string]string{
		"page": "1", "page_size": "100",
	})
	listResp.AssertSuccess(t)

	var listResult struct {
		List []struct {
			ID   int64  `json:"id"`
			Code string `json:"code"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listResult)

	var existingCode string
	for _, g := range listResult.List {
		if g.ID == groupID {
			existingCode = g.Code
			break
		}
	}
	if existingCode == "" {
		t.Fatal("could not find the created group's code")
	}

	// 用相同 code 创建，应返回 10085
	dupResp := client.Post("/api/admin/model-groups", map[string]any{
		"name": "Duplicate Code Test",
		"code": existingCode,
	})
	dupResp.AssertError(t, 10085)
}

func TestModelGroupDeleteWithTenants(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	groupID, groupCleanup := testinfra.CreateTestModelGroup(t, client)
	defer groupCleanup()

	tenantID, tenantCleanup := testinfra.CreateTestTenant(t, client)
	defer tenantCleanup()

	// 将分组关联到租户
	client.Put(fmt.Sprintf("/api/admin/tenants/%d/groups", tenantID), map[string]any{
		"group_ids": []int64{groupID},
	}).AssertSuccess(t)
	defer func() {
		// 清理关联以便分组能被 fixture cleanup 删除
		client.Put(fmt.Sprintf("/api/admin/tenants/%d/groups", tenantID), map[string]any{
			"group_ids": []int64{},
		})
	}()

	// 删除有租户关联的分组，应返回 10086
	delResp := client.Delete(fmt.Sprintf("/api/admin/model-groups/%d", groupID))
	delResp.AssertError(t, 10086)
}

func TestModelGroupUpdateNotFound(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Put("/api/admin/model-groups/999999999", map[string]any{
		"name": "ghost",
	})
	resp.AssertError(t, 10084)
}

func TestModelGroupDeleteNotFound(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Delete("/api/admin/model-groups/999999999")
	resp.AssertError(t, 10084)
}

func TestModelGroupCreateWithModels(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	model1ID, model1Cleanup := testinfra.CreateTestModel(t, client)
	defer model1Cleanup()

	model2ID, model2Cleanup := testinfra.CreateTestModel(t, client)
	defer model2Cleanup()

	suffix := randomSuffix()
	createResp := client.Post("/api/admin/model-groups", map[string]any{
		"name":      fmt.Sprintf("InitModels %s", suffix),
		"code":      fmt.Sprintf("init-models-%s", suffix),
		"model_ids": []int64{model1ID, model2ID},
	})
	createResp.AssertSuccess(t)
	groupID := createResp.GetID(t)
	defer func() {
		client.Delete(fmt.Sprintf("/api/admin/model-groups/%d", groupID))
	}()

	// 验证分组内包含 2 个模型
	modelsResp := client.Get(fmt.Sprintf("/api/admin/model-groups/%d/models", groupID), nil)
	modelsResp.AssertSuccess(t)

	var result struct {
		List []struct {
			ModelId string `json:"model_id"`
		} `json:"list"`
	}
	modelsResp.DecodeData(t, &result)
	if len(result.List) != 2 {
		t.Fatalf("expected 2 models after create-with-models, got %d", len(result.List))
	}
}

func TestModelGroupSetEmptyModels(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	groupID, groupCleanup := testinfra.CreateTestModelGroup(t, client)
	defer groupCleanup()

	modelID, modelCleanup := testinfra.CreateTestModel(t, client)
	defer modelCleanup()

	// 先添加一个模型
	client.Put(fmt.Sprintf("/api/admin/model-groups/%d/models", groupID), map[string]any{
		"model_ids": []int64{modelID},
	}).AssertSuccess(t)

	// 设置为空数组
	client.Put(fmt.Sprintf("/api/admin/model-groups/%d/models", groupID), map[string]any{
		"model_ids": []int64{},
	}).AssertSuccess(t)

	// 验证已清空
	resp := client.Get(fmt.Sprintf("/api/admin/model-groups/%d/models", groupID), nil)
	resp.AssertSuccess(t)

	var result struct {
		List []struct {
			ModelId string `json:"model_id"`
		} `json:"list"`
	}
	resp.DecodeData(t, &result)
	if len(result.List) != 0 {
		t.Fatalf("expected 0 models after clearing, got %d", len(result.List))
	}
}

func TestTenantGroupSetEmpty(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, tenantCleanup := testinfra.CreateTestTenant(t, client)
	defer tenantCleanup()

	groupID, groupCleanup := testinfra.CreateTestModelGroup(t, client)
	defer groupCleanup()

	// 先关联一个分组
	client.Put(fmt.Sprintf("/api/admin/tenants/%d/groups", tenantID), map[string]any{
		"group_ids": []int64{groupID},
	}).AssertSuccess(t)

	// 设置为空数组
	client.Put(fmt.Sprintf("/api/admin/tenants/%d/groups", tenantID), map[string]any{
		"group_ids": []int64{},
	}).AssertSuccess(t)

	// 验证已清空
	resp := client.Get(fmt.Sprintf("/api/admin/tenants/%d/groups", tenantID), nil)
	resp.AssertSuccess(t)

	var result struct {
		List []struct {
			GroupID int64 `json:"group_id"`
		} `json:"list"`
	}
	resp.DecodeData(t, &result)
	if len(result.List) != 0 {
		t.Fatalf("expected 0 groups after clearing, got %d", len(result.List))
	}
}
