//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestTenantModelAssign(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, tenantCleanup := testinfra.CreateTestTenant(t, client)
	defer tenantCleanup()

	model1ID, model1Cleanup := testinfra.CreateTestModel(t, client)
	defer model1Cleanup()

	model2ID, model2Cleanup := testinfra.CreateTestModel(t, client)
	defer model2Cleanup()

	// --- Assign models to tenant ---
	assignResp := client.Post(fmt.Sprintf("/api/admin/tenants/%d/models", tenantID), map[string]any{
		"assignments": []map[string]any{
			{
				"model_id":     model1ID,
				"enabled":      true,
				"billing_mode": "token",
			},
			{
				"model_id":     model2ID,
				"enabled":      true,
				"billing_mode": "token",
			},
		},
	})
	assignResp.AssertSuccess(t)

	var assignResult struct {
		Assigned int `json:"assigned"`
	}
	assignResp.DecodeData(t, &assignResult)
	if assignResult.Assigned != 2 {
		t.Fatalf("expected assigned=2, got %d", assignResult.Assigned)
	}

	// --- Verify via list ---
	listResp := client.Get(fmt.Sprintf("/api/admin/tenants/%d/models", tenantID), nil)
	listResp.AssertSuccess(t)

	var listResult struct {
		List []struct {
			ID        int64  `json:"id"`
			TenantID  int64  `json:"tenant_id"`
			ModelID   int64  `json:"model_id"`
			ModelCode string `json:"model_code"`
			ModelName string `json:"model_name"`
			Category  string `json:"category"`
			Enabled   bool   `json:"enabled"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listResult)
	if len(listResult.List) < 2 {
		t.Fatalf("expected at least 2 tenant models, got %d", len(listResult.List))
	}
}

func TestTenantModelList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, tenantCleanup := testinfra.CreateTestTenant(t, client)
	defer tenantCleanup()

	modelID, modelCleanup := testinfra.CreateTestModel(t, client)
	defer modelCleanup()

	// Assign a model first
	assignResp := client.Post(fmt.Sprintf("/api/admin/tenants/%d/models", tenantID), map[string]any{
		"assignments": []map[string]any{
			{
				"model_id":     modelID,
				"enabled":      true,
				"billing_mode": "token",
			},
		},
	})
	assignResp.AssertSuccess(t)

	// List tenant models
	listResp := client.Get(fmt.Sprintf("/api/admin/tenants/%d/models", tenantID), nil)
	listResp.AssertSuccess(t)

	var result struct {
		List []struct {
			ModelID   int64  `json:"model_id"`
			ModelName string `json:"model_name"`
			Category  string `json:"category"`
			Enabled   bool   `json:"enabled"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &result)

	found := false
	for _, m := range result.List {
		if m.ModelID == modelID {
			found = true
			if !m.Enabled {
				t.Fatal("expected model to be enabled")
			}
			break
		}
	}
	if !found {
		t.Fatalf("assigned model id=%d not found in tenant model list", modelID)
	}
}

func TestTenantModelUpdate(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, tenantCleanup := testinfra.CreateTestTenant(t, client)
	defer tenantCleanup()

	modelID, modelCleanup := testinfra.CreateTestModel(t, client)
	defer modelCleanup()

	// Assign model
	assignResp := client.Post(fmt.Sprintf("/api/admin/tenants/%d/models", tenantID), map[string]any{
		"assignments": []map[string]any{
			{
				"model_id":     modelID,
				"enabled":      true,
				"billing_mode": "token",
			},
		},
	})
	assignResp.AssertSuccess(t)

	// --- Update tenant model ---
	updateResp := client.Put(fmt.Sprintf("/api/admin/tenants/%d/models/%d", tenantID, modelID), map[string]any{
		"enabled":      false,
		"billing_mode": "per_request",
	})
	updateResp.AssertSuccess(t)

	// Verify update
	listResp := client.Get(fmt.Sprintf("/api/admin/tenants/%d/models", tenantID), nil)
	listResp.AssertSuccess(t)

	var result struct {
		List []struct {
			ModelID     int64  `json:"model_id"`
			Enabled     bool   `json:"enabled"`
			BillingMode string `json:"billing_mode"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &result)

	for _, m := range result.List {
		if m.ModelID == modelID {
			if m.Enabled {
				t.Fatal("expected model to be disabled after update")
			}
			if m.BillingMode != "per_request" {
				t.Fatalf("expected billing_mode=per_request, got %s", m.BillingMode)
			}
			return
		}
	}
	t.Fatalf("model id=%d not found after update", modelID)
}

func TestTenantModelDelete(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, tenantCleanup := testinfra.CreateTestTenant(t, client)
	defer tenantCleanup()

	modelID, modelCleanup := testinfra.CreateTestModel(t, client)
	defer modelCleanup()

	// Assign model
	assignResp := client.Post(fmt.Sprintf("/api/admin/tenants/%d/models", tenantID), map[string]any{
		"assignments": []map[string]any{
			{
				"model_id":     modelID,
				"enabled":      true,
				"billing_mode": "token",
			},
		},
	})
	assignResp.AssertSuccess(t)

	// --- Delete tenant model ---
	deleteResp := client.Delete(fmt.Sprintf("/api/admin/tenants/%d/models/%d", tenantID, modelID))
	deleteResp.AssertSuccess(t)

	// Verify deletion
	listResp := client.Get(fmt.Sprintf("/api/admin/tenants/%d/models", tenantID), nil)
	listResp.AssertSuccess(t)

	var result struct {
		List []struct {
			ModelID int64 `json:"model_id"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &result)

	for _, m := range result.List {
		if m.ModelID == modelID {
			t.Fatalf("model id=%d should have been deleted from tenant", modelID)
		}
	}
}
