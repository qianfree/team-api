package oauth

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	OpenAIClientID           = "app_EMoamEEZ73f0CkXaXp7hrann"
	OpenAIAuthorizeURL       = "https://auth.openai.com/oauth/authorize"
	OpenAITokenURL           = "https://auth.openai.com/oauth/token"
	OpenAIDefaultRedirectURI = "http://localhost:1455/auth/callback"
	OpenAIDefaultScopes      = "openid profile email offline_access"
	OpenAIRefreshScopes      = "openid profile email"
)

// openaiTokenResponse 表示 OpenAI OAuth token 端点的响应
type openaiTokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// openaiIDTokenClaims 表示 OpenAI ID token JWT payload 中的字段
type openaiIDTokenClaims struct {
	Email            string `json:"email"`
	ChatGPTAccountID string `json:"chatgpt_account_id"`
	ChatGPTUserID    string `json:"chatgpt_user_id"`
	ChatGPTPlanType  string `json:"chatgpt_plan_type"`
	Organizations    []struct {
		ID      string `json:"id"`
		Default bool   `json:"is_default"`
	} `json:"organizations"`
}

// OpenAIGenerateAuthURL 生成 OpenAI OAuth 授权 URL 和对应的会话 ID
func OpenAIGenerateAuthURL() (authURL string, sessionID string, err error) {
	state, err := GenerateState()
	if err != nil {
		return "", "", fmt.Errorf("生成 state 失败: %w", err)
	}

	codeVerifier, err := GenerateOpenAICodeVerifier()
	if err != nil {
		return "", "", fmt.Errorf("生成 code_verifier 失败: %w", err)
	}

	codeChallenge := GenerateCodeChallenge(codeVerifier)

	sessionID, err = GenerateSessionID()
	if err != nil {
		return "", "", fmt.Errorf("生成 session_id 失败: %w", err)
	}

	GlobalSessionStore.Set(sessionID, &OAuthSession{
		Platform:     "openai",
		State:        state,
		CodeVerifier: codeVerifier,
		Scope:        OpenAIDefaultScopes,
		RedirectURI:  OpenAIDefaultRedirectURI,
		CreatedAt:    time.Now(),
	})

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", OpenAIClientID)
	params.Set("redirect_uri", OpenAIDefaultRedirectURI)
	params.Set("scope", OpenAIDefaultScopes)
	params.Set("state", state)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("id_token_add_organizations", "true")
	params.Set("codex_cli_simplified_flow", "true")

	authURL = OpenAIAuthorizeURL + "?" + params.Encode()
	return authURL, sessionID, nil
}

// OpenAIExchangeCode 使用授权码换取 OAuth 凭证
func OpenAIExchangeCode(sessionID, code, state string) (*OAuthKeyData, error) {
	session, ok := GlobalSessionStore.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("无效的会话 ID: %s", sessionID)
	}

	// 使用常量时间比较防止计时攻击
	if subtle.ConstantTimeCompare([]byte(session.State), []byte(state)) != 1 {
		return nil, fmt.Errorf("state 不匹配")
	}

	defer GlobalSessionStore.Delete(sessionID)

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", OpenAIClientID)
	form.Set("code", code)
	form.Set("redirect_uri", OpenAIDefaultRedirectURI)
	form.Set("code_verifier", session.CodeVerifier)

	req, err := http.NewRequest(http.MethodPost, OpenAITokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建 token 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "codex-cli/0.91.0")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 token 端点失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 token 响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		g.Log().Warningf(nil, "OpenAI token 交换失败, status=%d, body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("token 交换失败 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp openaiTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析 token 响应失败: %w", err)
	}

	// 解析 ID token JWT payload 提取用户信息
	claims, err := parseOpenAIIDToken(tokenResp.IDToken)
	if err != nil {
		g.Log().Warningf(nil, "解析 OpenAI ID token 失败: %v", err)
		// ID token 解析失败不阻断流程，继续返回基本 token 数据
	}

	keyData := &OAuthKeyData{
		Platform:     "openai",
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Unix(),
		TokenType:    tokenResp.TokenType,
		Scope:        tokenResp.Scope,
	}

	if claims != nil {
		keyData.EmailAddress = claims.Email
		keyData.AccountID = claims.ChatGPTAccountID
		keyData.UserID = claims.ChatGPTUserID
		keyData.PlanType = claims.ChatGPTPlanType
		keyData.OrgID = pickOpenAIDefaultOrg(claims.Organizations)
	}

	return keyData, nil
}

// OpenAIRefreshToken 使用 refresh token 刷新 OAuth 凭证
func OpenAIRefreshToken(refreshToken string) (*OAuthKeyData, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", OpenAIClientID)
	form.Set("scope", OpenAIRefreshScopes)

	req, err := http.NewRequest(http.MethodPost, OpenAITokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建 refresh 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "codex-cli/0.91.0")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 refresh 端点失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 refresh 响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		g.Log().Warningf(nil, "OpenAI token 刷新失败, status=%d, body=%s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("token 刷新失败 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp openaiTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析 refresh 响应失败: %w", err)
	}

	// 刷新响应中可能包含新的 ID token
	claims, err := parseOpenAIIDToken(tokenResp.IDToken)
	if err != nil {
		g.Log().Warningf(nil, "解析 OpenAI ID token (refresh) 失败: %v", err)
	}

	keyData := &OAuthKeyData{
		Platform:     "openai",
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Unix(),
		TokenType:    tokenResp.TokenType,
		Scope:        tokenResp.Scope,
	}

	if claims != nil {
		keyData.EmailAddress = claims.Email
		keyData.AccountID = claims.ChatGPTAccountID
		keyData.UserID = claims.ChatGPTUserID
		keyData.PlanType = claims.ChatGPTPlanType
		keyData.OrgID = pickOpenAIDefaultOrg(claims.Organizations)
	}

	return keyData, nil
}

// parseOpenAIIDToken 解析 OpenAI ID token JWT 的 payload 部分
func parseOpenAIIDToken(idToken string) (*openaiIDTokenClaims, error) {
	if idToken == "" {
		return nil, fmt.Errorf("ID token 为空")
	}

	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("无效的 JWT 格式，期望 3 段，实际 %d 段", len(parts))
	}

	// Base64url 解码 payload（第二段）
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("解码 JWT payload 失败: %w", err)
	}

	var claims openaiIDTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("解析 JWT claims 失败: %w", err)
	}

	return &claims, nil
}

// pickOpenAIDefaultOrg 从组织列表中选择默认组织，优先选择 is_default=true 的组织
func pickOpenAIDefaultOrg(orgs []struct {
	ID      string `json:"id"`
	Default bool   `json:"is_default"`
}) string {
	if len(orgs) == 0 {
		return ""
	}
	for _, org := range orgs {
		if org.Default {
			return org.ID
		}
	}
	// 没有标记为默认的组织，返回第一个
	return orgs[0].ID
}
