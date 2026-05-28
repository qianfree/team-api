//go:build integration

package admin_test

import (
	"encoding/json"
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

	templateCode := getFirstTemplateCode(t, client)
	if templateCode == "" {
		return
	}

	// --- Get template detail ---
	getResp := client.Get(fmt.Sprintf("/api/admin/notification/templates/%s", templateCode), nil)
	getResp.AssertSuccess(t)

	// Response has nested data: {"data": {...}}
	var outerMap map[string]json.RawMessage
	getResp.DecodeData(t, &outerMap)

	var template struct {
		Code         string `json:"code"`
		Subject      string `json:"subject"`
		BodyTemplate string `json:"body_template"`
		Channel      string `json:"channel"`
	}
	if innerData, ok := outerMap["data"]; ok {
		json.Unmarshal(innerData, &template)
	} else {
		var raw json.RawMessage
		raw, _ = json.Marshal(outerMap)
		json.Unmarshal(raw, &template)
	}

	if template.Code != templateCode {
		t.Fatalf("expected code=%q, got %q", templateCode, template.Code)
	}

	// --- Update with modified subject ---
	testSubject := template.Subject + " [测试]"
	updateResp := client.Put(fmt.Sprintf("/api/admin/notification/templates/%s", templateCode), map[string]any{
		"subject":       testSubject,
		"body_template": template.BodyTemplate,
		"channel":       template.Channel,
	})
	updateResp.AssertSuccess(t)

	// --- Verify update ---
	verifyResp := client.Get(fmt.Sprintf("/api/admin/notification/templates/%s", templateCode), nil)
	verifyResp.AssertSuccess(t)

	var verifyOuter map[string]json.RawMessage
	verifyResp.DecodeData(t, &verifyOuter)

	var verifyData struct {
		Subject string `json:"subject"`
	}
	if innerData, ok := verifyOuter["data"]; ok {
		json.Unmarshal(innerData, &verifyData)
	} else {
		var raw json.RawMessage
		raw, _ = json.Marshal(verifyOuter)
		json.Unmarshal(raw, &verifyData)
	}

	if verifyData.Subject != testSubject {
		t.Fatalf("expected subject=%q after update, got %q", testSubject, verifyData.Subject)
	}

	// --- Restore original ---
	restoreResp := client.Put(fmt.Sprintf("/api/admin/notification/templates/%s", templateCode), map[string]any{
		"subject":       template.Subject,
		"body_template": template.BodyTemplate,
		"channel":       template.Channel,
	})
	restoreResp.AssertSuccess(t)
}

func TestMessageList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/notification/messages", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestMessageSendAndVerify(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	// Count messages before sending
	beforeResp := client.Get("/api/admin/notification/messages", map[string]string{
		"tenant_id": fmt.Sprintf("%d", tenantID),
		"page":      "1",
		"page_size": "10",
	})
	beforeResp.AssertSuccess(t)
	beforeTotal := beforeResp.GetTotal(t)

	// Send a message
	msgTitle := fmt.Sprintf("验证测试消息 %s", randomSuffix())
	msgContent := "这是一条用于验证发送成功的测试消息"
	sendResp := client.Post("/api/admin/notification/messages/send", map[string]any{
		"tenant_id": tenantID,
		"title":     msgTitle,
		"content":   msgContent,
		"channel":   "system",
	})
	sendResp.AssertSuccess(t)

	// Verify the message appears in the list for this tenant
	afterResp := client.Get("/api/admin/notification/messages", map[string]string{
		"tenant_id": fmt.Sprintf("%d", tenantID),
		"page":      "1",
		"page_size": "10",
	})
	afterResp.AssertSuccess(t)
	afterTotal := afterResp.GetTotal(t)

	if afterTotal <= beforeTotal {
		t.Fatalf("expected message count to increase after send (before=%d, after=%d)", beforeTotal, afterTotal)
	}

	// Verify the message content
	var msgList struct {
		List []struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		} `json:"list"`
	}
	afterResp.DecodeData(t, &msgList)

	found := false
	for _, msg := range msgList.List {
		if msg.Title == msgTitle && msg.Content == msgContent {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("sent message with title=%q not found in message list", msgTitle)
	}

	t.Logf("Message send verified: tenant %d, before=%d, after=%d", tenantID, beforeTotal, afterTotal)
}

func TestMessageBroadcast(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	msgTitle := fmt.Sprintf("[集成测试] 广播消息 %s", randomSuffix())
	resp := client.Post("/api/admin/notification/messages/broadcast", map[string]any{
		"title":        msgTitle,
		"content":      "这是一条集成测试广播消息",
		"target_roles": []string{"owner", "admin"},
	})
	resp.AssertSuccess(t)

	// Hard-delete broadcast messages from database
	defer testinfra.HardDeleteMessagesByTitle(t, "[集成测试]")
}

func TestAnnouncementCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	suffix := randomSuffix()
	originalTitle := fmt.Sprintf("CRUD测试公告 %s", suffix)

	// --- Create ---
	createResp := client.Post("/api/admin/announcements", map[string]any{
		"title":            originalTitle,
		"type":             "info",
		"content":          "这是一条集成测试公告内容",
		"status":           "draft",
		"is_pinned":        false,
		"display_position": "console",
	})
	createResp.AssertSuccess(t)
	announcementID := createResp.GetID(t)
	defer func() {
		testinfra.HardDeleteAnnouncement(t, announcementID)
	}()

	// --- Verify creation via list ---
	listResp := client.Get("/api/admin/announcements", map[string]string{
		"page":      "1",
		"page_size": "50",
		"status":    "draft",
	})
	testinfra.AssertPaginatedList(t, listResp, 1)

	var listData struct {
		List []struct {
			Id              int64  `json:"id"`
			Title           string `json:"title"`
			Type            string `json:"type"`
			Status          string `json:"status"`
			IsPinned        int    `json:"is_pinned"`
			DisplayPosition string `json:"display_position"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	var created *struct {
		Id              int64  `json:"id"`
		Title           string `json:"title"`
		Type            string `json:"type"`
		Status          string `json:"status"`
		IsPinned        int    `json:"is_pinned"`
		DisplayPosition string `json:"display_position"`
	}
	for i := range listData.List {
		if listData.List[i].Id == announcementID {
			created = &listData.List[i]
			break
		}
	}
	if created == nil {
		t.Fatalf("announcement id=%d not found in draft list", announcementID)
	}
	if created.Title != originalTitle {
		t.Fatalf("expected title=%q, got %q", originalTitle, created.Title)
	}
	if created.Status != "draft" {
		t.Fatalf("new announcement should be draft, got %q", created.Status)
	}

	// --- Update ---
	updatedTitle := fmt.Sprintf("更新公告 %s", suffix)
	updateResp := client.Put(fmt.Sprintf("/api/admin/announcements/%d", announcementID), map[string]any{
		"title":            updatedTitle,
		"type":             "warning",
		"content":          "更新后的公告内容",
		"status":           "draft",
		"is_pinned":        1,
		"display_position": "both",
	})
	updateResp.AssertSuccess(t)

	// --- Verify update via list ---
	verifyResp := client.Get("/api/admin/announcements", map[string]string{
		"page":      "1",
		"page_size": "50",
		"status":    "draft",
	})
	verifyResp.AssertSuccess(t)

	var verifyData struct {
		List []struct {
			Id              int64  `json:"id"`
			Title           string `json:"title"`
			Type            string `json:"type"`
			IsPinned        int    `json:"is_pinned"`
			DisplayPosition string `json:"display_position"`
		} `json:"list"`
	}
	verifyResp.DecodeData(t, &verifyData)

	var updated *struct {
		Id              int64  `json:"id"`
		Title           string `json:"title"`
		Type            string `json:"type"`
		IsPinned        int    `json:"is_pinned"`
		DisplayPosition string `json:"display_position"`
	}
	for i := range verifyData.List {
		if verifyData.List[i].Id == announcementID {
			updated = &verifyData.List[i]
			break
		}
	}
	if updated == nil {
		t.Fatalf("announcement id=%d not found after update", announcementID)
	}
	if updated.Title != updatedTitle {
		t.Fatalf("expected updated title=%q, got %q", updatedTitle, updated.Title)
	}
	if updated.Type != "warning" {
		t.Fatalf("expected type=warning after update, got %q", updated.Type)
	}

	// --- Publish ---
	publishResp := client.Put(fmt.Sprintf("/api/admin/announcements/%d/publish", announcementID), nil)
	publishResp.AssertSuccess(t)

	// --- Verify published status ---
	pubListResp := client.Get("/api/admin/announcements", map[string]string{
		"page":      "1",
		"page_size": "50",
		"status":    "published",
	})
	pubListResp.AssertSuccess(t)

	var pubData struct {
		List []struct {
			Id     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"list"`
	}
	pubListResp.DecodeData(t, &pubData)

	found := false
	for _, item := range pubData.List {
		if item.Id == announcementID {
			found = true
			if item.Status != "published" {
				t.Fatalf("expected status=published after publish, got %q", item.Status)
			}
		}
	}
	if !found {
		t.Fatal("published announcement not found in published list")
	}

	// --- Archive ---
	archiveResp := client.Put(fmt.Sprintf("/api/admin/announcements/%d/archive", announcementID), nil)
	archiveResp.AssertSuccess(t)

	// --- Verify archived status ---
	archListResp := client.Get("/api/admin/announcements", map[string]string{
		"page":      "1",
		"page_size": "50",
		"status":    "archived",
	})
	archListResp.AssertSuccess(t)

	var archData struct {
		List []struct {
			Id     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"list"`
	}
	archListResp.DecodeData(t, &archData)

	foundArchived := false
	for _, item := range archData.List {
		if item.Id == announcementID {
			foundArchived = true
			if item.Status != "archived" {
				t.Fatalf("expected status=archived after archive, got %q", item.Status)
			}
		}
	}
	if !foundArchived {
		t.Fatal("archived announcement not found in archived list")
	}
}

func TestAnnouncementNegative(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create with missing title should fail
	missingTitleResp := client.Post("/api/admin/announcements", map[string]any{
		"content": "no title",
	})
	if missingTitleResp.Code == 0 {
		t.Fatal("expected error for missing announcement title, got success")
	}

	// Publish non-existent announcement
	publishNonExistResp := client.Put("/api/admin/announcements/999999999/publish", nil)
	if publishNonExistResp.Code == 0 {
		t.Fatal("expected error when publishing non-existent announcement, got success")
	}
}

// getFirstTemplateCode returns the code of the first notification template, or empty string if none.
func getFirstTemplateCode(t *testing.T, client *testinfra.APIClient) string {
	t.Helper()
	listResp := client.Get("/api/admin/notification/templates", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			Code string `json:"code"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) == 0 {
		t.Skip("No notification templates found")
	}
	return listData.List[0].Code
}
