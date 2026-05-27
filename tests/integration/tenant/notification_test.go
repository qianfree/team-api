//go:build integration

package tenant_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestNotificationList(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/notifications", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	resp.AssertSuccess(t)

	var data struct {
		List     []map[string]any `json:"list"`
		Total    int              `json:"total"`
		Page     int              `json:"page"`
		PageSize int              `json:"page_size"`
	}
	resp.DecodeData(t, &data)
}

func TestUnreadCount(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/notifications/unread-count", nil)
	resp.AssertSuccess(t)

	var data struct {
		UnreadCount int `json:"unread_count"`
	}
	resp.DecodeData(t, &data)
}

func TestMarkAllRead(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Post("/api/tenant/notifications/read-all", nil)
	resp.AssertSuccess(t)
}

func TestNotificationPreferences(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	// Get preferences
	getResp := client.Get("/api/tenant/notification-preferences", nil)
	getResp.AssertSuccess(t)

	// Update preferences (user scope)
	updateResp := client.Put("/api/tenant/notification-preferences", map[string]any{
		"scope": "user",
		"preferences": map[string]any{
			"billing": map[string]any{
				"email":  true,
				"in_app": true,
			},
		},
	})
	updateResp.AssertSuccess(t)

	// Verify update
	verifyResp := client.Get("/api/tenant/notification-preferences", nil)
	verifyResp.AssertSuccess(t)
}

func TestNotificationPreferencesOrg(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	// Update preferences (org scope, owner can do this)
	updateResp := client.Put("/api/tenant/notification-preferences", map[string]any{
		"scope": "org",
		"preferences": map[string]any{
			"security": map[string]any{
				"email": true,
			},
		},
	})
	updateResp.AssertSuccess(t)
}

func TestAnnouncements(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/announcements", nil)
	resp.AssertSuccess(t)

	var data struct {
		List []map[string]any `json:"list"`
	}
	resp.DecodeData(t, &data)
}

func TestNotificationsExport(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/notifications/export", map[string]string{
		"format": "csv",
	})
	resp.AssertSuccess(t)
}
