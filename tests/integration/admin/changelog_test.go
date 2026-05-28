//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestChangelogList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create one to ensure list is non-empty
	_, cleanup := createTestChangelog(t, client, "feature")
	defer cleanup()

	resp := client.Get("/api/admin/changelogs", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 1)
}

func TestChangelogListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	featureID, featureCleanup := createTestChangelog(t, client, "feature")
	defer featureCleanup()

	fixID, fixCleanup := createTestChangelog(t, client, "fix")
	defer fixCleanup()

	// Filter by type=feature — should include featureID but not fixID
	resp := client.Get("/api/admin/changelogs", map[string]string{
		"page":      "1",
		"page_size": "50",
		"type":      "feature",
	})
	resp.AssertSuccess(t)

	var featureList struct {
		List []struct {
			Id   int64  `json:"id"`
			Type string `json:"type"`
		} `json:"list"`
	}
	resp.DecodeData(t, &featureList)

	foundFeature := false
	for _, item := range featureList.List {
		if item.Id == featureID {
			foundFeature = true
			if item.Type != "feature" {
				t.Fatalf("expected type=feature, got %s", item.Type)
			}
		}
		if item.Id == fixID {
			t.Fatal("fix changelog should not appear when filtering type=feature")
		}
	}
	if !foundFeature {
		t.Fatal("feature changelog not found in filtered list")
	}

	// Filter by status=draft
	draftResp := client.Get("/api/admin/changelogs", map[string]string{
		"page":      "1",
		"page_size": "50",
		"status":    "draft",
	})
	draftResp.AssertSuccess(t)
}

func TestChangelogCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	suffix := randomSuffix()
	originalTitle := fmt.Sprintf("CRUD测试日志 %s", suffix)
	originalVersion := fmt.Sprintf("1.0.%s", suffix)

	// --- Create ---
	createResp := client.Post("/api/admin/changelogs", map[string]any{
		"version": originalVersion,
		"title":   originalTitle,
		"content": "## 新功能\n\n- 集成测试创建的更新日志",
		"type":    "feature",
	})
	createResp.AssertSuccess(t)
	changelogID := createResp.GetID(t)
	defer func() {
		client.Delete(fmt.Sprintf("/api/admin/changelogs/%d", changelogID))
	}()

	// --- Verify creation via list ---
	listResp := client.Get("/api/admin/changelogs", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	testinfra.AssertPaginatedList(t, listResp, 1)

	var listData struct {
		List []struct {
			Id      int64  `json:"id"`
			Version string `json:"version"`
			Title   string `json:"title"`
			Type    string `json:"type"`
			Status  string `json:"status"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	var created *struct {
		Id      int64  `json:"id"`
		Version string `json:"version"`
		Title   string `json:"title"`
		Type    string `json:"type"`
		Status  string `json:"status"`
	}
	for i := range listData.List {
		if listData.List[i].Id == changelogID {
			created = &listData.List[i]
			break
		}
	}
	if created == nil {
		t.Fatalf("changelog id=%d not found in list after creation", changelogID)
	}
	if created.Version != originalVersion {
		t.Fatalf("expected version=%q, got %q", originalVersion, created.Version)
	}
	if created.Title != originalTitle {
		t.Fatalf("expected title=%q, got %q", originalTitle, created.Title)
	}
	if created.Type != "feature" {
		t.Fatalf("expected type=feature, got %q", created.Type)
	}
	if created.Status != "draft" {
		t.Fatalf("new changelog should be draft, got %q", created.Status)
	}

	// --- Update ---
	updatedTitle := fmt.Sprintf("更新日志-已编辑 %s", suffix)
	updateResp := client.Put(fmt.Sprintf("/api/admin/changelogs/%d", changelogID), map[string]any{
		"version": originalVersion,
		"title":   updatedTitle,
		"content": "## 更新内容\n\n- 编辑后的内容",
		"type":    "improvement",
		"status":  "draft",
	})
	updateResp.AssertSuccess(t)

	// --- Verify update via list ---
	verifyResp := client.Get("/api/admin/changelogs", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	verifyResp.AssertSuccess(t)

	var verifyData struct {
		List []struct {
			Id     int64  `json:"id"`
			Title  string `json:"title"`
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"list"`
	}
	verifyResp.DecodeData(t, &verifyData)

	var updated *struct {
		Id     int64  `json:"id"`
		Title  string `json:"title"`
		Type   string `json:"type"`
		Status string `json:"status"`
	}
	for i := range verifyData.List {
		if verifyData.List[i].Id == changelogID {
			updated = &verifyData.List[i]
			break
		}
	}
	if updated == nil {
		t.Fatalf("changelog id=%d not found after update", changelogID)
	}
	if updated.Title != updatedTitle {
		t.Fatalf("expected updated title=%q, got %q", updatedTitle, updated.Title)
	}
	if updated.Type != "improvement" {
		t.Fatalf("expected type=improvement after update, got %q", updated.Type)
	}

	// --- Publish and verify status ---
	publishResp := client.Post(fmt.Sprintf("/api/admin/changelogs/%d/publish", changelogID), nil)
	publishResp.AssertSuccess(t)

	publishedResp := client.Get("/api/admin/changelogs", map[string]string{
		"page":      "1",
		"page_size": "50",
		"status":    "published",
	})
	publishedResp.AssertSuccess(t)

	var pubData struct {
		List []struct {
			Id     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"list"`
	}
	publishedResp.DecodeData(t, &pubData)

	found := false
	for _, item := range pubData.List {
		if item.Id == changelogID {
			found = true
			if item.Status != "published" {
				t.Fatalf("expected status=published after publish, got %q", item.Status)
			}
		}
	}
	if !found {
		t.Fatal("published changelog not found in published list")
	}

	// --- Delete ---
	deleteResp := client.Delete(fmt.Sprintf("/api/admin/changelogs/%d", changelogID))
	deleteResp.AssertSuccess(t)

	// --- Verify deletion via list ---
	afterDeleteResp := client.Get("/api/admin/changelogs", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	afterDeleteResp.AssertSuccess(t)

	var afterDeleteData struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
	}
	afterDeleteResp.DecodeData(t, &afterDeleteData)

	for _, item := range afterDeleteData.List {
		if item.Id == changelogID {
			t.Fatal("deleted changelog should not appear in list")
		}
	}
}

func TestChangelogNegative(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create with invalid type should fail
	invalidTypeResp := client.Post("/api/admin/changelogs", map[string]any{
		"version": "0.0.invalid-type",
		"title":   "Invalid Type Test",
		"content": "test",
		"type":    "invalid_type",
	})
	if invalidTypeResp.Code == 0 {
		t.Fatal("expected error for invalid type, got success")
	}

	// Create with missing required fields should fail
	missingFieldsResp := client.Post("/api/admin/changelogs", map[string]any{
		"type": "feature",
	})
	if missingFieldsResp.Code == 0 {
		t.Fatal("expected error for missing required fields, got success")
	}

	// Delete non-existent changelog should fail
	deleteNonExistResp := client.Delete("/api/admin/changelogs/999999999")
	if deleteNonExistResp.Code == 0 {
		t.Fatal("expected error when deleting non-existent changelog, got success")
	}

	// Publish non-existent changelog should fail
	publishNonExistResp := client.Post("/api/admin/changelogs/999999999/publish", nil)
	if publishNonExistResp.Code == 0 {
		t.Fatal("expected error when publishing non-existent changelog, got success")
	}
}

// createTestChangelog is a test helper that creates a changelog and returns its ID and cleanup function.
func createTestChangelog(t *testing.T, client *testinfra.APIClient, changelogType string) (int64, func()) {
	t.Helper()
	suffix := randomSuffix()
	resp := client.Post("/api/admin/changelogs", map[string]any{
		"version": fmt.Sprintf("0.0.%s", suffix),
		"title":   fmt.Sprintf("测试更新日志 %s", suffix),
		"content": "集成测试创建的更新日志",
		"type":    changelogType,
	})
	resp.AssertSuccess(t)
	id := resp.GetID(t)
	return id, func() {
		client.Delete(fmt.Sprintf("/api/admin/changelogs/%d", id))
	}
}
