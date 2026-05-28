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

// ─── 边界值测试 ────────────────────────────────────────────────────

// TestTenantRegister_EmptyRequiredFields 验证缺失必填字段时注册被拒绝
// Business rule: 注册时 username/password/tenant_name 不能为空
func TestTenantRegister_EmptyRequiredFields(t *testing.T) {
	captchaKey, captchaX := testinfra.SolveCaptcha(t)
	client := admintest.NewAPIClient(testinfra.DefaultBaseURL)

	// 空用户名
	resp := client.Post("/api/tenant/auth/register", map[string]any{
		"email":       "empty-user@test.com",
		"password":    testinfra.TestPassword,
		"tenant_name": "空用户名租户",
		"tenant_code": "t" + testinfra.RandomSuffix(),
		"username":    "",
		"captcha_key": captchaKey,
		"captcha_x":   captchaX,
	})
	if resp.Code == 0 {
		t.Fatal("empty username should be rejected")
	}
	t.Logf("empty username: code=%d msg=%q", resp.Code, resp.Message)

	// 空密码
	captchaKey, captchaX = testinfra.SolveCaptcha(t)
	resp = client.Post("/api/tenant/auth/register", map[string]any{
		"email":       "empty-pwd@test.com",
		"password":    "",
		"tenant_name": "空密码租户",
		"tenant_code": "t" + testinfra.RandomSuffix(),
		"username":    "emptypwduser",
		"captcha_key": captchaKey,
		"captcha_x":   captchaX,
	})
	if resp.Code == 0 {
		t.Fatal("empty password should be rejected")
	}
	t.Logf("empty password: code=%d msg=%q", resp.Code, resp.Message)

	// 空租户名称
	captchaKey, captchaX = testinfra.SolveCaptcha(t)
	resp = client.Post("/api/tenant/auth/register", map[string]any{
		"email":       "empty-name@test.com",
		"password":    testinfra.TestPassword,
		"tenant_name": "",
		"tenant_code": "t" + testinfra.RandomSuffix(),
		"username":    "emptynameuser",
		"captcha_key": captchaKey,
		"captcha_x":   captchaX,
	})
	if resp.Code == 0 {
		t.Fatal("empty tenant_name should be rejected")
	}
	t.Logf("empty tenant_name: code=%d msg=%q", resp.Code, resp.Message)
}

// TestTenantRegister_ShortPassword 验证短密码被拒绝
// Business rule: 密码长度最少 8 位
func TestTenantRegister_ShortPassword(t *testing.T) {
	captchaKey, captchaX := testinfra.SolveCaptcha(t)
	client := admintest.NewAPIClient(testinfra.DefaultBaseURL)

	resp := client.Post("/api/tenant/auth/register", map[string]any{
		"email":       "short-pwd@test.com",
		"password":    "abc123",
		"tenant_name": "短密码租户",
		"tenant_code": "t" + testinfra.RandomSuffix(),
		"username":    "shortpwduser",
		"captcha_key": captchaKey,
		"captcha_x":   captchaX,
	})
	if resp.Code == 0 {
		t.Fatal("short password should be rejected (min 8 chars)")
	}
	t.Logf("short password: code=%d msg=%q", resp.Code, resp.Message)
}

// TestTenantRegister_InvalidEmail 验证无效邮箱格式被拒绝
// Business rule: email 必须是合法邮箱格式
func TestTenantRegister_InvalidEmail(t *testing.T) {
	captchaKey, captchaX := testinfra.SolveCaptcha(t)
	client := admintest.NewAPIClient(testinfra.DefaultBaseURL)

	resp := client.Post("/api/tenant/auth/register", map[string]any{
		"email":       "not-an-email",
		"password":    testinfra.TestPassword,
		"tenant_name": "无效邮箱租户",
		"tenant_code": "t" + testinfra.RandomSuffix(),
		"username":    "bademailuser",
		"captcha_key": captchaKey,
		"captcha_x":   captchaX,
	})
	if resp.Code == 0 {
		t.Fatal("invalid email should be rejected")
	}
	t.Logf("invalid email: code=%d msg=%q", resp.Code, resp.Message)
}

// TestTenantLogin_EmptyFields 验证空字段登录被拒绝
func TestTenantLogin_EmptyFields(t *testing.T) {
	captchaKey, captchaX := testinfra.SolveCaptcha(t)
	client := admintest.NewAPIClient(testinfra.DefaultBaseURL)

	resp := client.Post("/api/tenant/auth/login", map[string]any{
		"account":     "",
		"password":    "",
		"type":        "ram",
		"captcha_key": captchaKey,
		"captcha_x":   captchaX,
	})
	if resp.Code == 0 {
		t.Fatal("empty login fields should be rejected")
	}
	t.Logf("empty login fields: code=%d msg=%q", resp.Code, resp.Message)
}
