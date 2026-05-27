//go:build integration

package admin_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestLogin_Success(t *testing.T) {
	result := testinfra.Authenticate(t)

	if result.AccessToken == "" {
		t.Fatal("expected non-empty access_token")
	}
	if result.RefreshToken == "" {
		t.Fatal("expected non-empty refresh_token")
	}
	if result.ExpiresAt == "" {
		t.Fatal("expected non-empty expires_at")
	}
}

func TestLogin_InvalidPassword(t *testing.T) {
	client := testinfra.NewAPIClient(testinfra.DefaultBaseURL)

	// Step 1: Get captcha
	captchaResp := client.Get("/api/captcha/", nil)
	captchaResp.AssertSuccess(t)

	var captchaData struct {
		CaptchaKey string `json:"captcha_key"`
	}
	captchaResp.DecodeData(t, &captchaData)

	// Step 2: Read captcha answer from Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:         testinfra.DefaultRedisAddr,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
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

	// Step 3: Login with wrong password
	loginResp := client.Post("/api/admin/auth/login", map[string]any{
		"username":    testinfra.DefaultUsername,
		"password":    "WrongPassword123!",
		"captcha_key": captchaData.CaptchaKey,
		"captcha_x":   answer.X,
	})

	if loginResp.Code == 0 {
		t.Fatal("expected login to fail with wrong password, but got success")
	}
}

func TestLogout_Success(t *testing.T) {
	// Login
	result := testinfra.Authenticate(t)
	client := testinfra.NewAPIClient(testinfra.DefaultBaseURL).WithToken(result.AccessToken)

	// Logout
	logoutResp := client.Post("/api/admin/auth/logout", nil)
	logoutResp.AssertSuccess(t)

	// Verify token is invalidated by calling an auth-required endpoint
	sessionResp := client.Get("/api/admin/auth/sessions", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	sessionResp.AssertHTTPStatus(t, 401)
}

func TestTokenRefresh(t *testing.T) {
	// Login to get initial tokens
	result := testinfra.Authenticate(t)

	// Use refresh_token to get new tokens
	client := testinfra.NewAPIClient(testinfra.DefaultBaseURL)
	refreshResp := client.Post("/api/admin/auth/refresh", map[string]any{
		"refresh_token": result.RefreshToken,
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
	if refreshData.RefreshToken == "" {
		t.Fatal("expected non-empty refresh_token after refresh")
	}

	// Verify the new access_token works on an auth-required endpoint
	authedClient := testinfra.NewAPIClient(testinfra.DefaultBaseURL).WithToken(refreshData.AccessToken)
	sessionsResp := authedClient.Get("/api/admin/auth/sessions", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	sessionsResp.AssertSuccess(t)
}

func TestSessionList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/auth/sessions", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 1)
}

func TestChangePassword_Unauthorized(t *testing.T) {
	// Verify endpoint requires auth (no token sent)
	client := testinfra.NewAPIClient(testinfra.DefaultBaseURL)
	resp := client.Put("/api/admin/auth/change-password", map[string]any{
		"old_password": "anything",
		"new_password": "anything",
	})
	resp.AssertHTTPStatus(t, 401)
}
