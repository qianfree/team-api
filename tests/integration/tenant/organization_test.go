//go:build integration

package tenant_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestOrgInfo(t *testing.T) {
	client, regResult := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/organization", nil)
	resp.AssertSuccess(t)

	var data struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Code        string `json:"code"`
		Status      string `json:"status"`
		MemberCount int    `json:"member_count"`
	}
	resp.DecodeData(t, &data)

	if data.Code != regResult.TenantCode {
		t.Fatalf("expected code=%s, got %s", regResult.TenantCode, data.Code)
	}
	if data.Status != "active" {
		t.Fatalf("expected status=active, got %s", data.Status)
	}
	if data.MemberCount < 1 {
		t.Fatalf("expected member_count >= 1, got %d", data.MemberCount)
	}
}

func TestOrgUpdate(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	newName := fmt.Sprintf("更新组织名 %s", testinfra.RandomSuffix())
	updateResp := client.Put("/api/tenant/organization", map[string]any{
		"name": newName,
	})
	updateResp.AssertSuccess(t)

	// Verify update
	verifyResp := client.Get("/api/tenant/organization", nil)
	verifyResp.AssertSuccess(t)

	var data struct {
		Name string `json:"name"`
	}
	verifyResp.DecodeData(t, &data)
	if data.Name != newName {
		t.Fatalf("expected name=%s, got %s", newName, data.Name)
	}
}

func TestProfile(t *testing.T) {
	client, regResult := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/profile", nil)
	resp.AssertSuccess(t)

	var data struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
		Status   string `json:"status"`
	}
	resp.DecodeData(t, &data)

	if data.Username != regResult.Username {
		t.Fatalf("expected username=%s, got %s", regResult.Username, data.Username)
	}
	if data.Role != "owner" {
		t.Fatalf("expected role=owner, got %s", data.Role)
	}
}

func TestProfileUpdate(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	newDisplayName := fmt.Sprintf("测试用户 %s", testinfra.RandomSuffix())
	updateResp := client.Put("/api/tenant/profile", map[string]any{
		"display_name": newDisplayName,
	})
	updateResp.AssertSuccess(t)

	// Verify update
	verifyResp := client.Get("/api/tenant/profile", nil)
	verifyResp.AssertSuccess(t)

	var data struct {
		DisplayName string `json:"display_name"`
	}
	verifyResp.DecodeData(t, &data)
	if data.DisplayName != newDisplayName {
		t.Fatalf("expected display_name=%s, got %s", newDisplayName, data.DisplayName)
	}
}

func TestIPWhitelist(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	// Get current whitelist
	getResp := client.Get("/api/tenant/security/ip-whitelist", nil)
	getResp.AssertSuccess(t)

	var getData struct {
		Enabled   bool     `json:"enabled"`
		Whitelist []string `json:"whitelist"`
	}
	getResp.DecodeData(t, &getData)

	// Update whitelist (disable it to avoid locking ourselves out)
	updateResp := client.Put("/api/tenant/security/ip-whitelist", map[string]any{
		"enabled":   false,
		"whitelist": []string{},
	})
	updateResp.AssertSuccess(t)

	// Verify update
	verifyResp := client.Get("/api/tenant/security/ip-whitelist", nil)
	verifyResp.AssertSuccess(t)

	var verifyData struct {
		Enabled bool `json:"enabled"`
	}
	verifyResp.DecodeData(t, &verifyData)
	if verifyData.Enabled {
		t.Fatal("expected ip whitelist to be disabled after update")
	}
}
