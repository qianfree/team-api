//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestTemplateList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/notification/templates", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestTemplateGetAndUpdate(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// First list to find a template code
	listResp := client.Get("/api/admin/notification/templates", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			Code string `json:"code"`
		} `json:"list"`
		Total int `json:"total"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) == 0 {
		t.Skip("No notification templates found, skipping get/update test")
	}

	templateCode := listData.List[0].Code

	// --- Get template detail ---
	getResp := client.Get(fmt.Sprintf("/api/admin/notification/templates/%s", templateCode), nil)
	getResp.AssertSuccess(t)

	var template struct {
		Code         string `json:"code"`
		Subject      string `json:"subject"`
		BodyTemplate string `json:"body_template"`
		Channel      string `json:"channel"`
	}
	getResp.DecodeData(t, &template)

	t.Logf("Got template: code=%s, channel=%s", template.Code, template.Channel)

	// --- Update with same values (non-destructive) ---
	updateResp := client.Put(fmt.Sprintf("/api/admin/notification/templates/%s", templateCode), map[string]any{
		"subject":       template.Subject,
		"body_template": template.BodyTemplate,
		"channel":       template.Channel,
	})
	updateResp.AssertSuccess(t)

	t.Logf("Template %s updated successfully", templateCode)
}

func TestMessageList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/notification/messages", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestMessageSend(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create a tenant to send message to
	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	resp := client.Post("/api/admin/notification/messages/send", map[string]any{
		"tenant_id": tenantID,
		"title":     fmt.Sprintf("测试通知消息 %s", randomSuffix()),
		"content":   "这是一条集成测试通知消息",
		"channel":   "system",
	})
	resp.AssertSuccess(t)

	t.Logf("Message sent to tenant %d", tenantID)
}

func TestMessageBroadcast(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Post("/api/admin/notification/messages/broadcast", map[string]any{
		"title":        fmt.Sprintf("测试广播消息 %s", randomSuffix()),
		"content":      "这是一条集成测试广播消息",
		"target_roles": []string{"owner", "admin"},
	})
	resp.AssertSuccess(t)

	t.Logf("Broadcast message sent successfully")
}

func TestAnnouncementCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	suffix := randomSuffix()

	// --- Create ---
	createResp := client.Post("/api/admin/announcements", map[string]any{
		"title":            fmt.Sprintf("测试公告 %s", suffix),
		"type":             "system",
		"content":          "这是一条集成测试公告内容",
		"status":           "draft",
		"is_pinned":        false,
		"display_position": "dashboard",
	})
	createResp.AssertSuccess(t)
	announcementID := createResp.GetID(t)
	defer func() {
		client.Delete(fmt.Sprintf("/api/admin/announcements/%d", announcementID))
	}()

	t.Logf("Created announcement: id=%d", announcementID)

	// --- List should contain the announcement ---
	listResp := client.Get("/api/admin/announcements", map[string]string{
		"page":      "1",
		"page_size": "50",
		"status":    "draft",
	})
	testinfra.AssertPaginatedList(t, listResp, 1)

	// --- Update ---
	updateResp := client.Put(fmt.Sprintf("/api/admin/announcements/%d", announcementID), map[string]any{
		"title":            fmt.Sprintf("更新公告 %s", suffix),
		"type":             "system",
		"content":          "更新后的公告内容",
		"status":           "draft",
		"is_pinned":        true,
		"display_position": "dashboard",
	})
	updateResp.AssertSuccess(t)

	t.Logf("Updated announcement %d", announcementID)

	// --- Publish ---
	publishResp := client.Put(fmt.Sprintf("/api/admin/announcements/%d/publish", announcementID), nil)
	publishResp.AssertSuccess(t)

	t.Logf("Published announcement %d", announcementID)

	// --- Archive ---
	archiveResp := client.Put(fmt.Sprintf("/api/admin/announcements/%d/archive", announcementID), nil)
	archiveResp.AssertSuccess(t)

	t.Logf("Archived announcement %d", announcementID)
}
