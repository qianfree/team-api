//go:build integration

package testinfra

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	admintest "github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func RandomSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func getRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         DefaultRedisAddr,
		DB:           0,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
}

func SolveCaptcha(t *testing.T) (captchaKey string, captchaX int) {
	t.Helper()
	client := admintest.NewAPIClient(DefaultBaseURL)

	captchaResp := client.Get("/api/captcha/", nil)
	captchaResp.AssertSuccess(t)

	var captchaData struct {
		CaptchaKey string `json:"captcha_key"`
	}
	captchaResp.DecodeData(t, &captchaData)

	rdb := getRedisClient()
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	redisKey := fmt.Sprintf("captcha:state:%s", captchaData.CaptchaKey)
	val, err := rdb.Get(ctx, redisKey).Result()
	if err != nil {
		t.Fatalf("read captcha from redis error: %v", err)
	}

	var answer struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	if err := json.Unmarshal([]byte(val), &answer); err != nil {
		t.Fatalf("unmarshal captcha answer error: %v", err)
	}

	return captchaData.CaptchaKey, answer.X
}

type TenantRegisterResult struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
	Tenant       struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Code string `json:"code"`
	} `json:"tenant"`
	User struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
	} `json:"user"`

	// Stored for re-login tests
	TenantCode string
	Username   string
	Password   string
	Email      string
}

// deleteTenantViaDB hard-deletes a tenant and all associated data from the database.
// Used as t.Cleanup callback — logs errors instead of failing the test.
func deleteTenantViaDB(t *testing.T, tenantID int64) {
	HardDeleteTenant(t, tenantID)
}

func RegisterTestTenant(t *testing.T) *TenantRegisterResult {
	t.Helper()

	suffix := RandomSuffix()
	tenantCode := fmt.Sprintf("t-%s", suffix)
	username := fmt.Sprintf("owner%s", suffix)
	email := fmt.Sprintf("%s@test.com", username)

	captchaKey, captchaX := SolveCaptcha(t)

	client := admintest.NewAPIClient(DefaultBaseURL)
	resp := client.Post("/api/tenant/auth/register", map[string]any{
		"email":       email,
		"password":    TestPassword,
		"tenant_name": fmt.Sprintf("测试租户 %s", suffix),
		"tenant_code": tenantCode,
		"username":    username,
		"captcha_key": captchaKey,
		"captcha_x":   captchaX,
	})
	resp.AssertSuccess(t)

	var result TenantRegisterResult
	resp.DecodeData(t, &result)
	result.TenantCode = tenantCode
	result.Username = username
	result.Password = TestPassword
	result.Email = email

	// Automatically hard-delete the tenant when the test completes
	t.Cleanup(func() {
		deleteTenantViaDB(t, result.Tenant.ID)
	})

	return &result
}

func GetAuthedClient(t *testing.T) (*admintest.APIClient, *TenantRegisterResult) {
	t.Helper()
	result := RegisterTestTenant(t)
	client := admintest.NewAPIClient(DefaultBaseURL).WithToken(result.AccessToken)
	return client, result
}

func LoginTenant(t *testing.T, username, tenantCode, password string) *TenantRegisterResult {
	t.Helper()
	captchaKey, captchaX := SolveCaptcha(t)

	client := admintest.NewAPIClient(DefaultBaseURL)
	account := fmt.Sprintf("%s@%s", username, tenantCode)
	resp := client.Post("/api/tenant/auth/login", map[string]any{
		"account":     account,
		"password":    password,
		"type":        "ram",
		"captcha_key": captchaKey,
		"captcha_x":   captchaX,
	})
	resp.AssertSuccess(t)

	var result TenantRegisterResult
	resp.DecodeData(t, &result)
	result.TenantCode = tenantCode
	result.Username = username
	result.Password = password

	return &result
}
