//go:build integration

package admin_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestRedemptionList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create some to ensure list is non-empty
	_, cleanup := createTestRedemptions(t, client, 3, "quota")
	defer cleanup()

	resp := client.Get("/api/admin/redemptions", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 1)
}

func TestRedemptionListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	_, cleanup := createTestRedemptions(t, client, 2, "quota")
	defer cleanup()

	// Filter by status=active
	resp := client.Get("/api/admin/redemptions", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "active",
	})
	resp.AssertSuccess(t)

	// Filter by status=used
	resp2 := client.Get("/api/admin/redemptions", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "used",
	})
	resp2.AssertSuccess(t)
}

func TestRedemptionBatchCreate(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Batch create quota-type redemption codes
	count := 5
	created, cleanup := createTestRedemptions(t, client, count, "quota")
	defer cleanup()

	if created != count {
		t.Fatalf("expected %d created, got %d", count, created)
	}

	t.Logf("Batch created %d redemption codes", created)
}

func TestRedemptionDisable(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create redemptions and find one to disable
	_, cleanup := createTestRedemptions(t, client, 1, "quota")
	defer cleanup()

	// Find the created code in the list
	listResp := client.Get("/api/admin/redemptions", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "active",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			Id     int64  `json:"id"`
			Code   string `json:"code"`
			Status string `json:"status"`
			Type   string `json:"type"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) == 0 {
		t.Fatal("no active redemption codes found to test disable")
	}

	targetID := listData.List[0].Id

	// Disable the code
	disableResp := client.Put(fmt.Sprintf("/api/admin/redemptions/%d/disable", targetID), nil)
	disableResp.AssertSuccess(t)

	// Verify the code is no longer active
	activeResp := client.Get("/api/admin/redemptions", map[string]string{
		"page":      "1",
		"page_size": "50",
		"status":    "active",
	})
	activeResp.AssertSuccess(t)

	var activeData struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
	}
	activeResp.DecodeData(t, &activeData)

	for _, item := range activeData.List {
		if item.Id == targetID {
			t.Fatal("disabled redemption code should not appear in active list")
		}
	}

	t.Logf("Redemption code %d disabled successfully", targetID)
}

func TestRedemptionUsages(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Usage list (may be empty)
	resp := client.Get("/api/admin/redemptions/usages", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestRedemptionBatchCreateWithMaxUsesAndExpiry(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Post("/api/admin/redemptions", map[string]any{
		"count":        2,
		"type":         "quota",
		"value":        10.0,
		"max_uses":     5,
		"expires_days": 7,
	})
	resp.AssertSuccess(t)

	var data struct {
		Created int `json:"created"`
	}
	resp.DecodeData(t, &data)
	if data.Created != 2 {
		t.Fatalf("expected 2 created, got %d", data.Created)
	}

	// 列表按 created_at 倒序，新建的码带 max_uses=5 标记，取前两条核对
	listResp := client.Get("/api/admin/redemptions", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "active",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			Id        int64  `json:"id"`
			MaxUses   int    `json:"max_uses"`
			ExpiresAt string `json:"expires_at"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	now := time.Now()
	checked := 0
	var ids []int64
	for _, item := range listData.List {
		if item.MaxUses != 5 {
			continue
		}
		expiresAt := parseTime(t, item.ExpiresAt)
		days := expiresAt.Sub(now).Hours() / 24
		if days < 6.9 || days > 7.1 {
			t.Fatalf("expected expires_at ~7 days from now, got %.2f days (%s)", days, item.ExpiresAt)
		}
		ids = append(ids, item.Id)
		checked++
		if checked == 2 {
			break
		}
	}
	if checked < 2 {
		t.Fatalf("expected 2 codes with max_uses=5, found %d", checked)
	}

	for _, id := range ids {
		testinfra.HardDeleteRedemption(t, id)
	}
}

func TestRedemptionUsagesFilterByRedemptionId(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// 新建的码没有任何兑换记录，按其 id 过滤使用记录应为空
	_, cleanup := createTestRedemptions(t, client, 1, "quota")
	defer cleanup()

	listResp := client.Get("/api/admin/redemptions", map[string]string{
		"page":      "1",
		"page_size": "50",
		"status":    "active",
	})
	listResp.AssertSuccess(t)
	var listData struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)
	if len(listData.List) == 0 {
		t.Fatal("no active redemption codes found")
	}

	targetID := listData.List[0].Id
	resp := client.Get("/api/admin/redemptions/usages", map[string]string{
		"page":          "1",
		"page_size":     "10",
		"redemption_id": fmt.Sprintf("%d", targetID),
	})
	resp.AssertSuccess(t)
	if resp.GetTotal(t) != 0 {
		t.Fatalf("expected 0 usages for fresh redemption %d, got %d", targetID, resp.GetTotal(t))
	}
}

// parseTime 兼容 gtime 序列化的两种常见时间格式。
// 无时区后缀的墙上时间按 DB 时区（Asia/Shanghai, +08:00）解释，
// time.Parse 会误当 UTC 导致偏差 8 小时。
func parseTime(t *testing.T, s string) time.Time {
	t.Helper()
	loc := time.FixedZone("+08", 8*3600)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if ts, err := time.ParseInLocation(layout, s, loc); err == nil {
			return ts
		}
	}
	t.Fatalf("cannot parse time %q", s)
	return time.Time{}
}

func TestRedemptionExport(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/redemptions/export", map[string]string{
		"format": "csv",
	})
	resp.AssertSuccess(t)

	resp2 := client.Get("/api/admin/redemptions/export", map[string]string{
		"format": "xlsx",
	})
	resp2.AssertSuccess(t)
}

func TestRedemptionNegative(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create with count=0 should fail
	zeroCountResp := client.Post("/api/admin/redemptions", map[string]any{
		"count": 0,
		"type":  "quota",
	})
	if zeroCountResp.Code == 0 {
		t.Fatal("expected error for count=0, got success")
	}

	// Create with invalid type should fail
	invalidTypeResp := client.Post("/api/admin/redemptions", map[string]any{
		"count": 1,
		"type":  "invalid",
	})
	if invalidTypeResp.Code == 0 {
		t.Fatal("expected error for invalid type, got success")
	}

	// Disable non-existent code
	disableNonExistResp := client.Put("/api/admin/redemptions/999999999/disable", nil)
	if disableNonExistResp.Code == 0 {
		t.Fatal("expected error when disabling non-existent code, got success")
	}
}

// createTestRedemptions is a test helper that batch-creates redemption codes and returns count and cleanup.
// The cleanup disables the created codes after the test.
func createTestRedemptions(t *testing.T, client *testinfra.APIClient, count int, redemptionType string) (int, func()) {
	t.Helper()
	resp := client.Post("/api/admin/redemptions", map[string]any{
		"count": count,
		"type":  redemptionType,
		"value": 10.0,
	})
	resp.AssertSuccess(t)

	var data struct {
		Created int `json:"created"`
	}
	resp.DecodeData(t, &data)

	// Find the newly created codes by listing active codes (most recent = highest IDs)
	var ids []int64
	listResp := client.Get("/api/admin/redemptions", map[string]string{
		"page":      "1",
		"page_size": "50",
		"status":    "active",
	})
	if listResp.Code == 0 {
		var listData struct {
			List []struct {
				Id int64 `json:"id"`
			} `json:"list"`
		}
		listResp.DecodeData(t, &listData)

		// Take the last `count` items (most recently created)
		start := len(listData.List) - data.Created
		if start < 0 {
			start = 0
		}
		for i := start; i < len(listData.List); i++ {
			ids = append(ids, listData.List[i].Id)
		}
	}

	return data.Created, func() {
		for _, id := range ids {
			testinfra.HardDeleteRedemption(t, id)
		}
	}
}
