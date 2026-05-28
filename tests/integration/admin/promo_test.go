//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestPromoCodeList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create one to ensure list is non-empty
	_, cleanup := createTestPromoCode(t, client)
	defer cleanup()

	resp := client.Get("/api/admin/promo-codes", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 1)
}

func TestPromoCodeCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	suffix := randomSuffix()
	code := fmt.Sprintf("PROMO-%s", suffix)
	name := fmt.Sprintf("测试优惠券 %s", suffix)

	// --- Create ---
	createResp := client.Post("/api/admin/promo-codes", map[string]any{
		"data": map[string]any{
			"code":           code,
			"name":           name,
			"type":           "percentage",
			"discount_value": 10.0,
			"min_amount":     50.0,
			"max_discount":   100.0,
			"total_count":    100,
			"per_user_limit": 1,
			"status":         "active",
		},
	})
	createResp.AssertSuccess(t)
	promoID := createResp.GetID(t)
	defer func() {
		client.Delete(fmt.Sprintf("/api/admin/promo-codes/%d", promoID))
	}()

	// --- Verify creation via list ---
	listResp := client.Get("/api/admin/promo-codes", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	testinfra.AssertPaginatedList(t, listResp, 1)

	var listData struct {
		List []struct {
			Id            int64   `json:"id"`
			Code          string  `json:"code"`
			Name          string  `json:"name"`
			Type          string  `json:"type"`
			DiscountValue float64 `json:"discount_value"`
			MinAmount     float64 `json:"min_amount"`
			MaxDiscount   float64 `json:"max_discount"`
			TotalCount    int     `json:"total_count"`
			PerUserLimit  int     `json:"per_user_limit"`
			Status        string  `json:"status"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	var created *struct {
		Id            int64   `json:"id"`
		Code          string  `json:"code"`
		Name          string  `json:"name"`
		Type          string  `json:"type"`
		DiscountValue float64 `json:"discount_value"`
		MinAmount     float64 `json:"min_amount"`
		MaxDiscount   float64 `json:"max_discount"`
		TotalCount    int     `json:"total_count"`
		PerUserLimit  int     `json:"per_user_limit"`
		Status        string  `json:"status"`
	}
	for i := range listData.List {
		if listData.List[i].Id == promoID {
			created = &listData.List[i]
			break
		}
	}
	if created == nil {
		t.Fatalf("promo code id=%d not found in list after creation", promoID)
	}
	if created.Code != code {
		t.Fatalf("expected code=%q, got %q", code, created.Code)
	}
	if created.Name != name {
		t.Fatalf("expected name=%q, got %q", name, created.Name)
	}
	if created.Type != "percentage" {
		t.Fatalf("expected type=percentage, got %q", created.Type)
	}
	if created.DiscountValue != 10.0 {
		t.Fatalf("expected discount_value=10.0, got %f", created.DiscountValue)
	}
	if created.TotalCount != 100 {
		t.Fatalf("expected total_count=100, got %d", created.TotalCount)
	}
	if created.Status != "active" {
		t.Fatalf("expected status=active, got %q", created.Status)
	}

	// --- Update ---
	updatedName := fmt.Sprintf("更新优惠券 %s", suffix)
	updateResp := client.Put(fmt.Sprintf("/api/admin/promo-codes/%d", promoID), map[string]any{
		"update": map[string]any{
			"name":           updatedName,
			"discount_value": 20.0,
			"total_count":    200,
			"status":         "disabled",
		},
	})
	updateResp.AssertSuccess(t)

	// --- Verify update via list ---
	verifyResp := client.Get("/api/admin/promo-codes", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	verifyResp.AssertSuccess(t)

	var verifyData struct {
		List []struct {
			Id            int64   `json:"id"`
			Name          string  `json:"name"`
			DiscountValue float64 `json:"discount_value"`
			TotalCount    int     `json:"total_count"`
			Status        string  `json:"status"`
		} `json:"list"`
	}
	verifyResp.DecodeData(t, &verifyData)

	var updated *struct {
		Id            int64   `json:"id"`
		Name          string  `json:"name"`
		DiscountValue float64 `json:"discount_value"`
		TotalCount    int     `json:"total_count"`
		Status        string  `json:"status"`
	}
	for i := range verifyData.List {
		if verifyData.List[i].Id == promoID {
			updated = &verifyData.List[i]
			break
		}
	}
	if updated == nil {
		t.Fatalf("promo code id=%d not found after update", promoID)
	}
	if updated.Name != updatedName {
		t.Fatalf("expected updated name=%q, got %q", updatedName, updated.Name)
	}
	if updated.DiscountValue != 20.0 {
		t.Fatalf("expected discount_value=20.0 after update, got %f", updated.DiscountValue)
	}
	if updated.Status != "disabled" {
		t.Fatalf("expected status=disabled after update, got %q", updated.Status)
	}

	// --- Usage list (should be empty) ---
	usagesResp := client.Get(fmt.Sprintf("/api/admin/promo-codes/%d/usages", promoID), map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, usagesResp, 0)

	// --- Export ---
	exportResp := client.Get("/api/admin/promo-codes/export", map[string]string{
		"format": "csv",
	})
	exportResp.AssertSuccess(t)
}

func TestPromoCodeNegative(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create with empty data should fail
	emptyResp := client.Post("/api/admin/promo-codes", map[string]any{
		"data": map[string]any{},
	})
	if emptyResp.Code == 0 {
		t.Fatal("expected error for empty promo code data, got success")
	}

	// Update non-existent promo code
	updateNonExistResp := client.Put("/api/admin/promo-codes/999999999", map[string]any{
		"update": map[string]any{"name": "test"},
	})
	if updateNonExistResp.Code == 0 {
		t.Fatal("expected error when updating non-existent promo code, got success")
	}
}

// createTestPromoCode is a test helper that creates a promo code and returns its ID and cleanup.
func createTestPromoCode(t *testing.T, client *testinfra.APIClient) (int64, func()) {
	t.Helper()
	suffix := randomSuffix()
	resp := client.Post("/api/admin/promo-codes", map[string]any{
		"data": map[string]any{
			"code":           fmt.Sprintf("TEST-%s", suffix),
			"name":           fmt.Sprintf("测试优惠券 %s", suffix),
			"type":           "fixed",
			"discount_value": 5.0,
			"total_count":    10,
			"per_user_limit": 1,
			"status":         "active",
		},
	})
	resp.AssertSuccess(t)
	id := resp.GetID(t)
	return id, func() {
		client.Delete(fmt.Sprintf("/api/admin/promo-codes/%d", id))
	}
}
