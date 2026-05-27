//go:build integration

package tenant_test

import (
	"fmt"
	"testing"

	admintest "github.com/qianfree/team-api/tests/integration/admin/testinfra"
	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestTenantRegister_Success(t *testing.T) {
	result := testinfra.RegisterTestTenant(t)

	if result.AccessToken == "" {
		t.Fatal("expected non-empty access_token")
	}
	if result.RefreshToken == "" {
		t.Fatal("expected non-empty refresh_token")
	}
	if result.ExpiresAt == "" {
		t.Fatal("expected non-empty expires_at")
	}
	if result.Tenant.ID == 0 {
		t.Fatal("expected tenant.id > 0")
	}
	if result.Tenant.Code == "" {
		t.Fatal("expected tenant.code")
	}
	if result.User.Role != "owner" {
		t.Fatalf("expected role=owner, got %s", result.User.Role)
	}
}

func TestTenantRegister_DuplicateTenantCode(t *testing.T) {
	result := testinfra.RegisterTestTenant(t)

	// Try registering again with the same tenant_code
	captchaKey, captchaX := testinfra.SolveCaptcha(t)
	client := admintest.NewAPIClient(testinfra.DefaultBaseURL)
	resp := client.Post("/api/tenant/auth/register", map[string]any{
		"email":       fmt.Sprintf("dup-%s@test.com", result.TenantCode),
		"password":    testinfra.TestPassword,
		"tenant_name": "重复租户",
		"tenant_code": result.TenantCode,
		"username":    "anotheruser",
		"captcha_key": captchaKey,
		"captcha_x":   captchaX,
	})
	if resp.Code == 0 {
		t.Fatal("expected error for duplicate tenant_code, but got success")
	}
}

func TestTenantLogin_RAM_Success(t *testing.T) {
	regResult := testinfra.RegisterTestTenant(t)

	loginResult := testinfra.LoginTenant(t, regResult.Username, regResult.TenantCode, regResult.Password)

	if loginResult.AccessToken == "" {
		t.Fatal("expected non-empty access_token after RAM login")
	}
	if loginResult.RefreshToken == "" {
		t.Fatal("expected non-empty refresh_token after RAM login")
	}
}

func TestTenantLogin_InvalidPassword(t *testing.T) {
	regResult := testinfra.RegisterTestTenant(t)

	captchaKey, captchaX := testinfra.SolveCaptcha(t)
	client := admintest.NewAPIClient(testinfra.DefaultBaseURL)
	account := fmt.Sprintf("%s@%s", regResult.Username, regResult.TenantCode)
	resp := client.Post("/api/tenant/auth/login", map[string]any{
		"account":     account,
		"password":    "WrongPassword123!",
		"type":        "ram",
		"captcha_key": captchaKey,
		"captcha_x":   captchaX,
	})
	if resp.Code == 0 {
		t.Fatal("expected login to fail with wrong password, but got success")
	}
}

func TestTenantLogout_Success(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	logoutResp := client.Post("/api/tenant/auth/logout", nil)
	logoutResp.AssertSuccess(t)

	// Verify token is invalidated
	sessionResp := client.Get("/api/tenant/auth/sessions", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	sessionResp.AssertHTTPStatus(t, 401)
}

func TestTenantTokenRefresh(t *testing.T) {
	regResult := testinfra.RegisterTestTenant(t)

	client := admintest.NewAPIClient(testinfra.DefaultBaseURL)
	refreshResp := client.Post("/api/tenant/auth/refresh", map[string]any{
		"refresh_token": regResult.RefreshToken,
	})
	refreshResp.AssertSuccess(t)

	var refreshData struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresAt    string `json:"expires_at"`
	}
	refreshResp.DecodeData(t, &refreshData)

	if refreshData.AccessToken == "" {
		t.Fatal("expected non-empty access_token after refresh")
	}

	// Verify new token works
	authedClient := admintest.NewAPIClient(testinfra.DefaultBaseURL).WithToken(refreshData.AccessToken)
	sessionsResp := authedClient.Get("/api/tenant/auth/sessions", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	sessionsResp.AssertSuccess(t)
}

func TestTenantChangePassword(t *testing.T) {
	regResult := testinfra.RegisterTestTenant(t)
	client := admintest.NewAPIClient(testinfra.DefaultBaseURL).WithToken(regResult.AccessToken)

	newPassword := "NewPass456!"
	changeResp := client.Put("/api/tenant/auth/change-password", map[string]any{
		"old_password": regResult.Password,
		"new_password": newPassword,
	})
	changeResp.AssertSuccess(t)

	// Verify can login with new password
	loginResult := testinfra.LoginTenant(t, regResult.Username, regResult.TenantCode, newPassword)
	if loginResult.AccessToken == "" {
		t.Fatal("expected login with new password to succeed")
	}
}

func TestTenantSessionList(t *testing.T) {
	regResult := testinfra.RegisterTestTenant(t)
	client := admintest.NewAPIClient(testinfra.DefaultBaseURL).WithToken(regResult.AccessToken)

	resp := client.Get("/api/tenant/auth/sessions", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	resp.AssertSuccess(t)

	// Verify paginated response structure exists
	var data struct {
		List     []any `json:"list"`
		Total    int   `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
	}
	resp.DecodeData(t, &data)
}

func TestTenantSessionRevoke(t *testing.T) {
	regResult := testinfra.RegisterTestTenant(t)
	client := admintest.NewAPIClient(testinfra.DefaultBaseURL).WithToken(regResult.AccessToken)

	// List sessions
	listResp := client.Get("/api/tenant/auth/sessions", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	listResp.AssertSuccess(t)

	var listData struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
		Total int `json:"total"`
	}
	listResp.DecodeData(t, &listData)

	if len(listData.List) == 0 {
		t.Skip("no sessions to revoke, session list empty")
	}

	// Revoke the first session
	revokeResp := client.Delete(fmt.Sprintf("/api/tenant/auth/sessions/%d", listData.List[0].ID))
	revokeResp.AssertSuccess(t)
}

func TestTenantAuth_Unauthorized(t *testing.T) {
	client := admintest.NewAPIClient(testinfra.DefaultBaseURL)
	resp := client.Get("/api/tenant/auth/sessions", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	resp.AssertHTTPStatus(t, 401)
}
