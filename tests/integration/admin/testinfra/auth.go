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
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresAt        string `json:"expires_at"`
	TotpRequired     bool   `json:"totp_required"`
	ProvisionalToken string `json:"provisional_token"`
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
	result := LoginWithCreds(t, username, password)
	if result.TotpRequired {
		t.Fatal("admin user has 2FA enabled, cannot auto-login for tests")
	}
	return result
}

// LoginWithCreds 走完整登录流程并原样返回结果（含 totp_required 二段式分支），
// 供 2FA 生命周期等需要拿 provisional_token 的用例使用。
func LoginWithCreds(t *testing.T, username, password string) *LoginResult {
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

	return &loginResult
}

func GetAuthedClient(t *testing.T) *APIClient {
	t.Helper()
	result := Authenticate(t)
	return NewAPIClient(DefaultBaseURL).WithToken(result.AccessToken)
}

// DelRedisKey 删除指定 Redis 键。测试用它在受控前提下重置服务器状态
// （如清空 2FA 失败计数，等价于等待固定窗口过期自愈，避免测试等待 5 分钟）。
func DelRedisKey(t *testing.T, key string) {
	t.Helper()
	rdb := getRedisClient()
	defer rdb.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Del(ctx, key).Err(); err != nil {
		t.Fatalf("delete redis key %s error: %v", key, err)
	}
}
