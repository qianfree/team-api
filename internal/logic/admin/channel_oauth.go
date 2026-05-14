package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common/oauth"
	relayLogic "github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/utility/crypto"
)

// 平台到渠道类型的映射
var platformToChannelType = map[string]int{
	"claude": 2, // ProviderClaude
	"openai": 1, // ProviderOpenAI
	"gemini": 3, // ProviderGemini
}

// 平台默认 base_url
var platformDefaultBaseURL = map[string]string{
	"claude": "https://api.anthropic.com",
	"openai": "https://api.openai.com",
	"gemini": "https://generativelanguage.googleapis.com",
}

// ChannelOAuthAuthURL 生成 OAuth 授权链接
func (s *sAdmin) ChannelOAuthAuthURL(ctx context.Context, req *v1.ChannelOAuthAuthURLReq) (*v1.ChannelOAuthAuthURLRes, error) {
	// Cookie 模式（仅 Claude）
	if req.AuthMode == "cookie" {
		if req.Platform != "claude" {
			return nil, gerror.New("Cookie 授权模式仅支持 Claude")
		}
		if req.SessionKey == "" {
			return nil, gerror.New("Cookie 模式需要提供 sessionKey")
		}

		scope := oauth.ClaudeScopeAPI
		if req.Scope == "inference" {
			scope = oauth.ClaudeScopeInference
		}

		oauthData, err := oauth.ClaudeCookieAuth(req.SessionKey, scope)
		if err != nil {
			return nil, gerror.Wrap(err, "Claude Cookie 授权失败")
		}

		// 自动创建渠道和密钥
		channelID, keyID, err := createOAuthChannelAndKey(ctx, oauthData, req.Platform, "OAuth 官方账号")
		if err != nil {
			return nil, err
		}

		return &v1.ChannelOAuthAuthURLRes{
			ChannelID: channelID,
			KeyID:     keyID,
		}, nil
	}

	// 浏览器模式
	var authURL, sessionID string
	var err error

	switch req.Platform {
	case "claude":
		scope := oauth.ClaudeScopeFull
		if req.Scope == "inference" {
			scope = oauth.ClaudeScopeInference
		}
		authURL, sessionID, err = oauth.ClaudeGenerateAuthURL(scope)
	case "openai":
		authURL, sessionID, err = oauth.OpenAIGenerateAuthURL()
	case "gemini":
		authURL, sessionID, err = oauth.GeminiGenerateAuthURL(req.OAuthType)
	default:
		return nil, gerror.Newf("不支持的平台: %s", req.Platform)
	}

	if err != nil {
		return nil, gerror.Wrap(err, "生成授权链接失败")
	}

	return &v1.ChannelOAuthAuthURLRes{
		AuthURL:   authURL,
		SessionID: sessionID,
	}, nil
}

// ChannelOAuthExchange OAuth 授权码换取令牌
func (s *sAdmin) ChannelOAuthExchange(ctx context.Context, req *v1.ChannelOAuthExchangeReq) (*v1.ChannelOAuthExchangeRes, error) {
	session, ok := oauth.GlobalSessionStore.Get(req.SessionID)
	if !ok {
		return nil, gerror.New("授权会话不存在或已过期")
	}

	var oauthData *oauth.OAuthKeyData
	var err error

	switch session.Platform {
	case "claude":
		oauthData, err = oauth.ClaudeExchangeCode(req.SessionID, req.Code)
	case "openai":
		oauthData, err = oauth.OpenAIExchangeCode(req.SessionID, req.Code, req.State)
	case "gemini":
		oauthData, err = oauth.GeminiExchangeCode(req.SessionID, req.Code)
	default:
		return nil, gerror.Newf("不支持的平台: %s", session.Platform)
	}

	if err != nil {
		return nil, gerror.Wrap(err, "授权码换取令牌失败")
	}

	// 删除会话
	oauth.GlobalSessionStore.Delete(req.SessionID)

	// 创建渠道和密钥
	channelID, keyID, err := createOAuthChannelAndKey(ctx, oauthData, session.Platform, req.KeyName)
	if err != nil {
		return nil, err
	}

	return &v1.ChannelOAuthExchangeRes{
		ChannelID: channelID,
		KeyID:     keyID,
	}, nil
}

// ChannelOAuthRefresh 手动刷新 OAuth 令牌
func (s *sAdmin) ChannelOAuthRefresh(ctx context.Context, req *v1.ChannelOAuthRefreshReq) (*v1.ChannelOAuthRefreshRes, error) {
	// 读取密钥记录
	var key *struct {
		ID           int64  `json:"id"`
		ChannelID    int64  `json:"channel_id"`
		EncryptedKey string `json:"encrypted_key"`
		KeyType      string `json:"key_type"`
	}

	err := dao.ChnChannelKeys.Ctx(ctx).
		Where("id", req.KeyID).
		Fields("id, channel_id, encrypted_key, key_type").
		Scan(&key)
	if err != nil {
		return nil, gerror.Wrap(err, "查询密钥失败")
	}
	if key == nil {
		return nil, gerror.New("密钥不存在")
	}
	if key.KeyType != "oauth" {
		return nil, gerror.New("该密钥不是 OAuth 类型")
	}

	// 解密并解析
	encKey := relayLogic.GetEncryptionKey()
	decrypted, err := crypto.DecryptString(encKey, key.EncryptedKey)
	if err != nil {
		return nil, gerror.Wrap(err, "解密密钥失败")
	}

	var oauthData oauth.OAuthKeyData
	if err := json.Unmarshal([]byte(decrypted), &oauthData); err != nil {
		return nil, gerror.Wrap(err, "解析 OAuth 凭证失败")
	}

	// 根据平台刷新
	var newToken *oauth.OAuthKeyData
	switch oauthData.Platform {
	case "claude":
		newToken, err = oauth.ClaudeRefreshToken(oauthData.RefreshToken)
	case "openai":
		newToken, err = oauth.OpenAIRefreshToken(oauthData.RefreshToken)
	case "gemini":
		newToken, err = oauth.GeminiRefreshToken(oauthData.RefreshToken)
	default:
		return nil, gerror.Newf("不支持的平台: %s", oauthData.Platform)
	}
	if err != nil {
		return nil, gerror.Wrap(err, "刷新令牌失败")
	}

	// 保留平台专属字段
	newToken.Platform = oauthData.Platform
	if newToken.RefreshToken == "" {
		newToken.RefreshToken = oauthData.RefreshToken
	}
	// 保留旧的非刷新返回字段
	if newToken.OrgUUID == "" {
		newToken.OrgUUID = oauthData.OrgUUID
	}
	if newToken.AccountUUID == "" {
		newToken.AccountUUID = oauthData.AccountUUID
	}
	if newToken.EmailAddress == "" {
		newToken.EmailAddress = oauthData.EmailAddress
	}
	if newToken.AccountID == "" {
		newToken.AccountID = oauthData.AccountID
	}
	if newToken.UserID == "" {
		newToken.UserID = oauthData.UserID
	}
	if newToken.OrgID == "" {
		newToken.OrgID = oauthData.OrgID
	}
	if newToken.ProjectID == "" {
		newToken.ProjectID = oauthData.ProjectID
	}
	if newToken.OAuthType == "" {
		newToken.OAuthType = oauthData.OAuthType
	}

	// 重新序列化 + 加密 + 更新
	jsonData, err := json.Marshal(newToken)
	if err != nil {
		return nil, gerror.Wrap(err, "序列化 OAuth 凭证失败")
	}

	encrypted, err := crypto.EncryptString(encKey, string(jsonData))
	if err != nil {
		return nil, gerror.Wrap(err, "加密 OAuth 凭证失败")
	}

	expiresAt := gtime.NewFromTimeStamp(newToken.ExpiresAt)
	_, err = dao.ChnChannelKeys.Ctx(ctx).
		Where("id", key.ID).
		Data(do.ChnChannelKeys{
			EncryptedKey:   encrypted,
			TokenExpiresAt: expiresAt,
		}).
		Update()
	if err != nil {
		return nil, gerror.Wrap(err, "更新 OAuth 凭证失败")
	}

	return &v1.ChannelOAuthRefreshRes{
		ExpiresAt: expiresAt.String(),
	}, nil
}

// createOAuthChannelAndKey 自动创建 OAuth 渠道和密钥
func createOAuthChannelAndKey(ctx context.Context, oauthData *oauth.OAuthKeyData, platform, keyName string) (channelID, keyID int64, err error) {
	channelType, ok := platformToChannelType[platform]
	if !ok {
		return 0, 0, gerror.Newf("不支持的平台: %s", platform)
	}

	// 渠道名称：平台 - 邮箱
	name := fmt.Sprintf("%s - %s", platformTitle(platform), oauthData.EmailAddress)
	if oauthData.EmailAddress == "" {
		name = fmt.Sprintf("%s - OAuth", platformTitle(platform))
	}

	baseURL := platformDefaultBaseURL[platform]

	// 创建渠道
	channelID, err = dao.ChnChannels.Ctx(ctx).InsertAndGetId(do.ChnChannels{
		Name:     name,
		Type:     channelType,
		BaseUrl:  baseURL,
		Status:   "active",
		Priority: 10,
		Weight:   100,
	})
	if err != nil {
		return 0, 0, gerror.Wrap(err, "创建渠道失败")
	}

	// 序列化 + 加密 OAuth 凭证
	encKey := relayLogic.GetEncryptionKey()
	jsonData, err := json.Marshal(oauthData)
	if err != nil {
		return 0, 0, gerror.Wrap(err, "序列化 OAuth 凭证失败")
	}

	encrypted, err := crypto.EncryptString(encKey, string(jsonData))
	if err != nil {
		return 0, 0, gerror.Wrap(err, "加密 OAuth 凭证失败")
	}

	// 计算过期时间
	expiresAt := gtime.NewFromTimeStamp(oauthData.ExpiresAt)

	// 创建密钥
	keyID, err = dao.ChnChannelKeys.Ctx(ctx).InsertAndGetId(do.ChnChannelKeys{
		ChannelId:      channelID,
		Name:           keyName,
		EncryptedKey:   encrypted,
		Status:         "active",
		KeyType:        "oauth",
		TokenExpiresAt: expiresAt,
	})
	if err != nil {
		return 0, 0, gerror.Wrap(err, "创建密钥失败")
	}

	g.Log().Infof(ctx, "[OAuth] 创建 OAuth 渠道: platform=%s, channel_id=%d, key_id=%d, email=%s",
		platform, channelID, keyID, oauthData.EmailAddress)

	return channelID, keyID, nil
}

// platformTitle 平台名称首字母大写
func platformTitle(platform string) string {
	switch platform {
	case "claude":
		return "Claude"
	case "openai":
		return "OpenAI"
	case "gemini":
		return "Gemini"
	default:
		return platform
	}
}

// Ensure imports compile - use time for potential future use
var _ = time.Time{}
