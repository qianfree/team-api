//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

// TestTenantTagsAndRemark 租户标签/备注管理：更新 → 列表/详情可读 → 标签筛选 → 清空。
// 运行前置：服务已部署包含 000025 迁移（tnt_tenants.tags/remark 列）。
func TestTenantTagsAndRemark(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// 取一个真实租户：列表第一条
	listResp := client.Get("/api/admin/tenants", map[string]string{"page": "1", "page_size": "1"})
	listResp.AssertSuccess(t)
	var listData struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)
	if len(listData.List) == 0 {
		t.Skip("无租户数据，跳过标签/备注集成测试")
	}
	tenantID := listData.List[0].ID

	marker := fmt.Sprintf("it-tag-%d", tenantID)

	// 1. 更新标签与备注
	updateResp := client.Put(fmt.Sprintf("/api/admin/tenants/%d", tenantID), map[string]any{
		"tags":   []string{marker, "集成测试"},
		"remark": "集成测试备注",
	})
	updateResp.AssertSuccess(t)

	// 2. 详情可读
	getResp := client.Get(fmt.Sprintf("/api/admin/tenants/%d", tenantID), nil)
	getResp.AssertSuccess(t)
	var detail struct {
		Tags          []string `json:"tags"`
		Remark        string   `json:"remark"`
		TotalConsumed string   `json:"total_consumed"`
	}
	getResp.DecodeData(t, &detail)
	if len(detail.Tags) != 2 || detail.Tags[0] != marker {
		t.Fatalf("tags = %v, want [%s 集成测试]", detail.Tags, marker)
	}
	if detail.Remark != "集成测试备注" {
		t.Fatalf("remark = %q", detail.Remark)
	}
	if detail.TotalConsumed == "" {
		t.Fatal("total_consumed 应返回 decimal 字符串（无消费时为 \"0\"）")
	}

	// 3. 标签精确筛选能命中
	filterResp := client.Get("/api/admin/tenants", map[string]string{
		"page": "1", "page_size": "20", "tag": marker,
	})
	filterResp.AssertSuccess(t)
	var filterData struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	filterResp.DecodeData(t, &filterData)
	if len(filterData.List) != 1 || filterData.List[0].ID != tenantID {
		t.Fatalf("tag filter should hit tenant %d exactly, got %v", tenantID, filterData.List)
	}

	// 4. 标签数量超限被拒（11 个）
	tooMany := make([]string, 11)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("tag-%d", i)
	}
	badResp := client.Put(fmt.Sprintf("/api/admin/tenants/%d", tenantID), map[string]any{
		"tags": tooMany,
	})
	if badResp.Code == 0 {
		t.Fatal("11 个标签应被拒绝")
	}

	// 5. 清空标签/备注（空数组语义）
	clearResp := client.Put(fmt.Sprintf("/api/admin/tenants/%d", tenantID), map[string]any{
		"tags": []string{}, "remark": "",
	})
	clearResp.AssertSuccess(t)
	getResp2 := client.Get(fmt.Sprintf("/api/admin/tenants/%d", tenantID), nil)
	getResp2.AssertSuccess(t)
	var detail2 struct {
		Tags   []string `json:"tags"`
		Remark string   `json:"remark"`
	}
	getResp2.DecodeData(t, &detail2)
	if len(detail2.Tags) != 0 || detail2.Remark != "" {
		t.Fatalf("清空后 tags=%v remark=%q", detail2.Tags, detail2.Remark)
	}
}
