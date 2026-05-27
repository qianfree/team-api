//go:build integration

package tenant_test

import (
	"fmt"
	"testing"

	admintest "github.com/qianfree/team-api/tests/integration/admin/testinfra"
	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestMemberList(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	memberID, cleanup := testinfra.CreateTestTenantMember(t, client)
	defer cleanup()

	resp := client.Get("/api/tenant/members", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	admintest.AssertPaginatedList(t, resp, 2) // owner + member

	// Verify the created member appears
	var listData struct {
		List []struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"list"`
	}
	resp.DecodeData(t, &listData)

	found := false
	for _, m := range listData.List {
		if m.ID == memberID {
			found = true
			if m.Role != "member" {
				t.Fatalf("expected role=member, got %s", m.Role)
			}
			break
		}
	}
	if !found {
		t.Fatalf("created member %d not found in list", memberID)
	}
}

func TestMemberListWithFilters(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	_, cleanup := testinfra.CreateTestTenantMember(t, client)
	defer cleanup()

	// Filter by role
	resp := client.Get("/api/tenant/members", map[string]string{
		"page":      "1",
		"page_size": "10",
		"role":      "member",
	})
	resp.AssertSuccess(t)
	var memberList struct {
		List []struct {
			Role string `json:"role"`
		} `json:"list"`
	}
	resp.DecodeData(t, &memberList)
	for _, m := range memberList.List {
		if m.Role != "member" {
			t.Fatalf("expected role=member when filtering, got %s", m.Role)
		}
	}

	// Filter by keyword
	resp = client.Get("/api/tenant/members", map[string]string{
		"page":      "1",
		"page_size": "10",
		"keyword":   "member",
	})
	resp.AssertSuccess(t)
}

func TestMemberCreate(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	memberID, cleanup := testinfra.CreateTestTenantMember(t, client)
	defer cleanup()

	// Get detail
	detailResp := client.Get(fmt.Sprintf("/api/tenant/members/%d", memberID), nil)
	detailResp.AssertSuccess(t)

	var detail struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
		Status   string `json:"status"`
	}
	detailResp.DecodeData(t, &detail)

	if detail.ID != memberID {
		t.Fatalf("expected id=%d, got %d", memberID, detail.ID)
	}
	if detail.Role != "member" {
		t.Fatalf("expected role=member, got %s", detail.Role)
	}
	if detail.Status != "active" {
		t.Fatalf("expected status=active, got %s", detail.Status)
	}
}

func TestMemberInvite(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	// Create invitation
	inviteResp := client.Post("/api/tenant/members/invite", map[string]any{
		"role":         "member",
		"expires_days": 7,
		"max_uses":     5,
	})
	inviteResp.AssertSuccess(t)

	var inviteData struct {
		Code      string `json:"code"`
		InviteURL string `json:"invite_url"`
		ExpiresAt string `json:"expires_at"`
		MaxUses   int    `json:"max_uses"`
	}
	inviteResp.DecodeData(t, &inviteData)

	if inviteData.Code == "" {
		t.Fatal("expected non-empty invite code")
	}
	if inviteData.MaxUses != 5 {
		t.Fatalf("expected max_uses=5, got %d", inviteData.MaxUses)
	}

	// List invitations
	listResp := client.Get("/api/tenant/members/invitations", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	admintest.AssertPaginatedList(t, listResp, 1)
}

func TestMemberInviteInfo(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	inviteResp := client.Post("/api/tenant/members/invite", map[string]any{
		"role": "member",
	})
	inviteResp.AssertSuccess(t)

	var inviteData struct {
		Code string `json:"code"`
	}
	inviteResp.DecodeData(t, &inviteData)

	// Query invite info (public endpoint)
	pubClient := admintest.NewAPIClient(testinfra.DefaultBaseURL)
	infoResp := pubClient.Get("/api/tenant/members/invite-info", map[string]string{
		"code": inviteData.Code,
	})
	infoResp.AssertSuccess(t)

	var info struct {
		TenantName string `json:"tenant_name"`
		Role       string `json:"role"`
		Valid      bool   `json:"valid"`
	}
	infoResp.DecodeData(t, &info)

	if !info.Valid {
		t.Fatal("expected invite to be valid")
	}
	if info.Role != "member" {
		t.Fatalf("expected role=member, got %s", info.Role)
	}
}

func TestMemberInviteRevoke(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	inviteResp := client.Post("/api/tenant/members/invite", map[string]any{
		"role": "member",
	})
	inviteResp.AssertSuccess(t)

	// List invitations to get the ID
	listResp := client.Get("/api/tenant/members/invitations", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listData)
	if len(listData.List) == 0 {
		t.Fatal("expected at least 1 invitation")
	}

	// Revoke
	revokeResp := client.Delete(fmt.Sprintf("/api/tenant/members/invitations/%d", listData.List[0].ID))
	revokeResp.AssertSuccess(t)
}

func TestMemberUpdateRole(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	memberID, cleanup := testinfra.CreateTestTenantMember(t, client)
	defer cleanup()

	// Update role to admin
	updateResp := client.Put(fmt.Sprintf("/api/tenant/members/%d/role", memberID), map[string]any{
		"role": "admin",
	})
	updateResp.AssertSuccess(t)

	// Verify
	detailResp := client.Get(fmt.Sprintf("/api/tenant/members/%d", memberID), nil)
	detailResp.AssertSuccess(t)

	var detail struct {
		Role string `json:"role"`
	}
	detailResp.DecodeData(t, &detail)
	if detail.Role != "admin" {
		t.Fatalf("expected role=admin after update, got %s", detail.Role)
	}
}

func TestMemberResetPassword(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	memberID, cleanup := testinfra.CreateTestTenantMember(t, client)
	defer cleanup()

	resetResp := client.Put(fmt.Sprintf("/api/tenant/members/%d/reset-password", memberID), map[string]any{
		"password": "NewPass789!",
	})
	resetResp.AssertSuccess(t)
}

func TestMemberRemove(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	memberID, _ := testinfra.CreateTestTenantMember(t, client)

	// Remove
	removeResp := client.Delete(fmt.Sprintf("/api/tenant/members/%d", memberID))
	removeResp.AssertSuccess(t)

	// Verify removed - get detail should fail or return inactive status
	detailResp := client.Get(fmt.Sprintf("/api/tenant/members/%d", memberID), nil)
	detailResp.AssertSuccess(t)

	var detail struct {
		Status string `json:"status"`
	}
	detailResp.DecodeData(t, &detail)
	if detail.Status == "active" {
		t.Fatal("expected member to not be active after removal")
	}
}

func TestMemberUsage(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	memberID, cleanup := testinfra.CreateTestTenantMember(t, client)
	defer cleanup()

	resp := client.Get(fmt.Sprintf("/api/tenant/members/%d/usage", memberID), nil)
	resp.AssertSuccess(t)

	var usage struct {
		TodayRequests  float64 `json:"today_requests"`
		MonthRequests  float64 `json:"month_requests"`
		MonthInputTok  float64 `json:"month_input_tokens"`
		MonthOutputTok float64 `json:"month_output_tokens"`
		MonthTotalCost float64 `json:"month_total_cost"`
	}
	resp.DecodeData(t, &usage)
	// New member should have zero usage
	if usage.TodayRequests != 0 {
		t.Fatalf("expected today_requests=0, got %f", usage.TodayRequests)
	}
}

func TestMemberApiKeys(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	memberID, cleanup := testinfra.CreateTestTenantMember(t, client)
	defer cleanup()

	resp := client.Get(fmt.Sprintf("/api/tenant/members/%d/api-keys", memberID), map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	resp.AssertSuccess(t)

	var data struct {
		List     []any `json:"list"`
		Total    int   `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
	}
	resp.DecodeData(t, &data)
	if data.Total != 0 {
		t.Fatalf("expected 0 api keys for new member, got %d", data.Total)
	}
}

func TestMemberExport(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/members/export", map[string]string{
		"format": "csv",
	})
	resp.AssertSuccess(t)
}
