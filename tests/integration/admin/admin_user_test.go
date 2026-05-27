//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestAdminUserList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/users", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 1)
}

func TestAdminUserCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create
	userID, cleanup := testinfra.CreateTestAdminUser(t, client)
	defer cleanup()

	t.Logf("created admin user id=%d", userID)

	// Read — find the created user via list with keyword filter
	listResp := client.Get("/api/admin/users", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			ID          int64  `json:"id"`
			Username    string `json:"username"`
			Email       string `json:"email"`
			DisplayName string `json:"display_name"`
			Role        string `json:"role"`
			Status      string `json:"status"`
		} `json:"list"`
		Total int `json:"total"`
	}
	listResp.DecodeData(t, &listData)

	found := false
	for _, u := range listData.List {
		if u.ID == userID {
			found = true
			if u.Role != "admin" {
				t.Fatalf("expected role=admin, got %s", u.Role)
			}
			break
		}
	}
	if !found {
		t.Fatalf("created user id=%d not found in list", userID)
	}

	// Update
	updateResp := client.Put(fmt.Sprintf("/api/admin/users/%d", userID), map[string]any{
		"display_name": "Updated Name",
		"email":        fmt.Sprintf("updated-%d@test.com", userID),
		"role":         "admin",
	})
	updateResp.AssertSuccess(t)

	// Verify update
	listResp2 := client.Get("/api/admin/users", map[string]string{
		"page":      "1",
		"page_size": "100",
	})
	listResp2.AssertSuccess(t)
	listResp2.DecodeData(t, &listData)

	for _, u := range listData.List {
		if u.ID == userID {
			if u.DisplayName != "Updated Name" {
				t.Fatalf("expected display_name=Updated Name, got %s", u.DisplayName)
			}
			break
		}
	}

	// Delete
	deleteResp := client.Delete(fmt.Sprintf("/api/admin/users/%d", userID))
	deleteResp.AssertSuccess(t)

	// Verify deletion — cleanup is already called but should be a no-op now
	// Prevent double-delete in cleanup
	cleanup = func() {}
}

func TestAdminUserStatus(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	userID, cleanup := testinfra.CreateTestAdminUser(t, client)
	defer cleanup()

	// Disable user
	disableResp := client.Put(fmt.Sprintf("/api/admin/users/%d/status", userID), map[string]any{
		"status": "disabled",
	})
	disableResp.AssertSuccess(t)

	// Verify disabled status via list
	listResp := client.Get("/api/admin/users", map[string]string{
		"page":      "1",
		"page_size": "100",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	for _, u := range listData.List {
		if u.ID == userID {
			if u.Status != "disabled" {
				t.Fatalf("expected disabled status, got %s", u.Status)
			}
			break
		}
	}

	// Re-enable user
	enableResp := client.Put(fmt.Sprintf("/api/admin/users/%d/status", userID), map[string]any{
		"status": "active",
	})
	enableResp.AssertSuccess(t)

	// Verify re-enabled status
	listResp2 := client.Get("/api/admin/users", map[string]string{
		"page":      "1",
		"page_size": "100",
	})
	listResp2.AssertSuccess(t)
	listResp2.DecodeData(t, &listData)

	for _, u := range listData.List {
		if u.ID == userID {
			if u.Status != "active" {
				t.Fatalf("expected active status, got %s", u.Status)
			}
			break
		}
	}
}

func TestAdminUserResetPassword(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	userID, cleanup := testinfra.CreateTestAdminUser(t, client)
	defer cleanup()

	// Reset password
	newPassword := "NewPassword456!"
	resetResp := client.Put(fmt.Sprintf("/api/admin/users/%d/reset-password", userID), map[string]any{
		"new_password": newPassword,
	})
	resetResp.AssertSuccess(t)

	// Verify the new password works by logging in as that user
	// Note: We need the username to login. Retrieve it from the list.
	listResp := client.Get("/api/admin/users", map[string]string{
		"page":      "1",
		"page_size": "100",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	var username string
	for _, u := range listData.List {
		if u.ID == userID {
			username = u.Username
			break
		}
	}
	if username == "" {
		t.Fatalf("could not find username for user id=%d", userID)
	}

	// Authenticate with the new password
	loginResult := testinfra.AuthenticateWithCreds(t, username, newPassword)
	if loginResult.AccessToken == "" {
		t.Fatal("expected login with new password to succeed")
	}
}
