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
