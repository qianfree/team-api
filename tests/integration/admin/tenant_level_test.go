//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestTenantLevelList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create a level to ensure at least one exists
	_, cleanup := testinfra.CreateTestTenantLevel(t, client)
	defer cleanup()

	resp := client.Get("/api/admin/tenant-level-configs", nil)
	resp.AssertSuccess(t)

	var result struct {
		List []struct {
			ID                          int64   `json:"id"`
			Level                       int     `json:"level"`
			Name                        string  `json:"name"`
			CumulativeRechargeThreshold float64 `json:"cumulative_recharge_threshold"`
			MaxMembers                  int     `json:"max_members"`
			MaxConcurrency              int     `json:"max_concurrency"`
			PriceMultiplier             float64 `json:"price_multiplier"`
			SortOrder                   int     `json:"sort_order"`
		} `json:"list"`
	}
	resp.DecodeData(t, &result)
	if len(result.List) < 1 {
		t.Fatalf("expected at least 1 tenant level config, got %d", len(result.List))
	}
}

func TestTenantLevelCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// --- Create ---
	suffix := randomSuffix()
	createResp := client.Post("/api/admin/tenant-level-configs", map[string]any{
		"level":                         80 + int(suffix[0]%10),
		"name":                          fmt.Sprintf("CRUD测试等级 %s", suffix),
		"cumulative_recharge_threshold": 200.0,
		"max_members":                   100,
		"max_concurrency":               20,
		"price_multiplier":              0.85,
		"sort_order":                    10,
	})
	createResp.AssertSuccess(t)
	levelID := createResp.GetID(t)

	// --- List should contain the created level ---
	listResp := client.Get("/api/admin/tenant-level-configs", nil)
	listResp.AssertSuccess(t)

	var listResult struct {
		List []struct {
			ID              int64   `json:"id"`
			Name            string  `json:"name"`
			MaxMembers      int     `json:"max_members"`
			MaxConcurrency  int     `json:"max_concurrency"`
			PriceMultiplier float64 `json:"price_multiplier"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listResult)

	found := false
	for _, item := range listResult.List {
		if item.ID == levelID {
			found = true
			if item.MaxMembers != 100 {
				t.Fatalf("expected max_members=100, got %d", item.MaxMembers)
			}
			if item.MaxConcurrency != 20 {
				t.Fatalf("expected max_concurrency=20, got %d", item.MaxConcurrency)
			}
			break
		}
	}
	if !found {
		t.Fatalf("created level id=%d not found in list", levelID)
	}

	// --- Update ---
	updateResp := client.Put(fmt.Sprintf("/api/admin/tenant-level-configs/%d", levelID), map[string]any{
		"name":             fmt.Sprintf("更新等级名 %s", suffix),
		"max_members":      200,
		"max_concurrency":  50,
		"price_multiplier": 0.80,
	})
	updateResp.AssertSuccess(t)

	// Verify update via list
	verifyResp := client.Get("/api/admin/tenant-level-configs", nil)
	verifyResp.AssertSuccess(t)

	var verifyResult struct {
		List []struct {
			ID              int64   `json:"id"`
			MaxMembers      int     `json:"max_members"`
			MaxConcurrency  int     `json:"max_concurrency"`
			PriceMultiplier float64 `json:"price_multiplier"`
		} `json:"list"`
	}
	verifyResp.DecodeData(t, &verifyResult)

	for _, item := range verifyResult.List {
		if item.ID == levelID {
			if item.MaxMembers != 200 {
				t.Fatalf("expected max_members=200 after update, got %d", item.MaxMembers)
			}
			if item.MaxConcurrency != 50 {
				t.Fatalf("expected max_concurrency=50 after update, got %d", item.MaxConcurrency)
			}
			break
		}
	}

	// --- Delete ---
	deleteResp := client.Delete(fmt.Sprintf("/api/admin/tenant-level-configs/%d", levelID))
	deleteResp.AssertSuccess(t)

	// Verify deletion
	afterDeleteResp := client.Get("/api/admin/tenant-level-configs", nil)
	afterDeleteResp.AssertSuccess(t)

	var afterDeleteResult struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	afterDeleteResp.DecodeData(t, &afterDeleteResult)
	for _, item := range afterDeleteResult.List {
		if item.ID == levelID {
			t.Fatalf("level id=%d should have been deleted", levelID)
		}
	}
}

// ==================== 边界 / 错误场景 ====================

func TestTenantLevelDuplicateLevel(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	suffix := randomSuffix()
	levelNum := 70 + int(suffix[0]%10)

	createResp := client.Post("/api/admin/tenant-level-configs", map[string]any{
		"level":                         levelNum,
		"name":                          fmt.Sprintf("Dup测试 %s", suffix),
		"cumulative_recharge_threshold": 50.0,
		"max_members":                   10,
		"max_concurrency":               5,
		"price_multiplier":              0.95,
	})
	createResp.AssertSuccess(t)
	levelID := createResp.GetID(t)
	defer func() {
		client.Delete(fmt.Sprintf("/api/admin/tenant-level-configs/%d", levelID))
	}()

	// 用相同 level 号创建，应返回 400
	dupResp := client.Post("/api/admin/tenant-level-configs", map[string]any{
		"level":                         levelNum,
		"name":                          "Dup Again",
		"cumulative_recharge_threshold": 100.0,
		"max_members":                   20,
		"max_concurrency":               10,
		"price_multiplier":              0.9,
	})
	dupResp.AssertError(t, 400)
}

func TestTenantLevelDeleteDefault(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// 找到 level=1 (LV1) 的 ID
	listResp := client.Get("/api/admin/tenant-level-configs", nil)
	listResp.AssertSuccess(t)

	var listResult struct {
		List []struct {
			ID    int64 `json:"id"`
			Level int   `json:"level"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listResult)

	var lv1ID int64
	for _, item := range listResult.List {
		if item.Level == 1 {
			lv1ID = item.ID
			break
		}
	}
	if lv1ID == 0 {
		t.Fatal("LV1 (level=1) not found in list")
	}

	// 删除默认等级 LV1，应返回 400
	delResp := client.Delete(fmt.Sprintf("/api/admin/tenant-level-configs/%d", lv1ID))
	delResp.AssertError(t, 400)
}

func TestTenantLevelPartialUpdate(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	suffix := randomSuffix()
	createResp := client.Post("/api/admin/tenant-level-configs", map[string]any{
		"level":                         60 + int(suffix[0]%10),
		"name":                          fmt.Sprintf("Partial %s", suffix),
		"cumulative_recharge_threshold": 300.0,
		"max_members":                   30,
		"max_concurrency":               15,
		"price_multiplier":              0.88,
		"sort_order":                    5,
	})
	createResp.AssertSuccess(t)
	levelID := createResp.GetID(t)
	defer func() {
		client.Delete(fmt.Sprintf("/api/admin/tenant-level-configs/%d", levelID))
	}()

	// 只更新 name，其他字段应保持不变
	client.Put(fmt.Sprintf("/api/admin/tenant-level-configs/%d", levelID), map[string]any{
		"name": "Only Name Changed",
	}).AssertSuccess(t)

	listResp := client.Get("/api/admin/tenant-level-configs", nil)
	listResp.AssertSuccess(t)

	var listResult struct {
		List []struct {
			ID              int64   `json:"id"`
			Name            string  `json:"name"`
			MaxMembers      int     `json:"max_members"`
			MaxConcurrency  int     `json:"max_concurrency"`
			PriceMultiplier float64 `json:"price_multiplier"`
			SortOrder       int     `json:"sort_order"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listResult)

	for _, item := range listResult.List {
		if item.ID == levelID {
			if item.Name != "Only Name Changed" {
				t.Fatalf("expected name='Only Name Changed', got %q", item.Name)
			}
			if item.MaxMembers != 30 {
				t.Fatalf("max_members changed unexpectedly: expected 30, got %d", item.MaxMembers)
			}
			if item.MaxConcurrency != 15 {
				t.Fatalf("max_concurrency changed unexpectedly: expected 15, got %d", item.MaxConcurrency)
			}
			if item.SortOrder != 5 {
				t.Fatalf("sort_order changed unexpectedly: expected 5, got %d", item.SortOrder)
			}
			return
		}
	}
	t.Fatalf("level id=%d not found after update", levelID)
}

func TestTenantLevelSortOrder(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// 列表应按 level 升序排列（种子数据已有 1-5）
	listResp := client.Get("/api/admin/tenant-level-configs", nil)
	listResp.AssertSuccess(t)

	var listResult struct {
		List []struct {
			Level int `json:"level"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listResult)

	if len(listResult.List) < 2 {
		t.Fatal("need at least 2 level configs to verify ordering")
	}

	for i := 1; i < len(listResult.List); i++ {
		if listResult.List[i].Level < listResult.List[i-1].Level {
			t.Fatalf("list not sorted by level ascending: level[%d]=%d < level[%d]=%d",
				i, listResult.List[i].Level, i-1, listResult.List[i-1].Level)
		}
	}
}
