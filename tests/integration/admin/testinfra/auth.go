//go:build integration

package testinfra

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func getRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         GetRedisAddr(),
		DB:           0,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
}

type LoginResult struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
	TotpRequired bool   `json:"totp_required"`
}

func Authenticate(t *testing.T) *LoginResult {
	t.Helper()
	if DefaultUsername == "" || DefaultPassword == "" {
		t.Fatal("TEST_ADMIN_USERNAME and TEST_ADMIN_PASSWORD environment variables must be set")
	}
	return AuthenticateWithCreds(t, DefaultUsername, DefaultPassword)
}

func AuthenticateWithCreds(t *testing.T, username, password string) *LoginResult {
	t.Helper()
	client := NewAPIClient(DefaultBaseURL)

	// Step 1: Get captcha
	captchaResp := client.Get("/api/captcha/", nil)
	captchaResp.AssertSuccess(t)

	var captchaData struct {
		CaptchaKey string `json:"captcha_key"`
	}
	captchaResp.DecodeData(t, &captchaData)

	// Step 2: Read answer from Redis
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

	// Step 3: Login with the answer
	loginResp := client.Post("/api/admin/auth/login", map[string]any{
		"username":    username,
		"password":    password,
		"captcha_key": captchaData.CaptchaKey,
		"captcha_x":   answer.X,
	})
	loginResp.AssertSuccess(t)

	var loginResult LoginResult
	loginResp.DecodeData(t, &loginResult)

	if loginResult.TotpRequired {
		t.Fatal("admin user has 2FA enabled, cannot auto-login for tests")
	}

	return &loginResult
}

func GetAuthedClient(t *testing.T) *APIClient {
	t.Helper()
	result := Authenticate(t)
	return NewAPIClient(DefaultBaseURL).WithToken(result.AccessToken)
}
