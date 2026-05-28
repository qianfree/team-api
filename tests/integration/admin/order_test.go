//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestOrderList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/orders", map[string]string{
		"page":      "1",
		"page_size": "20",
	})
	testinfra.AssertPaginatedList(t, resp, 0)

	total := resp.GetTotal(t)
	t.Logf("Order list: total=%d", total)
}

func TestOrderListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by status=paid
	paidResp := client.Get("/api/admin/orders", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "paid",
	})
	testinfra.AssertPaginatedList(t, paidResp, 0)

	// Filter by status=pending
	pendingResp := client.Get("/api/admin/orders", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "pending",
	})
	pendingResp.AssertSuccess(t)

	// Filter by tenant_id
	tenantID, cleanup := testinfra.CreateTestTenant(t, client)
	defer cleanup()

	tenantResp := client.Get("/api/admin/orders", map[string]string{
		"page":      "1",
		"page_size": "10",
		"tenant_id": fmt.Sprintf("%d", tenantID),
	})
	tenantResp.AssertSuccess(t)

	// Verify tenant filter returns empty for new tenant (no orders)
	var tenantData struct {
		Total int `json:"total"`
	}
	tenantResp.DecodeData(t, &tenantData)
	if tenantData.Total != 0 {
		t.Fatalf("new tenant should have 0 orders, got %d", tenantData.Total)
	}

	// Verify paid orders actually have status=paid
	if paidResp.GetTotal(t) > 0 {
		var paidData struct {
			List []struct {
				Id     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"list"`
		}
		paidResp.DecodeData(t, &paidData)

		for _, order := range paidData.List {
			if order.Status != "paid" {
				t.Fatalf("filter status=paid returned order with status=%q (id=%d)", order.Status, order.Id)
			}
		}
	}
}

func TestOrderDetail(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// List first to find an order
	listResp := client.Get("/api/admin/orders", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			Id      int64   `json:"id"`
			OrderNo string  `json:"order_no"`
			Status  string  `json:"status"`
			Amount  float64 `json:"amount"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) == 0 {
		t.Skip("No orders found, skipping detail test")
	}

	orderID := listData.List[0].Id
	expectedOrderNo := listData.List[0].OrderNo

	// Get detail
	detailResp := client.Get(fmt.Sprintf("/api/admin/orders/%d", orderID), nil)
	detailResp.AssertSuccess(t)

	var outer map[string]any
	detailResp.DecodeData(t, &outer)

	// Response has nested data: {"data": {...}}
	detail, ok := outer["data"].(map[string]any)
	if !ok {
		detail = outer
	}

	// Verify essential fields
	requiredFields := []string{"id", "order_no", "status", "amount", "tenant_id"}
	for _, field := range requiredFields {
		if _, ok := detail[field]; !ok {
			t.Fatalf("order detail missing required field: %s", field)
		}
	}

	// Verify order_no matches
	if detailOrderNo, ok := detail["order_no"].(string); !ok || detailOrderNo != expectedOrderNo {
		t.Fatalf("expected order_no=%q, got %v", expectedOrderNo, detail["order_no"])
	}

	t.Logf("Order detail verified: id=%d, order_no=%s", orderID, expectedOrderNo)
}

func TestOrderExport(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// CSV export
	csvResp := client.Get("/api/admin/orders/export", map[string]string{
		"format": "csv",
	})
	csvResp.AssertSuccess(t)

	// XLSX export
	xlsxResp := client.Get("/api/admin/orders/export", map[string]string{
		"format": "xlsx",
	})
	xlsxResp.AssertSuccess(t)
}

func TestOrderNegative(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Detail for non-existent order
	detailNonExistResp := client.Get("/api/admin/orders/999999999", nil)
	if detailNonExistResp.Code == 0 {
		t.Fatal("expected error for non-existent order detail, got success")
	}

	// Refund non-existent order
	refundNonExistResp := client.Post("/api/admin/orders/999999999/refund", map[string]any{
		"reason": "test refund",
	})
	if refundNonExistResp.Code == 0 {
		t.Fatal("expected error when refunding non-existent order, got success")
	}

	// Complete non-existent order
	completeNonExistResp := client.Post("/api/admin/orders/999999999/complete", nil)
	if completeNonExistResp.Code == 0 {
		t.Fatal("expected error when completing non-existent order, got success")
	}

	// Export with invalid format
	invalidExportResp := client.Get("/api/admin/orders/export", map[string]string{
		"format": "pdf",
	})
	if invalidExportResp.Code == 0 {
		t.Fatal("expected error for invalid export format, got success")
	}
}
