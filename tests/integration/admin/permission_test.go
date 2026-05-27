//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestAllPermissions(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/permissions", nil)
	resp.AssertSuccess(t)

	var data struct {
		Groups []struct {
			Name        string `json:"name"`
			Label       string `json:"label"`
			Permissions []any  `json:"permissions"`
		} `json:"groups"`
	}
	resp.DecodeData(t, &data)

	if len(data.Groups) == 0 {
		t.Fatal("expected at least one permission group")
	}

	t.Logf("found %d permission groups", len(data.Groups))
}

func TestPermissionList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	userID, cleanup := testinfra.CreateTestAdminUser(t, client)
	defer cleanup()

	resp := client.Get(fmt.Sprintf("/api/admin/users/%d/permissions", userID), nil)
	resp.AssertSuccess(t)

	var data struct {
		Permissions []string `json:"permissions"`
		DataScopes  []struct {
			ID         int64  `json:"id"`
			ScopeType  string `json:"scope_type"`
			ScopeValue string `json:"scope_value"`
		} `json:"data_scopes"`
	}
	resp.DecodeData(t, &data)

	// A newly created user may have no permissions; just verify the structure
	t.Logf("user %d has %d permissions and %d data scopes", userID, len(data.Permissions), len(data.DataScopes))
}

func TestPermissionUpdate(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	userID, cleanup := testinfra.CreateTestAdminUser(t, client)
	defer cleanup()

	// Update permissions
	perms := []string{"tenant:view", "tenant:create", "model:view"}
	updateResp := client.Put(fmt.Sprintf("/api/admin/users/%d/permissions", userID), map[string]any{
		"permissions": perms,
	})
	updateResp.AssertSuccess(t)

	// Get permissions and verify
	getResp := client.Get(fmt.Sprintf("/api/admin/users/%d/permissions", userID), nil)
	getResp.AssertSuccess(t)

	var data struct {
		Permissions []string `json:"permissions"`
	}
	getResp.DecodeData(t, &data)

	if len(data.Permissions) != len(perms) {
		t.Fatalf("expected %d permissions, got %d", len(perms), len(data.Permissions))
	}

	permMap := make(map[string]bool)
	for _, p := range data.Permissions {
		permMap[p] = true
	}
	for _, expected := range perms {
		if !permMap[expected] {
			t.Fatalf("expected permission %q not found", expected)
		}
	}
}

func TestDataScopeUpdate(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	userID, cleanup := testinfra.CreateTestAdminUser(t, client)
	defer cleanup()

	// Update data scopes
	dataScopes := []map[string]any{
		{"scope_type": "all", "scope_value": ""},
	}
	updateResp := client.Put(fmt.Sprintf("/api/admin/users/%d/data-scopes", userID), map[string]any{
		"data_scopes": dataScopes,
	})
	updateResp.AssertSuccess(t)

	// Get permissions and verify data scopes
	getResp := client.Get(fmt.Sprintf("/api/admin/users/%d/permissions", userID), nil)
	getResp.AssertSuccess(t)

	var data struct {
		DataScopes []struct {
			ID         int64  `json:"id"`
			ScopeType  string `json:"scope_type"`
			ScopeValue string `json:"scope_value"`
		} `json:"data_scopes"`
	}
	getResp.DecodeData(t, &data)

	if len(data.DataScopes) != 1 {
		t.Fatalf("expected 1 data scope, got %d", len(data.DataScopes))
	}

	if data.DataScopes[0].ScopeType != "all" {
		t.Fatalf("expected scope_type=all, got %s", data.DataScopes[0].ScopeType)
	}
}
