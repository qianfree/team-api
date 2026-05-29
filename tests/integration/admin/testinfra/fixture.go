//go:build integration

package testinfra

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"
)

func randomSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func CreateTestAdminUser(t *testing.T, client *APIClient) (id int64, cleanup func()) {
	t.Helper()
	username := fmt.Sprintf("testuser%s", randomSuffix())
	resp := client.Post("/api/admin/users", map[string]any{
		"username": username,
		"password": DefaultPassword,
		"email":    fmt.Sprintf("%s@test.com", username),
		"role":     "admin",
	})
	resp.AssertSuccess(t)
	id = resp.GetID(t)
	return id, func() {
		client.Delete(fmt.Sprintf("/api/admin/users/%d", id))
	}
}

func CreateTestTenant(t *testing.T, client *APIClient) (id int64, cleanup func()) {
	t.Helper()
	code := fmt.Sprintf("test-tenant-%s", randomSuffix())
	resp := client.Post("/api/admin/tenants", map[string]any{
		"tenant_name": fmt.Sprintf("测试租户 %s", code),
		"tenant_code": code,
		"username":    fmt.Sprintf("owner%s", randomSuffix()),
		"email":       fmt.Sprintf("owner-%s@test.com", code),
		"password":    DefaultPassword,
	})
	resp.AssertSuccess(t)
	id = resp.GetID(t)
	return id, func() {
		HardDeleteTenant(t, id)
	}
}

func CreateTestModel(t *testing.T, client *APIClient) (id int64, cleanup func()) {
	t.Helper()
	modelID := fmt.Sprintf("test-model-%s", randomSuffix())
	resp := client.Post("/api/admin/models", map[string]any{
		"model_id":   modelID,
		"model_name": fmt.Sprintf("Test Model %s", modelID),
		"category":   "chat",
	})
	resp.AssertSuccess(t)
	id = resp.GetID(t)
	return id, func() {
		client.Delete(fmt.Sprintf("/api/admin/models/%d", id))
	}
}

func CreateTestChannel(t *testing.T, client *APIClient) (id int64, cleanup func()) {
	t.Helper()
	name := fmt.Sprintf("Test Channel %s", randomSuffix())
	resp := client.Post("/api/admin/channels", map[string]any{
		"name":    name,
		"type":    1,
		"api_key": "sk-test-key-" + randomSuffix(),
	})
	resp.AssertSuccess(t)
	id = resp.GetID(t)
	return id, func() {
		client.Delete(fmt.Sprintf("/api/admin/channels/%d", id))
	}
}

func CreateTestPlan(t *testing.T, client *APIClient) (id int64, cleanup func()) {
	t.Helper()
	ident := fmt.Sprintf("test-plan-%s", randomSuffix())
	resp := client.Post("/api/admin/plans", map[string]any{
		"name":       fmt.Sprintf("Test Plan %s", ident),
		"identifier": ident,
	})
	resp.AssertSuccess(t)
	id = resp.GetID(t)
	return id, func() {
		client.Delete(fmt.Sprintf("/api/admin/plans/%d", id))
	}
}

func CreateTestModelGroup(t *testing.T, client *APIClient) (id int64, cleanup func()) {
	t.Helper()
	code := fmt.Sprintf("test-group-%s", randomSuffix())
	resp := client.Post("/api/admin/model-groups", map[string]any{
		"name": fmt.Sprintf("Test Group %s", code),
		"code": code,
	})
	resp.AssertSuccess(t)
	id = resp.GetID(t)
	return id, func() {
		client.Delete(fmt.Sprintf("/api/admin/model-groups/%d", id))
	}
}

func CreateTestTenantLevel(t *testing.T, client *APIClient) (id int64, cleanup func()) {
	t.Helper()
	suffix := randomSuffix()
	resp := client.Post("/api/admin/tenant-level-configs", map[string]any{
		"level":                         90 + int(suffix[0]%10),
		"name":                          fmt.Sprintf("测试等级 %s", suffix),
		"cumulative_recharge_threshold": 100.0,
		"max_members":                   50,
		"max_concurrency":               10,
		"price_multiplier":              0.9,
	})
	resp.AssertSuccess(t)
	id = resp.GetID(t)
	return id, func() {
		client.Delete(fmt.Sprintf("/api/admin/tenant-level-configs/%d", id))
	}
}
