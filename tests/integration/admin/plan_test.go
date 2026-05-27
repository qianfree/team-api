//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestPlanList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/plans", map[string]string{
		"page":      "1",
		"page_size": "20",
	})
	testinfra.AssertPaginatedList(t, resp, 0)

	t.Logf("Plan list: total=%d", resp.GetTotal(t))

	// Filter by active status
	respActive := client.Get("/api/admin/plans", map[string]string{
		"page":      "1",
		"page_size": "20",
		"status":    "active",
	})
	testinfra.AssertPaginatedList(t, respActive, 0)

	t.Logf("Active plan list: total=%d", respActive.GetTotal(t))
}

func TestPlanCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create
	planID, cleanup := testinfra.CreateTestPlan(t, client)
	defer cleanup()

	t.Logf("Created plan: id=%d", planID)

	// Read
	detailResp := client.Get(fmt.Sprintf("/api/admin/plans/%d", planID), nil)
	detailResp.AssertSuccess(t)

	var detail struct {
		Data struct {
			Id         int64  `json:"id"`
			Name       string `json:"name"`
			Identifier string `json:"identifier"`
			Status     string `json:"status"`
		} `json:"data"`
	}
	detailResp.DecodeData(t, &detail)

	if detail.Data.Id != planID {
		t.Fatalf("expected plan id=%d, got %d", planID, detail.Data.Id)
	}

	t.Logf("Plan detail: name=%s, identifier=%s, status=%s",
		detail.Data.Name, detail.Data.Identifier, detail.Data.Status)

	// Update
	updateResp := client.Put(fmt.Sprintf("/api/admin/plans/%d", planID), map[string]any{
		"update": map[string]any{
			"description": "updated by integration test",
		},
	})
	updateResp.AssertSuccess(t)

	t.Logf("Plan updated successfully")

	// Verify update
	verifyResp := client.Get(fmt.Sprintf("/api/admin/plans/%d", planID), nil)
	verifyResp.AssertSuccess(t)

	var verifyData struct {
		Data struct {
			Description string `json:"description"`
		} `json:"data"`
	}
	verifyResp.DecodeData(t, &verifyData)

	if verifyData.Data.Description != "updated by integration test" {
		t.Fatalf("expected description 'updated by integration test', got %q",
			verifyData.Data.Description)
	}

	t.Logf("Plan update verified")
}

func TestPlanToggleRecommend(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	planID, cleanup := testinfra.CreateTestPlan(t, client)
	defer cleanup()

	// Toggle recommend on
	resp := client.Put(fmt.Sprintf("/api/admin/plans/%d/toggle-recommend", planID), nil)
	resp.AssertSuccess(t)

	// Verify toggle
	detailResp := client.Get(fmt.Sprintf("/api/admin/plans/%d", planID), nil)
	detailResp.AssertSuccess(t)

	var detail struct {
		Data struct {
			IsRecommended bool `json:"is_recommended"`
		} `json:"data"`
	}
	detailResp.DecodeData(t, &detail)

	t.Logf("Plan %d is_recommended=%v after first toggle", planID, detail.Data.IsRecommended)

	// Toggle again to revert
	resp2 := client.Put(fmt.Sprintf("/api/admin/plans/%d/toggle-recommend", planID), nil)
	resp2.AssertSuccess(t)

	t.Logf("Plan toggle-recommend completed successfully")
}
