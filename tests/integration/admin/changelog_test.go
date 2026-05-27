//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestChangelogCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	suffix := randomSuffix()

	// --- Create ---
	createResp := client.Post("/api/admin/changelogs", map[string]any{
		"version": fmt.Sprintf("0.0.%s", suffix),
		"title":   fmt.Sprintf("测试更新日志 %s", suffix),
		"content": "## 新功能\n\n- 集成测试创建的更新日志\n\n## 修复\n\n- 无",
		"type":    "feature",
	})
	createResp.AssertSuccess(t)
	changelogID := createResp.GetID(t)
	defer func() {
		client.Delete(fmt.Sprintf("/api/admin/changelogs/%d", changelogID))
	}()

	t.Logf("Created changelog: id=%d", changelogID)

	// --- List should contain the changelog ---
	listResp := client.Get("/api/admin/changelogs", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	testinfra.AssertPaginatedList(t, listResp, 1)

	// --- List with type filter ---
	listByType := client.Get("/api/admin/changelogs", map[string]string{
		"page":      "1",
		"page_size": "10",
		"type":      "feature",
	})
	listByType.AssertSuccess(t)

	// --- Update ---
	updateResp := client.Put(fmt.Sprintf("/api/admin/changelogs/%d", changelogID), map[string]any{
		"version": fmt.Sprintf("0.0.%s", suffix),
		"title":   fmt.Sprintf("更新日志-已编辑 %s", suffix),
		"content": "## 更新内容\n\n- 编辑后的内容",
		"type":    "improvement",
		"status":  "draft",
	})
	updateResp.AssertSuccess(t)

	t.Logf("Updated changelog %d", changelogID)

	// --- Publish ---
	publishResp := client.Post(fmt.Sprintf("/api/admin/changelogs/%d/publish", changelogID), nil)
	publishResp.AssertSuccess(t)

	t.Logf("Published changelog %d", changelogID)

	// --- Delete ---
	deleteResp := client.Delete(fmt.Sprintf("/api/admin/changelogs/%d", changelogID))
	deleteResp.AssertSuccess(t)

	t.Logf("Deleted changelog %d", changelogID)
}
