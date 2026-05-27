//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestHelpCategoryCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	suffix := randomSuffix()

	// --- Create ---
	createResp := client.Post("/api/admin/help-categories", map[string]any{
		"name":        fmt.Sprintf("测试帮助分类 %s", suffix),
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

	t.Logf("Created help category: id=%d", categoryID)

	// --- List should contain the category ---
	listResp := client.Get("/api/admin/help-categories", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	testinfra.AssertPaginatedList(t, listResp, 1)

	// --- Update ---
	updateResp := client.Put(fmt.Sprintf("/api/admin/help-categories/%d", categoryID), map[string]any{
		"name":        fmt.Sprintf("更新帮助分类 %s", suffix),
		"slug":        fmt.Sprintf("updated-category-%s", suffix),
		"description": "更新后的帮助分类描述",
		"sort_order":  200,
		"is_visible":  true,
	})
	updateResp.AssertSuccess(t)

	t.Logf("Updated help category %d", categoryID)

	// --- Delete ---
	deleteResp := client.Delete(fmt.Sprintf("/api/admin/help-categories/%d", categoryID))
	deleteResp.AssertSuccess(t)

	t.Logf("Deleted help category %d", categoryID)
}

func TestHelpArticleCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	suffix := randomSuffix()

	// Create a category first
	catResp := client.Post("/api/admin/help-categories", map[string]any{
		"name":        fmt.Sprintf("文章分类 %s", suffix),
		"slug":        fmt.Sprintf("article-cat-%s", suffix),
		"description": "用于文章测试的分类",
		"sort_order":  50,
		"is_visible":  true,
	})
	catResp.AssertSuccess(t)
	categoryID := catResp.GetID(t)
	defer func() {
		client.Delete(fmt.Sprintf("/api/admin/help-categories/%d", categoryID))
	}()

	// --- Create article ---
	createResp := client.Post("/api/admin/help-articles", map[string]any{
		"category_id": categoryID,
		"title":       fmt.Sprintf("测试帮助文章 %s", suffix),
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

	t.Logf("Created help article: id=%d, category_id=%d", articleID, categoryID)

	// --- List should contain the article ---
	listResp := client.Get("/api/admin/help-articles", map[string]string{
		"category_id": fmt.Sprintf("%d", categoryID),
		"page":        "1",
		"page_size":   "50",
	})
	testinfra.AssertPaginatedList(t, listResp, 1)

	// --- Update ---
	updateResp := client.Put(fmt.Sprintf("/api/admin/help-articles/%d", articleID), map[string]any{
		"category_id": categoryID,
		"title":       fmt.Sprintf("更新帮助文章 %s", suffix),
		"slug":        fmt.Sprintf("updated-article-%s", suffix),
		"content":     "更新后的文章正文",
		"summary":     "更新后的摘要",
		"status":      "published",
		"sort_order":  20,
		"keywords":    []string{"更新", "测试"},
	})
	updateResp.AssertSuccess(t)

	t.Logf("Updated help article %d", articleID)

	// --- Delete ---
	deleteResp := client.Delete(fmt.Sprintf("/api/admin/help-articles/%d", articleID))
	deleteResp.AssertSuccess(t)

	t.Logf("Deleted help article %d", articleID)
}
