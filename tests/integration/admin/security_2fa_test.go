//go:build integration

package admin_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

// TestAdmin2FALifecycle 管理后台两步验证完整生命周期：
// 绑定（setup→enable）→ me 状态 → 登录二段式（totp_required）→
// 错码失败计数（10048 → 10056，per-user 计数跨临时令牌存活）→
// 窗口自愈（清计数等价于等待 300s 过期）→ 恢复码登录 → 恢复码关闭 → me 复核。
func TestAdmin2FALifecycle(t *testing.T) {
	admin := testinfra.GetAuthedClient(t)
	userID, username, password, cleanup := testinfra.CreateTestAdminUserWithCreds(t, admin)
	defer cleanup()

	public := testinfra.NewAPIClient(testinfra.DefaultBaseURL)

	// ── 未开启 2FA：登录直接拿令牌 ──
	login1 := testinfra.AuthenticateWithCreds(t, username, password)
	if login1.TotpRequired {
		t.Fatal("fresh user should not require 2FA")
	}
	userClient := public.WithToken(login1.AccessToken)

	// ── 绑定：setup 返回密钥与 otpauth URI ──
	setupResp := userClient.Post("/api/admin/security/2fa/setup", map[string]any{})
	setupResp.AssertSuccess(t)
	var setupData struct {
		Secret string `json:"secret"`
		URI    string `json:"uri"`
	}
	setupResp.DecodeData(t, &setupData)
	if setupData.Secret == "" || !strings.HasPrefix(setupData.URI, "otpauth://") {
		t.Fatalf("unexpected setup response: secret=%q uri=%q", setupData.Secret, setupData.URI)
	}

	// ── 用当前 TOTP 码 + 密码确认启用 ──
	totpCode, err := totp.GenerateCode(setupData.Secret, time.Now())
	if err != nil {
		t.Fatalf("generate totp code error: %v", err)
	}
	enableResp := userClient.Post("/api/admin/security/2fa/enable", map[string]any{
		"code":     totpCode,
		"password": password,
	})
	enableResp.AssertSuccess(t)
	var enableData struct {
		BackupCodes []string `json:"backup_codes"`
	}
	enableResp.DecodeData(t, &enableData)
	if len(enableData.BackupCodes) != 8 {
		t.Fatalf("expected 8 backup codes, got %d", len(enableData.BackupCodes))
	}

	// ── me 返回 totp_enabled=true ──
	meResp := userClient.Get("/api/admin/auth/me", nil)
	meResp.AssertSuccess(t)
	var meData struct {
		TotpEnabled bool `json:"totp_enabled"`
	}
	meResp.DecodeData(t, &meData)
	if !meData.TotpEnabled {
		t.Fatal("me should report totp_enabled=true after enable")
	}

	// ── 已启用守卫：setup+enable 重复绑定被拒绝（10049），封死覆盖已绑定
	//    totp_secret/backup_codes 的持久接管链；换绑必须先 disable ──
	setupResp2 := userClient.Post("/api/admin/security/2fa/setup", map[string]any{})
	setupResp2.AssertSuccess(t)
	var setupData2 struct {
		Secret string `json:"secret"`
	}
	setupResp2.DecodeData(t, &setupData2)
	rebindCode, err := totp.GenerateCode(setupData2.Secret, time.Now())
	if err != nil {
		t.Fatalf("generate totp rebind code error: %v", err)
	}
	userClient.Post("/api/admin/security/2fa/enable", map[string]any{
		"code":     rebindCode,
		"password": password,
	}).AssertError(t, consts.CodeTotpAlreadyEnabled)

	// ── 重新登录：totp_required + provisional_token ──
	login2 := testinfra.LoginWithCreds(t, username, password)
	if !login2.TotpRequired || login2.ProvisionalToken == "" {
		t.Fatalf("expected totp_required with provisional_token, got %+v", login2)
	}

	verifyPayload := func(provisional, code string) map[string]any {
		return map[string]any{"provisional_token": provisional, "code": code}
	}

	// ── 错码直至被失败计数拦截（10048 → 10056），阈值读运行配置，循环上限兜底 ──
	sawInvalid, sawBlocked := false, false
	for i := 0; i < 15; i++ {
		r := public.Post("/api/admin/auth/2fa/verify", verifyPayload(login2.ProvisionalToken, "000000"))
		switch r.Code {
		case consts.CodeTotpInvalid:
			sawInvalid = true
		case consts.Code2FATooManyAttempts:
			sawBlocked = true
		default:
			t.Fatalf("unexpected verify code=%d message=%q", r.Code, r.Message)
		}
		if sawBlocked {
			break
		}
	}
	if !sawInvalid || !sawBlocked {
		t.Fatalf("expected invalid (10048) then blocked (10056), got invalid=%v blocked=%v", sawInvalid, sawBlocked)
	}

	// ── 失败计数按用户维度：换新临时令牌仍被拦截 ──
	login3 := testinfra.LoginWithCreds(t, username, password)
	if !login3.TotpRequired {
		t.Fatal("expected totp_required on re-login")
	}
	public.Post("/api/admin/auth/2fa/verify", verifyPayload(login3.ProvisionalToken, "000000")).
		AssertError(t, consts.Code2FATooManyAttempts)

	// ── 窗口自愈：清失败计数（等价于 300s 固定窗口过期）──
	testinfra.DelRedisKey(t, fmt.Sprintf("2fa:fail:admin:%d", userID))

	// ── 恢复码登录成功，拿正式令牌 ──
	login4 := testinfra.LoginWithCreds(t, username, password)
	if !login4.TotpRequired {
		t.Fatal("expected totp_required on re-login after window reset")
	}
	verifyResp := public.Post("/api/admin/auth/2fa/verify", verifyPayload(login4.ProvisionalToken, enableData.BackupCodes[0]))
	verifyResp.AssertSuccess(t)
	var verifyData struct {
		AccessToken string `json:"access_token"`
	}
	verifyResp.DecodeData(t, &verifyData)
	if verifyData.AccessToken == "" {
		t.Fatal("verify should return access token")
	}

	// ── 用另一恢复码关闭 2FA ──
	userClient2 := public.WithToken(verifyData.AccessToken)
	disableResp := userClient2.Post("/api/admin/security/2fa/disable", map[string]any{
		"code": enableData.BackupCodes[1],
	})
	disableResp.AssertSuccess(t)

	// ── me 复核 totp_enabled=false ──
	meResp2 := userClient2.Get("/api/admin/auth/me", nil)
	meResp2.AssertSuccess(t)
	meResp2.DecodeData(t, &meData)
	if meData.TotpEnabled {
		t.Fatal("me should report totp_enabled=false after disable")
	}

	// ── enable 密码尝试计入失败闸门：错密码连发先 10023，达阈值后 10056，
	//    且正确密码也被拦——enable 是绕过账号锁定在线试密码的唯一入口 ──
	setupResp3 := userClient2.Post("/api/admin/security/2fa/setup", map[string]any{})
	setupResp3.AssertSuccess(t)
	var setupData3 struct {
		Secret string `json:"secret"`
	}
	setupResp3.DecodeData(t, &setupData3)
	sawPwdWrong, sawPwdBlocked := false, false
	for i := 0; i < 15; i++ {
		r := userClient2.Post("/api/admin/security/2fa/enable", map[string]any{
			"code":     "123456",
			"password": "definitely-wrong-password",
		})
		switch r.Code {
		case consts.CodeOldPasswordWrong: // enable 复用 10023「密码错误」
			sawPwdWrong = true
		case consts.Code2FATooManyAttempts:
			sawPwdBlocked = true
		default:
			t.Fatalf("unexpected enable code=%d message=%q", r.Code, r.Message)
		}
		if sawPwdBlocked {
			break
		}
	}
	if !sawPwdWrong || !sawPwdBlocked {
		t.Fatalf("expected password-wrong (10023) then blocked (10056) on enable, got wrong=%v blocked=%v", sawPwdWrong, sawPwdBlocked)
	}

	// 正确密码 + 有效码也必须被闸门拦截（按用户维度计数，非按请求维度）
	rightCode, err := totp.GenerateCode(setupData3.Secret, time.Now())
	if err != nil {
		t.Fatalf("generate totp code error: %v", err)
	}
	userClient2.Post("/api/admin/security/2fa/enable", map[string]any{
		"code":     rightCode,
		"password": password,
	}).AssertError(t, consts.Code2FATooManyAttempts)
}
