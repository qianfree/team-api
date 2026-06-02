package tenant

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

const (
	oauthStatePrefix = "oauth:state:"
	oauthStateTTL    = 5 * time.Minute
)

// OAuthProvider defines the interface for OAuth providers.
type OAuthProvider interface {
	GetName() string
	GetAuthorizeURL(clientID, redirectURI, state string) string
	ExchangeToken(ctx context.Context, code string) (*OAuthToken, error)
	GetUserInfo(ctx context.Context, token string) (*OAuthUserInfo, error)
}

type OAuthToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type OAuthUserInfo struct {
	ProviderUserID string         `json:"provider_user_id"`
	Username       string         `json:"username"`
	DisplayName    string         `json:"display_name"`
	Email          string         `json:"email"`
	AvatarURL      string         `json:"avatar_url"`
	RawData        map[string]any `json:"raw_data"`
}

var providers = map[string]OAuthProvider{
	"github": &GitHubProvider{},
	"google": &GoogleProvider{},
}

func getOAuthSetting(ctx context.Context, key string) string {
	return common.Config().GetOption(ctx, key)
}

// GetOAuthAuthorizeURL 获取 OAuth 授权跳转 URL
func (s *sTenant) GetOAuthAuthorizeURL(ctx context.Context, req *v1.OAuthAuthorizeReq) (*v1.OAuthAuthorizeRes, error) {
	provider, ok := providers[req.Provider]
	if !ok {
		return nil, common.NewBusinessError(10059, "该 OAuth 供应商未启用")
	}

	enabled := getOAuthSetting(ctx, fmt.Sprintf("oauth_%s_enabled", req.Provider))
	if enabled != "true" {
		return nil, common.NewBusinessError(10059, "该 OAuth 供应商未启用")
	}

	clientID := getOAuthSetting(ctx, fmt.Sprintf("oauth_%s_client_id", req.Provider))
	if clientID == "" {
		return nil, common.NewBusinessError(10059, "该 OAuth 供应商未启用")
	}

	siteURL := getOAuthSetting(ctx, "site_url")
	if siteURL == "" {
		siteURL = "http://localhost:3000"
	}
	redirectURI := siteURL + fmt.Sprintf("/api/tenant/oauth/%s/callback", req.Provider)

	state := generateState()
	_, err := g.Redis().Set(ctx, oauthStatePrefix+state, req.Provider)
	if err != nil {
		return nil, err
	}
	_, _ = g.Redis().Expire(ctx, oauthStatePrefix+state, int64(oauthStateTTL.Seconds()))

	authorizeURL := provider.GetAuthorizeURL(clientID, redirectURI, state)
	return &v1.OAuthAuthorizeRes{
		AuthorizeUrl: authorizeURL,
		State:        state,
	}, nil
}

// OAuthCallback 处理 OAuth 回调
func (s *sTenant) OAuthCallback(ctx context.Context, req *v1.OAuthCallbackReq) (*v1.OAuthCallbackRes, error) {
	provider, ok := providers[req.Provider]
	if !ok {
		return nil, common.NewBusinessError(10059, "该 OAuth 供应商未启用")
	}

	// 原子验证并消费 state（Lua 脚本防止 TOCTOU 重放）
	val, err := g.Redis().Do(ctx, "EVAL", `
		local v = redis.call("GET", KEYS[1])
		if v then
			redis.call("DEL", KEYS[1])
			return v
		end
		return nil
	`, 1, oauthStatePrefix+req.State)
	if err != nil || val.IsNil() || val.String() != req.Provider {
		return nil, common.NewBusinessError(10060, "OAuth 授权码无效")
	}

	// 换取 token
	oauthToken, err := provider.ExchangeToken(ctx, req.Code)
	if err != nil {
		return nil, common.NewBusinessError(10061, "获取 OAuth 令牌失败")
	}

	// 获取用户信息
	userInfo, err := provider.GetUserInfo(ctx, oauthToken.AccessToken)
	if err != nil {
		return nil, common.NewBusinessError(10061, "获取 OAuth 令牌失败")
	}

	// 查找已绑定的身份
	var identity *struct {
		Id       int64 `json:"id"`
		TenantId int64 `json:"tenant_id"`
		UserId   int64 `json:"user_id"`
	}
	err = dao.TntOauthIdentities.Ctx(ctx).
		Where("provider", req.Provider).
		Where("provider_user_id", userInfo.ProviderUserID).
		Scan(&identity)
	if err != nil {
		return nil, err
	}

	isNewUser := false
	var tenantID int64
	var userID int64

	if identity != nil && identity.Id > 0 {
		tenantID = identity.TenantId
		userID = identity.UserId

		// 更新 token 信息
		_, err = dao.TntOauthIdentities.Ctx(ctx).
			Where("id", identity.Id).
			Data(do.TntOauthIdentities{
				AccessToken:      oauthToken.AccessToken,
				RefreshToken:     oauthToken.RefreshToken,
				TokenExpiresAt:   gtime.NewFromTime(time.Now().Add(time.Duration(oauthToken.ExpiresIn) * time.Second)),
				ProviderUsername: userInfo.Username,
			}).Update()
		if err != nil {
			return nil, err
		}
	} else {
		// 尝试自动注册
		autoRegister := getOAuthSetting(ctx, "oauth_auto_register")
		if autoRegister != "true" {
			return nil, common.NewBusinessError(10060, "OAuth 授权码无效或未绑定账号")
		}

		tenantCode := getOAuthSetting(ctx, "oauth_tenant_code")
		if tenantCode == "" {
			return nil, common.NewBusinessError(10060, "OAuth 自动注册未配置")
		}

		var tenant *struct {
			Id int64 `json:"id"`
		}
		err = dao.TntTenants.Ctx(ctx).
			Where("code", tenantCode).
			Where("status", "active").
			Scan(&tenant)
		if err != nil || tenant == nil {
			return nil, common.NewBusinessError(10060, "OAuth 自动注册未配置")
		}

		// 校验成员上限
		memberCount, _ := dao.TntUsers.Ctx(ctx).
			Where("tenant_id", tenant.Id).
			Where("status", "active").
			Count()
		effectiveMaxMembers, _, err := billing.GetTenantEffectiveLimits(ctx, tenant.Id)
		if err != nil {
			return nil, common.NewBusinessError(10060, "查询租户限制信息失败")
		}
		if memberCount >= effectiveMaxMembers {
			return nil, common.NewBusinessError(10060, "租户成员数已达上限")
		}

		username := userInfo.Username
		if username == "" {
			username = fmt.Sprintf("%s_%s", req.Provider, userInfo.ProviderUserID)
		}
		result, err := dao.TntUsers.Ctx(ctx).Data(do.TntUsers{
			TenantId:    tenant.Id,
			Username:    username,
			DisplayName: userInfo.DisplayName,
			Email:       userInfo.Email,
			Role:        "member",
			Status:      "active",
		}).Insert()
		if err != nil {
			username = fmt.Sprintf("%s_%s", username, randomHex(4))
			result, err = dao.TntUsers.Ctx(ctx).Data(do.TntUsers{
				TenantId:    tenant.Id,
				Username:    username,
				DisplayName: userInfo.DisplayName,
				Email:       userInfo.Email,
				Role:        "member",
				Status:      "active",
			}).Insert()
			if err != nil {
				return nil, err
			}
		}

		userID, _ = result.LastInsertId()
		tenantID = tenant.Id
		isNewUser = true

		// 创建 OAuth 身份绑定
		rawData, _ := json.Marshal(userInfo.RawData)
		_, err = dao.TntOauthIdentities.Ctx(ctx).Data(do.TntOauthIdentities{
			TenantId:         tenantID,
			UserId:           userID,
			Provider:         req.Provider,
			ProviderUserId:   userInfo.ProviderUserID,
			ProviderUsername: userInfo.Username,
			Email:            userInfo.Email,
			AvatarUrl:        userInfo.AvatarURL,
			AccessToken:      oauthToken.AccessToken,
			RefreshToken:     oauthToken.RefreshToken,
			TokenExpiresAt:   gtime.NewFromTime(time.Now().Add(time.Duration(oauthToken.ExpiresIn) * time.Second)),
			RawData:          string(rawData),
		}).Insert()
		if err != nil {
			return nil, err
		}
	}

	// 查找用户角色（先校验再创建 session）
	var user *struct {
		Role string `json:"role"`
	}
	err = dao.TntUsers.Ctx(ctx).Where("id", userID).Scan(&user)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewBusinessError(10060, "OAuth 登录失败：用户不存在")
	}

	// 创建 session
	refreshToken, err := common.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	refreshTokenHash := common.HashRefreshToken(refreshToken)
	r := g.RequestFromCtx(ctx)
	ipAddress := ""
	deviceInfo := `{"user_agent":"unknown"}`
	if r != nil {
		ipAddress = r.GetClientIp()
		ua := r.Header.Get("User-Agent")
		deviceInfo = fmt.Sprintf(`{"user_agent":"%s"}`, ua)
	}

	jti := common.GenerateJti()
	sessionID, err := common.CreateSession(ctx, "tenant", userID, tenantID, refreshTokenHash, ipAddress, deviceInfo, jti)
	if err != nil {
		return nil, err
	}

	// 生成 JWT token pair
	tokenPair, err := common.GenerateTokenPair(ctx, userID, "tenant", user.Role, tenantID, sessionID, jti)
	if err != nil {
		return nil, err
	}

	// 记录登录历史
	_ = common.RecordLoginHistory(ctx, "tenant", userID, tenantID, "sso", ipAddress, deviceInfo, "", true, "")

	return &v1.OAuthCallbackRes{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    tokenPair.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		IsNewUser:    isNewUser,
	}, nil
}

// LinkOAuth 绑定 OAuth 账号
func (s *sTenant) LinkOAuth(ctx context.Context, req *v1.OAuthLinkReq) (*v1.OAuthLinkRes, error) {
	provider, ok := providers[req.Provider]
	if !ok {
		return nil, common.NewBusinessError(10059, "该 OAuth 供应商未启用")
	}

	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)

	oauthToken, err := provider.ExchangeToken(ctx, req.Code)
	if err != nil {
		return nil, common.NewBusinessError(10061, "获取 OAuth 令牌失败")
	}

	userInfo, err := provider.GetUserInfo(ctx, oauthToken.AccessToken)
	if err != nil {
		return nil, common.NewBusinessError(10061, "获取 OAuth 令牌失败")
	}

	// 检查是否已被其他用户绑定
	var existing *struct {
		Id     int64 `json:"id"`
		UserId int64 `json:"user_id"`
	}
	err = dao.TntOauthIdentities.Ctx(ctx).
		Where("provider", req.Provider).
		Where("provider_user_id", userInfo.ProviderUserID).
		Scan(&existing)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.UserId != userID {
		return nil, common.NewBusinessError(10062, "该 OAuth 账号已绑定其他用户")
	}

	rawData, _ := json.Marshal(userInfo.RawData)
	if existing != nil && existing.Id > 0 {
		_, err = dao.TntOauthIdentities.Ctx(ctx).
			Where("id", existing.Id).
			Data(do.TntOauthIdentities{
				AccessToken:    oauthToken.AccessToken,
				RefreshToken:   oauthToken.RefreshToken,
				TokenExpiresAt: gtime.NewFromTime(time.Now().Add(time.Duration(oauthToken.ExpiresIn) * time.Second)),
				RawData:        string(rawData),
			}).Update()
		if err != nil {
			return nil, err
		}
	} else {
		_, err = dao.TntOauthIdentities.Ctx(ctx).Data(do.TntOauthIdentities{
			TenantId:         tenantID,
			UserId:           userID,
			Provider:         req.Provider,
			ProviderUserId:   userInfo.ProviderUserID,
			ProviderUsername: userInfo.Username,
			Email:            userInfo.Email,
			AvatarUrl:        userInfo.AvatarURL,
			AccessToken:      oauthToken.AccessToken,
			RefreshToken:     oauthToken.RefreshToken,
			TokenExpiresAt:   gtime.NewFromTime(time.Now().Add(time.Duration(oauthToken.ExpiresIn) * time.Second)),
			RawData:          string(rawData),
		}).Insert()
		if err != nil {
			return nil, err
		}
	}

	return &v1.OAuthLinkRes{}, nil
}

// UnlinkOAuth 解绑 OAuth 账号
func (s *sTenant) UnlinkOAuth(ctx context.Context, req *v1.OAuthUnlinkReq) (*v1.OAuthUnlinkRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)

	_, err := dao.TntOauthIdentities.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("user_id", userID).
		Where("provider", req.Provider).
		Delete()
	if err != nil {
		return nil, err
	}

	return &v1.OAuthUnlinkRes{}, nil
}

// ListOAuthProviders 获取已绑定的 OAuth 供应商列表
func (s *sTenant) ListOAuthProviders(ctx context.Context, req *v1.OAuthListProvidersReq) (*v1.OAuthListProvidersRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)

	type row struct {
		Provider         string `json:"provider" orm:"provider"`
		ProviderUserID   string `json:"provider_user_id" orm:"provider_user_id"`
		ProviderUsername string `json:"provider_username" orm:"provider_username"`
		CreatedAt        string `json:"created_at" orm:"created_at"`
	}

	rows := make([]row, 0)
	err := dao.TntOauthIdentities.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("user_id", userID).
		Scan(&rows)
	if err != nil {
		return nil, err
	}

	items := make([]v1.OAuthProviderItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, v1.OAuthProviderItem{
			Provider:         r.Provider,
			ProviderUserID:   r.ProviderUserID,
			ProviderUsername: r.ProviderUsername,
			LinkedAt:         r.CreatedAt,
		})
	}

	return &v1.OAuthListProvidersRes{List: items}, nil
}

func generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// === GitHub OAuth Provider ===

type GitHubProvider struct{}

func (p *GitHubProvider) GetName() string { return "github" }

func (p *GitHubProvider) GetAuthorizeURL(clientID, redirectURI, state string) string {
	return fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s&scope=read:user,user:email",
		clientID, redirectURI, state)
}

func (p *GitHubProvider) ExchangeToken(ctx context.Context, code string) (*OAuthToken, error) {
	bodyMap := map[string]string{
		"client_id":     getOAuthSetting(ctx, "oauth_github_client_id"),
		"client_secret": getOAuthSetting(ctx, "oauth_github_client_secret"),
		"code":          code,
	}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post("https://github.com/login/oauth/access_token", "application/json", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var token OAuthToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	if token.AccessToken == "" {
		return nil, fmt.Errorf("empty access token")
	}
	return &token, nil
}

func (p *GitHubProvider) GetUserInfo(ctx context.Context, token string) (*OAuthUserInfo, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var raw map[string]any
	json.Unmarshal(data, &raw)

	id := fmt.Sprintf("%v", raw["id"])
	login, _ := raw["login"].(string)
	name, _ := raw["name"].(string)
	email, _ := raw["email"].(string)
	avatar, _ := raw["avatar_url"].(string)
	if name == "" {
		name = login
	}

	return &OAuthUserInfo{
		ProviderUserID: id,
		Username:       login,
		DisplayName:    name,
		Email:          email,
		AvatarURL:      avatar,
		RawData:        raw,
	}, nil
}

// === Google OAuth Provider ===

type GoogleProvider struct{}

func (p *GoogleProvider) GetName() string { return "google" }

func (p *GoogleProvider) GetAuthorizeURL(clientID, redirectURI, state string) string {
	return fmt.Sprintf("https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=openid+email+profile&state=%s",
		clientID, redirectURI, state)
}

func (p *GoogleProvider) ExchangeToken(ctx context.Context, code string) (*OAuthToken, error) {
	v := url.Values{}
	v.Set("code", code)
	v.Set("client_id", getOAuthSetting(ctx, "oauth_google_client_id"))
	v.Set("client_secret", getOAuthSetting(ctx, "oauth_google_client_secret"))
	v.Set("redirect_uri", getOAuthSetting(ctx, "site_url"))
	v.Set("grant_type", "authorization_code")

	resp, err := http.Post("https://oauth2.googleapis.com/token", "application/x-www-form-urlencoded", strings.NewReader(v.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var token OAuthToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	if token.AccessToken == "" {
		return nil, fmt.Errorf("empty access token")
	}
	return &token, nil
}

func (p *GoogleProvider) GetUserInfo(ctx context.Context, token string) (*OAuthUserInfo, error) {
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var raw map[string]any
	json.Unmarshal(data, &raw)

	id, _ := raw["id"].(string)
	name, _ := raw["name"].(string)
	email, _ := raw["email"].(string)
	picture, _ := raw["picture"].(string)

	return &OAuthUserInfo{
		ProviderUserID: id,
		DisplayName:    name,
		Email:          email,
		AvatarURL:      picture,
		RawData:        raw,
	}, nil
}
