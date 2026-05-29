//go:build integration

package testinfra

import (
	"fmt"
	"testing"

	admintest "github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func CreateTestTenantMember(t *testing.T, client *admintest.APIClient) (id int64, cleanup func()) {
	t.Helper()
	suffix := RandomSuffix()
	username := fmt.Sprintf("member%s", suffix)
	resp := client.Post("/api/tenant/members/create", map[string]any{
		"username": username,
		"password": TestPassword,
		"email":    fmt.Sprintf("%s@test.com", username),
		"role":     "member",
	})
	resp.AssertSuccess(t)
	id = resp.GetID(t)
	return id, func() {
		client.Delete(fmt.Sprintf("/api/tenant/members/%d", id))
	}
}

func CreateTestApiKey(t *testing.T, client *admintest.APIClient) (id int64, cleanup func()) {
	t.Helper()
	suffix := RandomSuffix()
	resp := client.Post("/api/tenant/api-keys", map[string]any{
		"name": fmt.Sprintf("test-key-%s", suffix),
	})
	resp.AssertSuccess(t)
	id = resp.GetID(t)
	return id, func() {
		client.Delete(fmt.Sprintf("/api/tenant/api-keys/%d", id))
	}
}

// CreateTestApiKeyWithSecret creates an API key and returns the raw key value alongside the ID.
// The raw key (sk-xxx) is needed for making actual relay requests in E2E tests.
func CreateTestApiKeyWithSecret(t *testing.T, client *admintest.APIClient) (id int64, rawKey string, cleanup func()) {
	t.Helper()
	suffix := RandomSuffix()
	resp := client.Post("/api/tenant/api-keys", map[string]any{
		"name": fmt.Sprintf("test-key-%s", suffix),
	})
	resp.AssertSuccess(t)
	id = resp.GetID(t)

	// Extract the raw key from the response
	var data struct {
		ID  int64  `json:"id"`
		Key string `json:"key"`
	}
	resp.DecodeData(t, &data)
	rawKey = data.Key

	if rawKey == "" {
		t.Fatal("API key creation returned empty key value")
	}

	return id, rawKey, func() {
		client.Delete(fmt.Sprintf("/api/tenant/api-keys/%d", id))
	}
}

func CreateTestProject(t *testing.T, client *admintest.APIClient) (id int64, cleanup func()) {
	t.Helper()
	suffix := RandomSuffix()
	resp := client.Post("/api/tenant/projects", map[string]any{
		"name": fmt.Sprintf("test-project-%s", suffix),
	})
	resp.AssertSuccess(t)
	id = resp.GetID(t)
	return id, func() {
		client.Post(fmt.Sprintf("/api/tenant/projects/%d/archive", id), nil)
	}
}

func CreateTestFeedback(t *testing.T, client *admintest.APIClient) (id int64, cleanup func()) {
	t.Helper()
	suffix := RandomSuffix()
	resp := client.Post("/api/tenant/feedbacks", map[string]any{
		"category":    "bug_report",
		"title":       fmt.Sprintf("[集成测试] 反馈 %s", suffix),
		"description": "集成测试自动创建的反馈",
	})
	resp.AssertSuccess(t)
	id = resp.GetID(t)
	return id, func() {
		HardDeleteFeedback(t, id)
	}
}
