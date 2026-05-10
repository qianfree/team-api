package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Claude OAuth 常量
const (
	ClaudeClientID     = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	ClaudeAuthorizeURL = "https://claude.ai/oauth/authorize"
	ClaudeTokenURL     = "https://platform.claude.com/v1/oauth/token"
	ClaudeRedirectURI  = "https://platform.claude.com/oauth/code/callback"

	ClaudeScopeFull      = "org:create_api_key user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload"
	ClaudeScopeAPI       = "user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload"
	ClaudeScopeInference = "user:inference"
)

// claudeTokenResponse Claude OAuth token 响应（包含组织和账户信息）
type claudeTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	Organization *struct {
		UUID string `json:"uuid"`
	} `json:"organization,omitempty"`
	Account *struct {
		UUID         string `json:"uuid"`
		EmailAddress string `json:"email_address"`
	} `json:"account,omitempty"`
}

// claudeCookieOrg GET /api/organizations 返回的组织条目
type claudeCookieOrg struct {
	UUID      string `json:"uuid"`
	RavenType string `json:"raven_type"`
}

// claudeAuthorizeResponse POST /v1/oauth/{orgUUID}/authorize 的响应
type claudeAuthorizeResponse struct {
	RedirectURI string `json:"redirect_uri"`
}

// ClaudeGenerateAuthURL 生成 Claude OAuth 授权 URL（PKCE 流程）
func ClaudeGenerateAuthURL(scope string) (authURL string, sessionID string, err error) {
	state, err := GenerateState()
	if err != nil {
		return "", "", fmt.Errorf("generate state: %w", err)
	}

	codeVerifier, err := GenerateCodeVerifier()
	if err != nil {
		return "", "", fmt.Errorf("generate code verifier: %w", err)
	}

	codeChallenge := GenerateCodeChallenge(codeVerifier)

	sessionID, err = GenerateSessionID()
	if err != nil {
		return "", "", fmt.Errorf("generate session id: %w", err)
	}

	GlobalSessionStore.Set(sessionID, &OAuthSession{
		Platform:     "claude",
		State:        state,
		CodeVerifier: codeVerifier,
		Scope:        scope,
		RedirectURI:  ClaudeRedirectURI,
		CreatedAt:    time.Now(),
	})

	params := url.Values{}
	params.Set("code", "true")
	params.Set("client_id", ClaudeClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", ClaudeRedirectURI)
	params.Set("scope", scope)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)

	authURL = ClaudeAuthorizeURL + "?" + params.Encode()
	return authURL, sessionID, nil
}

// ClaudeExchangeCode 用授权码换取 Claude OAuth 令牌
func ClaudeExchangeCode(sessionID, code string) (*OAuthKeyData, error) {
	session, ok := GlobalSessionStore.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("oauth session not found or expired")
	}
	defer GlobalSessionStore.Delete(sessionID)

	// code 可能包含 state：格式 "authCode#state"
	state := session.State
	if idx := strings.Index(code, "#"); idx >= 0 {
		state = code[idx+1:]
		code = code[:idx]
	}

	// 构建请求体
	reqBody := map[string]interface{}{
		"code":          code,
		"grant_type":    "authorization_code",
		"client_id":     ClaudeClientID,
		"redirect_uri":  session.RedirectURI,
		"code_verifier": session.CodeVerifier,
		"state":         state,
	}

	// setup token（仅 inference 权限）设置较长过期时间
	if session.Scope == ClaudeScopeInference {
		reqBody["expires_in"] = int64(31536000)
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	respData, err := doPost(ClaudeTokenURL, bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	var tokenResp claudeTokenResponse
	if err := json.Unmarshal(respData, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	keyData := buildClaudeKeyData(&tokenResp)
	return keyData, nil
}

// ClaudeCookieAuth 使用 Cookie 方式自动完成 Claude OAuth 认证（4 步流程）
func ClaudeCookieAuth(sessionKey, scope string) (*OAuthKeyData, error) {
	client := GetHTTPClient()
	ctx := context.Background()

	// Step 1: 获取组织列表，选择 team 类型组织
	orgUUID, err := claudeSelectOrg(client, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("select organization: %w", err)
	}

	// Step 2: 生成 PKCE 参数
	state, err := GenerateState()
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	codeVerifier, err := GenerateCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("generate code verifier: %w", err)
	}

	codeChallenge := GenerateCodeChallenge(codeVerifier)

	// Step 3: 向组织发起 OAuth 授权请求，获取 code
	code, err := claudeAuthorizeOrg(client, sessionKey, orgUUID, scope, state, codeChallenge)
	if err != nil {
		return nil, fmt.Errorf("authorize with org: %w", err)
	}

	// Step 4: 用 code 换取 token
	reqBody := map[string]interface{}{
		"code":          code,
		"grant_type":    "authorization_code",
		"client_id":     ClaudeClientID,
		"redirect_uri":  ClaudeRedirectURI,
		"code_verifier": codeVerifier,
		"state":         state,
	}

	if scope == ClaudeScopeInference {
		reqBody["expires_in"] = int64(31536000)
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	respData, err := doPost(ClaudeTokenURL, bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	var tokenResp claudeTokenResponse
	if err := json.Unmarshal(respData, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	g.Log().Infof(ctx, "claude cookie auth succeeded, scope=%s", tokenResp.Scope)

	keyData := buildClaudeKeyData(&tokenResp)
	return keyData, nil
}

// ClaudeRefreshToken 使用 refresh_token 刷新 Claude OAuth 令牌
func ClaudeRefreshToken(refreshToken string) (*OAuthKeyData, error) {
	reqBody := map[string]interface{}{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     ClaudeClientID,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	respData, err := doPost(ClaudeTokenURL, bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}

	var tokenResp claudeTokenResponse
	if err := json.Unmarshal(respData, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	keyData := buildClaudeKeyData(&tokenResp)
	return keyData, nil
}

// claudeSelectOrg 通过 Cookie 获取组织列表并选择合适的组织（优先 team）
func claudeSelectOrg(client *http.Client, sessionKey string) (string, error) {
	req, err := http.NewRequest("GET", "https://claude.ai/api/organizations", nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Cookie", "sessionKey="+sessionKey)
	req.Header.Set("User-Agent", "axios/1.13.6")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request organizations: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("organizations request failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var orgs []claudeCookieOrg
	if err := json.Unmarshal(body, &orgs); err != nil {
		return "", fmt.Errorf("parse organizations: %w", err)
	}

	if len(orgs) == 0 {
		return "", fmt.Errorf("no organizations found")
	}

	// 优先选择 team 类型组织，否则取第一个
	for _, org := range orgs {
		if org.RavenType == "team" {
			return org.UUID, nil
		}
	}

	return orgs[0].UUID, nil
}

// claudeAuthorizeOrg 向指定组织发起 OAuth 授权请求，从 redirect_uri 中提取 code
func claudeAuthorizeOrg(client *http.Client, sessionKey, orgUUID, scope, state, codeChallenge string) (string, error) {
	reqBody := map[string]interface{}{
		"response_type":         "code",
		"client_id":             ClaudeClientID,
		"organization_uuid":     orgUUID,
		"redirect_uri":          ClaudeRedirectURI,
		"scope":                 scope,
		"state":                 state,
		"code_challenge":        codeChallenge,
		"code_challenge_method": "S256",
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequest("POST", "https://claude.ai/v1/oauth/"+orgUUID+"/authorize", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Cookie", "sessionKey="+sessionKey)
	req.Header.Set("User-Agent", "axios/1.13.6")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("authorize request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("authorize request failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var authResp claudeAuthorizeResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return "", fmt.Errorf("parse authorize response: %w", err)
	}

	if authResp.RedirectURI == "" {
		return "", fmt.Errorf("authorize response missing redirect_uri")
	}

	// 从 redirect_uri 中提取 code 参数
	redirectURL, err := url.Parse(authResp.RedirectURI)
	if err != nil {
		return "", fmt.Errorf("parse redirect_uri: %w", err)
	}

	code := redirectURL.Query().Get("code")
	if code == "" {
		return "", fmt.Errorf("redirect_uri missing code parameter: %s", authResp.RedirectURI)
	}

	return code, nil
}

// doPost 发送 JSON POST 请求并返回响应体
func doPost(endpoint string, body []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "axios/1.13.6")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// buildClaudeKeyData 从 token 响应构建 OAuthKeyData
func buildClaudeKeyData(resp *claudeTokenResponse) *OAuthKeyData {
	keyData := &OAuthKeyData{
		Platform:     "claude",
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    time.Now().Unix() + resp.ExpiresIn,
		TokenType:    resp.TokenType,
		Scope:        resp.Scope,
	}

	if resp.Organization != nil {
		keyData.OrgUUID = resp.Organization.UUID
	}

	if resp.Account != nil {
		keyData.AccountUUID = resp.Account.UUID
		keyData.EmailAddress = resp.Account.EmailAddress
	}

	return keyData
}
