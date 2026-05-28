//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestHelpCategoryList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	_, cleanup := createTestHelpCategory(t, client)
	defer cleanup()

	resp := client.Get("/api/admin/help-categories", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 1)
}

func TestHelpCategoryCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	suffix := randomSuffix()
	originalName := fmt.Sprintf("CRUD分类 %s", suffix)

	// --- Create ---
	createResp := client.Post("/api/admin/help-categories", map[string]any{
		"name":        originalName,
		"slug":        fmt.Sprintf("test-category-%s", suffix),
		"description": "集成测试帮助分类",
		"sort_order":  100,
		"is_visible":  true,
	})
	createResp.AssertSuccess(t)
	categoryID := createResp.GetID(t)
	defer func() {
		client.Delete(fmt.Sprintf("/api/admin/help-categories/%d", categoryID))
	}()

	// --- Verify creation via list ---
	listResp := client.Get("/api/admin/help-categories", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	testinfra.AssertPaginatedList(t, listResp, 1)

	var listData struct {
		List []struct {
			Id          int64  `json:"id"`
			Name        string `json:"name"`
			Slug        string `json:"slug"`
			Description string `json:"description"`
			SortOrder   int    `json:"sort_order"`
			IsVisible   bool   `json:"is_visible"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	var created *struct {
		Id          int64  `json:"id"`
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
		SortOrder   int    `json:"sort_order"`
		IsVisible   bool   `json:"is_visible"`
	}
	for i := range listData.List {
		if listData.List[i].Id == categoryID {
			created = &listData.List[i]
			break
		}
	}
	if created == nil {
		t.Fatalf("category id=%d not found in list after creation", categoryID)
	}
	if created.Name != originalName {
		t.Fatalf("expected name=%q, got %q", originalName, created.Name)
	}
	if created.SortOrder != 100 {
		t.Fatalf("expected sort_order=100, got %d", created.SortOrder)
	}
	if !created.IsVisible {
		t.Fatal("expected is_visible=true")
	}

	// --- Update ---
	updatedName := fmt.Sprintf("更新分类 %s", suffix)
	updateResp := client.Put(fmt.Sprintf("/api/admin/help-categories/%d", categoryID), map[string]any{
		"name":        updatedName,
		"slug":        fmt.Sprintf("updated-category-%s", suffix),
		"description": "更新后的帮助分类描述",
		"sort_order":  200,
		"is_visible":  false,
	})
	updateResp.AssertSuccess(t)

	// --- Verify update via list ---
	verifyResp := client.Get("/api/admin/help-categories", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	verifyResp.AssertSuccess(t)

	var verifyData struct {
		List []struct {
			Id          int64  `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			SortOrder   int    `json:"sort_order"`
			IsVisible   bool   `json:"is_visible"`
		} `json:"list"`
	}
	verifyResp.DecodeData(t, &verifyData)

	var updated *struct {
		Id          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		SortOrder   int    `json:"sort_order"`
		IsVisible   bool   `json:"is_visible"`
	}
	for i := range verifyData.List {
		if verifyData.List[i].Id == categoryID {
			updated = &verifyData.List[i]
			break
		}
	}
	if updated == nil {
		t.Fatalf("category id=%d not found after update", categoryID)
	}
	if updated.Name != updatedName {
		t.Fatalf("expected updated name=%q, got %q", updatedName, updated.Name)
	}
	if updated.SortOrder != 200 {
		t.Fatalf("expected sort_order=200 after update, got %d", updated.SortOrder)
	}
	if updated.IsVisible {
		t.Fatal("expected is_visible=false after update")
	}

	// --- Delete ---
	deleteResp := client.Delete(fmt.Sprintf("/api/admin/help-categories/%d", categoryID))
	deleteResp.AssertSuccess(t)

	// --- Verify deletion ---
	afterDeleteResp := client.Get("/api/admin/help-categories", map[string]string{
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
		if item.Id == categoryID {
			t.Fatal("deleted category should not appear in list")
		}
	}
}

func TestHelpArticleList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	_, catCleanup := createTestHelpCategory(t, client)
	defer catCleanup()

	// List articles (may be empty if no articles exist)
	resp := client.Get("/api/admin/help-articles", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestHelpArticleCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	suffix := randomSuffix()

	// Create a category first
	catResp := client.Post("/api/admin/help-categories", map[string]any{
		"name":       fmt.Sprintf("文章分类 %s", suffix),
		"slug":       fmt.Sprintf("article-cat-%s", suffix),
		"sort_order": 50,
		"is_visible": true,
	})
	catResp.AssertSuccess(t)
	categoryID := catResp.GetID(t)
	defer func() {
		client.Delete(fmt.Sprintf("/api/admin/help-categories/%d", categoryID))
	}()

	// --- Create article ---
	originalTitle := fmt.Sprintf("CRUD测试文章 %s", suffix)
	createResp := client.Post("/api/admin/help-articles", map[string]any{
		"category_id": categoryID,
		"title":       originalTitle,
		"slug":        fmt.Sprintf("test-article-%s", suffix),
		"content":     "这是集成测试帮助文章的正文内容",
		"summary":     "测试文章摘要",
		"status":      "draft",
		"sort_order":  10,
		"keywords":    []string{"测试", "帮助"},
	})
	createResp.AssertSuccess(t)
	articleID := createResp.GetID(t)
	defer func() {
		client.Delete(fmt.Sprintf("/api/admin/help-articles/%d", articleID))
	}()

	// --- Verify creation via article detail ---
	detailResp := client.Get(fmt.Sprintf("/api/admin/help-articles/%d", articleID), nil)
	detailResp.AssertSuccess(t)

	var detail struct {
		Id         int64    `json:"id"`
		CategoryId int64    `json:"category_id"`
		Title      string   `json:"title"`
		Slug       string   `json:"slug"`
		Content    string   `json:"content"`
		Summary    string   `json:"summary"`
		Status     string   `json:"status"`
		SortOrder  int      `json:"sort_order"`
		Keywords   []string `json:"keywords"`
	}
	detailResp.DecodeData(t, &detail)

	if detail.Id != articleID {
		t.Fatalf("expected id=%d, got %d", articleID, detail.Id)
	}
	if detail.CategoryId != categoryID {
		t.Fatalf("expected category_id=%d, got %d", categoryID, detail.CategoryId)
	}
	if detail.Title != originalTitle {
		t.Fatalf("expected title=%q, got %q", originalTitle, detail.Title)
	}
	if detail.Status != "draft" {
		t.Fatalf("new article should be draft, got %q", detail.Status)
	}
	if detail.SortOrder != 10 {
		t.Fatalf("expected sort_order=10, got %d", detail.SortOrder)
	}
	if len(detail.Keywords) != 2 {
		t.Fatalf("expected 2 keywords, got %d", len(detail.Keywords))
	}

	// --- Verify creation via list filter ---
	listResp := client.Get("/api/admin/help-articles", map[string]string{
		"category_id": fmt.Sprintf("%d", categoryID),
		"page":        "1",
		"page_size":   "50",
	})
	testinfra.AssertPaginatedList(t, listResp, 1)

	var listData struct {
		List []struct {
			Id     int64  `json:"id"`
			Title  string `json:"title"`
			Status string `json:"status"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	found := false
	for _, item := range listData.List {
		if item.Id == articleID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("created article not found in filtered list")
	}

	// --- Update article ---
	updatedTitle := fmt.Sprintf("更新文章 %s", suffix)
	updateResp := client.Put(fmt.Sprintf("/api/admin/help-articles/%d", articleID), map[string]any{
		"category_id": categoryID,
		"title":       updatedTitle,
		"slug":        fmt.Sprintf("updated-article-%s", suffix),
		"content":     "更新后的文章正文",
		"summary":     "更新后的摘要",
		"status":      "published",
		"sort_order":  20,
		"keywords":    []string{"更新", "测试"},
	})
	updateResp.AssertSuccess(t)

	// --- Verify update via detail ---
	verifyResp := client.Get(fmt.Sprintf("/api/admin/help-articles/%d", articleID), nil)
	verifyResp.AssertSuccess(t)

	var verifyDetail struct {
		Title     string   `json:"title"`
		Content   string   `json:"content"`
		Summary   string   `json:"summary"`
		Status    string   `json:"status"`
		SortOrder int      `json:"sort_order"`
		Keywords  []string `json:"keywords"`
	}
	verifyResp.DecodeData(t, &verifyDetail)

	if verifyDetail.Title != updatedTitle {
		t.Fatalf("expected updated title=%q, got %q", updatedTitle, verifyDetail.Title)
	}
	if verifyDetail.Status != "published" {
		t.Fatalf("expected status=published after update, got %q", verifyDetail.Status)
	}
	if verifyDetail.SortOrder != 20 {
		t.Fatalf("expected sort_order=20 after update, got %d", verifyDetail.SortOrder)
	}
	if len(verifyDetail.Keywords) != 2 {
		t.Fatalf("expected 2 keywords after update, got %d", len(verifyDetail.Keywords))
	}

	// --- Verify status filter works ---
	publishedResp := client.Get("/api/admin/help-articles", map[string]string{
		"category_id": fmt.Sprintf("%d", categoryID),
		"status":      "published",
		"page":        "1",
		"page_size":   "50",
	})
	publishedResp.AssertSuccess(t)

	// --- Delete ---
	deleteResp := client.Delete(fmt.Sprintf("/api/admin/help-articles/%d", articleID))
	deleteResp.AssertSuccess(t)

	// --- Verify deletion via list ---
	afterDeleteResp := client.Get("/api/admin/help-articles", map[string]string{
		"category_id": fmt.Sprintf("%d", categoryID),
		"page":        "1",
		"page_size":   "50",
	})
	afterDeleteResp.AssertSuccess(t)

	var afterDeleteData struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
	}
	afterDeleteResp.DecodeData(t, &afterDeleteData)

	for _, item := range afterDeleteData.List {
		if item.Id == articleID {
			t.Fatal("deleted article should not appear in list")
		}
	}
}

func TestHelpCenterNegative(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create category with missing required fields
	missingCatResp := client.Post("/api/admin/help-categories", map[string]any{})
	if missingCatResp.Code == 0 {
		t.Fatal("expected error for missing category fields, got success")
	}

	// Create article with non-existent category
	invalidArticleResp := client.Post("/api/admin/help-articles", map[string]any{
		"category_id": 999999999,
		"title":       "Invalid Article",
		"slug":        "invalid-article",
		"content":     "test",
	})
	if invalidArticleResp.Code == 0 {
		t.Fatal("expected error for non-existent category_id, got success")
	}

	// Get non-existent article detail
	getNonExistResp := client.Get("/api/admin/help-articles/999999999", nil)
	if getNonExistResp.Code == 0 {
		t.Fatal("expected error for non-existent article, got success")
	}

	// Delete non-existent category
	deleteNonExistResp := client.Delete("/api/admin/help-categories/999999999")
	if deleteNonExistResp.Code == 0 {
		t.Fatal("expected error for deleting non-existent category, got success")
	}
}

// createTestHelpCategory is a test helper that creates a help category and returns its ID and cleanup.
func createTestHelpCategory(t *testing.T, client *testinfra.APIClient) (int64, func()) {
	t.Helper()
	suffix := randomSuffix()
	resp := client.Post("/api/admin/help-categories", map[string]any{
		"name":       fmt.Sprintf("测试分类 %s", suffix),
		"slug":       fmt.Sprintf("test-cat-%s", suffix),
		"sort_order": 100,
		"is_visible": true,
	})
	resp.AssertSuccess(t)
	id := resp.GetID(t)
	return id, func() {
		client.Delete(fmt.Sprintf("/api/admin/help-categories/%d", id))
	}
}
