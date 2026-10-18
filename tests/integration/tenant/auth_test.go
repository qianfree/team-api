//go:build integration

package tenant_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/qianfree/team-api/internal/consts"
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

// TestTenantRegister_TeamEnabledSemantics 验证注册的团队功能激活语义：
// 显式传入组织代码 = 激活团队功能（与组织设置页「设置 code 即激活」一致）；
// 留空由系统分配随机码 = 保持个人模式（team_enabled=false）
func TestTenantRegister_TeamEnabledSemantics(t *testing.T) {
	client := admintest.NewAPIClient(testinfra.DefaultBaseURL)

	// 显式传入 tenant_code → team_enabled=true
	suffix := testinfra.RandomSuffix()
	captchaKey, captchaX := testinfra.SolveCaptcha(t)
	withCodeResp := client.Post("/api/tenant/auth/register", map[string]any{
		"email":       fmt.Sprintf("team-on-%s@test.com", suffix),
		"password":    testinfra.TestPassword,
		"tenant_name": fmt.Sprintf("显码租户%s", suffix),
		"tenant_code": "t-" + suffix,
		"username":    "teamon" + suffix,
		"captcha_key": captchaKey,
		"captcha_x":   captchaX,
	})
	withCodeResp.AssertSuccess(t)
	var withCode struct {
		Tenant struct {
			ID          int64 `json:"id"`
			TeamEnabled bool  `json:"team_enabled"`
		} `json:"tenant"`
	}
	withCodeResp.DecodeData(t, &withCode)
	t.Cleanup(func() { testinfra.HardDeleteTenant(t, withCode.Tenant.ID) })
	if !withCode.Tenant.TeamEnabled {
		t.Fatal("explicit tenant_code should activate team features (team_enabled=true)")
	}

	// 留空 tenant_code → 系统分配随机码，保持个人模式
	suffix = testinfra.RandomSuffix()
	captchaKey, captchaX = testinfra.SolveCaptcha(t)
	noCodeResp := client.Post("/api/tenant/auth/register", map[string]any{
		"email":       fmt.Sprintf("team-off-%s@test.com", suffix),
		"password":    testinfra.TestPassword,
		"tenant_name": fmt.Sprintf("隐码租户%s", suffix),
		"username":    "teamoff" + suffix,
		"captcha_key": captchaKey,
		"captcha_x":   captchaX,
	})
	noCodeResp.AssertSuccess(t)
	var noCode struct {
		Tenant struct {
			ID          int64  `json:"id"`
			Code        string `json:"code"`
			TeamEnabled bool   `json:"team_enabled"`
		} `json:"tenant"`
	}
	noCodeResp.DecodeData(t, &noCode)
	t.Cleanup(func() { testinfra.HardDeleteTenant(t, noCode.Tenant.ID) })
	if noCode.Tenant.TeamEnabled {
		t.Fatal("auto-generated tenant_code should keep personal mode (team_enabled=false)")
	}
	if !strings.HasPrefix(noCode.Tenant.Code, "org-") {
		t.Fatalf("expected auto-generated org- prefixed code, got %s", noCode.Tenant.Code)
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

func TestSuspendedTenantExistingTokensAreRejected(t *testing.T) {
	regResult := testinfra.RegisterTestTenant(t)
	adminClient := admintest.GetAuthedClient(t)

	setStatus := func(status string) {
		t.Helper()
		resp := adminClient.Put(fmt.Sprintf("/api/admin/tenants/%d/status", regResult.Tenant.ID), map[string]any{
			"status": status,
		})
		resp.AssertSuccess(t)
	}
	setStatus("suspended")
	defer setStatus("active")

	tenantClient := admintest.NewAPIClient(testinfra.DefaultBaseURL).WithToken(regResult.AccessToken)
	accessResp := tenantClient.Get("/api/tenant/auth/sessions", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	accessResp.AssertHTTPStatus(t, 403)
	accessResp.AssertError(t, consts.CodeTenantSuspended)

	publicClient := admintest.NewAPIClient(testinfra.DefaultBaseURL)
	refreshResp := publicClient.Post("/api/tenant/auth/refresh", map[string]any{
		"refresh_token": regResult.RefreshToken,
	})
	refreshResp.AssertError(t, consts.CodeTenantSuspended)

	setStatus("active")
	refreshAfterActivation := publicClient.Post("/api/tenant/auth/refresh", map[string]any{
		"refresh_token": regResult.RefreshToken,
	})
	refreshAfterActivation.AssertSuccess(t)
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
// Business rule: 注册时 username/password 必填；tenant_name 可选（留空自动以用户名填充）
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

	// 空租户名称 → 已放宽为可选（简化注册）：留空自动以用户名填充
	nameSuffix := testinfra.RandomSuffix()
	captchaKey, captchaX = testinfra.SolveCaptcha(t)
	resp = client.Post("/api/tenant/auth/register", map[string]any{
		"email":       fmt.Sprintf("empty-name-%s@test.com", nameSuffix),
		"password":    testinfra.TestPassword,
		"tenant_name": "",
		"tenant_code": "t" + testinfra.RandomSuffix(),
		"username":    "autoname" + nameSuffix,
		"captcha_key": captchaKey,
		"captcha_x":   captchaX,
	})
	resp.AssertSuccess(t)
	var autoName struct {
		Tenant struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"tenant"`
	}
	resp.DecodeData(t, &autoName)
	t.Cleanup(func() { testinfra.HardDeleteTenant(t, autoName.Tenant.ID) })
	if autoName.Tenant.Name != "autoname"+nameSuffix {
		t.Fatalf("expected tenant_name auto-filled from username, got %q", autoName.Tenant.Name)
	}
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
