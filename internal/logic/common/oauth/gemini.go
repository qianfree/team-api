package oauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

const (
	GeminiCLIClientID      = "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com"
	GeminiCLIClientSecret  = "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"
	GeminiAuthorizeURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	GeminiTokenURL         = "https://oauth2.googleapis.com/token"
	GeminiCLIRedirectURI   = "https://codeassist.google.com/authcode"
	GeminiCodeAssistScopes = "https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile"

	// geminiSafetyWindowSeconds 安全窗口时间，提前 5 分钟认为 token 过期
	geminiSafetyWindowSeconds int64 = 300
	// geminiMinTTLSeconds token 最小有效期
	geminiMinTTLSeconds int64 = 30
)

// googleTokenResponse Google OAuth 令牌响应
type googleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
}

// geminiCodeAssistResponse loadCodeAssist API 响应
type geminiCodeAssistResponse struct {
	CloudAICompanionProject string `json:"cloudaicompanionProject"`
}

// geminiUserInfoResponse Google userinfo 响应
type geminiUserInfoResponse struct {
	Email string `json:"email"`
}

// GeminiGenerateAuthURL 生成 Gemini OAuth 授权跳转 URL
// oauthType: "code_assist"（默认，使用内置 Gemini CLI 客户端）
func GeminiGenerateAuthURL(oauthType string) (authURL string, sessionID string, err error) {
	if oauthType == "" {
		oauthType = "code_assist"
	}

	// 生成 PKCE 值
	state, err := GenerateState()
	if err != nil {
		return "", "", fmt.Errorf("生成 state 失败: %w", err)
	}

	verifier, err := GenerateCodeVerifier()
	if err != nil {
		return "", "", fmt.Errorf("生成 code_verifier 失败: %w", err)
	}

	challenge := GenerateCodeChallenge(verifier)

	// 生成会话 ID
	sessionID, err = GenerateSessionID()
	if err != nil {
		return "", "", fmt.Errorf("生成 session_id 失败: %w", err)
	}

	// 存储会话
	session := &OAuthSession{
		Platform:     "gemini",
		State:        state,
		CodeVerifier: verifier,
		Scope:        GeminiCodeAssistScopes,
		RedirectURI:  GeminiCLIRedirectURI,
		Extra:        oauthType,
		CreatedAt:    time.Now(),
	}
	GlobalSessionStore.Set(sessionID, session)

	// 构建授权 URL
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", GeminiCLIClientID)
	params.Set("redirect_uri", GeminiCLIRedirectURI)
	params.Set("scope", GeminiCodeAssistScopes)
	params.Set("state", state)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")
	params.Set("include_granted_scopes", "true")

	authURL = fmt.Sprintf("%s?%s", GeminiAuthorizeURL, params.Encode())
	return authURL, sessionID, nil
}

// GeminiExchangeCode 用授权码换取 OAuth 令牌
func GeminiExchangeCode(sessionID, code string) (*OAuthKeyData, error) {
	// 获取会话
	session, ok := GlobalSessionStore.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("OAuth 会话不存在或已过期")
	}
	defer GlobalSessionStore.Delete(sessionID)

	// 验证平台
	if session.Platform != "gemini" {
		return nil, fmt.Errorf("无效的 OAuth 平台: %s", session.Platform)
	}

	oauthType := session.Extra
	if oauthType == "" {
		oauthType = "code_assist"
	}

	// 构建令牌请求
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", GeminiCLIClientID)
	data.Set("client_secret", GeminiCLIClientSecret)
	data.Set("code", code)
	data.Set("code_verifier", session.CodeVerifier)
	data.Set("redirect_uri", session.RedirectURI)

	req, err := http.NewRequest(http.MethodPost, GeminiTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建令牌请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Google 令牌失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取令牌响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google 令牌请求失败 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp googleTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析令牌响应失败: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("Google 返回空的 access_token")
	}

	// 获取 project_id（仅 code_assist 类型）
	var projectID string
	if oauthType == "code_assist" {
		projectID, err = geminiLoadCodeAssist(tokenResp.AccessToken)
		if err != nil {
			g.Log().Warningf(gctx.New(), "Gemini loadCodeAssist 失败: %v", err)
			// 不阻断流程，projectID 留空
		}
	}

	// 获取用户邮箱
	email, err := geminiGetUserInfo(tokenResp.AccessToken)
	if err != nil {
		g.Log().Warningf(gctx.New(), "Gemini 获取用户信息失败: %v", err)
	}

	// 计算过期时间，带安全窗口
	expiresIn := tokenResp.ExpiresIn - geminiSafetyWindowSeconds
	if expiresIn < geminiMinTTLSeconds {
		expiresIn = geminiMinTTLSeconds
	}
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second).Unix()

	keyData := &OAuthKeyData{
		Platform:     "gemini",
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    expiresAt,
		TokenType:    tokenResp.TokenType,
		Scope:        tokenResp.Scope,
		ProjectID:    projectID,
		OAuthType:    oauthType,
		EmailAddress: email,
	}

	return keyData, nil
}

// GeminiRefreshToken 使用 refresh_token 刷新访问令牌
// 内置最多 3 次重试，使用指数退避处理瞬时错误
func GeminiRefreshToken(refreshToken string) (*OAuthKeyData, error) {
	if refreshToken == "" {
		return nil, fmt.Errorf("refresh_token 不能为空")
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", GeminiCLIClientID)
	data.Set("client_secret", GeminiCLIClientSecret)

	var lastErr error
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// 指数退避: 1s, 2s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			time.Sleep(backoff)
		}

		req, err := http.NewRequest(http.MethodPost, GeminiTokenURL, strings.NewReader(data.Encode()))
		if err != nil {
			lastErr = fmt.Errorf("创建刷新请求失败: %w", err)
			continue
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := GetHTTPClient().Do(req)
		if err != nil {
			lastErr = fmt.Errorf("请求 Google 令牌失败: %w", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("读取令牌响应失败: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			// 检查是否为不可重试的错误
			errMsg := string(body)
			if strings.Contains(errMsg, "invalid_grant") ||
				strings.Contains(errMsg, "invalid_client") ||
				strings.Contains(errMsg, "unauthorized_client") {
				return nil, fmt.Errorf("Google 令牌刷新失败（不可重试）: %s", errMsg)
			}
			// 瞬时错误，可重试
			lastErr = fmt.Errorf("Google 令牌刷新失败 (HTTP %d): %s", resp.StatusCode, errMsg)
			continue
		}

		var tokenResp googleTokenResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			return nil, fmt.Errorf("解析令牌响应失败: %w", err)
		}

		if tokenResp.AccessToken == "" {
			return nil, fmt.Errorf("Google 返回空的 access_token")
		}

		// 计算过期时间，带安全窗口
		expiresIn := tokenResp.ExpiresIn - geminiSafetyWindowSeconds
		if expiresIn < geminiMinTTLSeconds {
			expiresIn = geminiMinTTLSeconds
		}
		expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second).Unix()

		keyData := &OAuthKeyData{
			Platform:     "gemini",
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			ExpiresAt:    expiresAt,
			TokenType:    tokenResp.TokenType,
			Scope:        tokenResp.Scope,
		}

		// 如果刷新时没有返回新的 refresh_token，保留原来的
		if keyData.RefreshToken == "" {
			keyData.RefreshToken = refreshToken
		}

		return keyData, nil
	}

	return nil, fmt.Errorf("Google 令牌刷新失败（已重试 %d 次）: %w", maxRetries, lastErr)
}

// geminiLoadCodeAssist 调用 Google Code Assist API 获取 GCP 项目 ID
func geminiLoadCodeAssist(accessToken string) (projectID string, err error) {
	reqBody := `{"metadata":{"ideType":"ANTIGRAVITY","ideVersion":"1.20.6","ideName":"antigravity"}}`

	req, err := http.NewRequest("POST", "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist", strings.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("创建 loadCodeAssist 请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GeminiCLI/0.1.5 (Windows; AMD64)")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 loadCodeAssist 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 loadCodeAssist 响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("loadCodeAssist 请求失败 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result geminiCodeAssistResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析 loadCodeAssist 响应失败: %w", err)
	}

	if result.CloudAICompanionProject == "" {
		return "", fmt.Errorf("loadCodeAssist 响应中缺少 project ID")
	}

	return result.CloudAICompanionProject, nil
}

// geminiGetUserInfo 获取 Google 用户信息（邮箱）
func geminiGetUserInfo(accessToken string) (email string, err error) {
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return "", fmt.Errorf("创建 userinfo 请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 userinfo 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 userinfo 响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("userinfo 请求失败 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result geminiUserInfoResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析 userinfo 响应失败: %w", err)
	}

	return result.Email, nil
}
