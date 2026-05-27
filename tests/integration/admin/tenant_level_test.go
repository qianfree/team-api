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
