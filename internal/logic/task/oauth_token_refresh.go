package task

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common/oauth"
	"github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/utility/crypto"
)

// RefreshExpiringOAuthTokens 刷新即将过期的 OAuth 令牌
func RefreshExpiringOAuthTokens(ctx context.Context) error {
	var keys []struct {
		ID             int64       `json:"id"`
		EncryptedKey   string      `json:"encrypted_key"`
		TokenExpiresAt *gtime.Time `json:"token_expires_at"`
	}

	// 查询 30 分钟内即将过期的 OAuth 密钥
	err := dao.ChnChannelKeys.Ctx(ctx).
		Where("key_type", "oauth").
		Where("status", "active").
		Where("token_expires_at < ?", gtime.Now().Add(30*time.Minute)).
		Where("token_expires_at > ?", gtime.Now().Add(-1*time.Hour)).
		Fields("id, encrypted_key, token_expires_at").
		Scan(&keys)
	if err != nil {
		return err
	}

	if len(keys) == 0 {
		return nil
	}

	g.Log().Infof(ctx, "[OAuthRefresh] Found %d tokens to refresh", len(keys))

	encKey := relay.GetEncryptionKey()
	refreshed := 0
	failed := 0

	for _, key := range keys {
		decrypted, err := crypto.DecryptString(encKey, key.EncryptedKey)
		if err != nil {
			g.Log().Warningf(ctx, "[OAuthRefresh] Decrypt failed for key %d: %v", key.ID, err)
			continue
		}

		var oauthData oauth.OAuthKeyData
		if err := json.Unmarshal([]byte(decrypted), &oauthData); err != nil {
			g.Log().Warningf(ctx, "[OAuthRefresh] Parse failed for key %d: %v", key.ID, err)
			continue
		}

		var newToken *oauth.OAuthKeyData
		switch oauthData.Platform {
		case "claude":
			newToken, err = oauth.ClaudeRefreshToken(oauthData.RefreshToken)
		case "openai":
			newToken, err = oauth.OpenAIRefreshToken(oauthData.RefreshToken)
		case "gemini":
			newToken, err = oauth.GeminiRefreshToken(oauthData.RefreshToken)
		default:
			g.Log().Warningf(ctx, "[OAuthRefresh] Unknown platform %s for key %d", oauthData.Platform, key.ID)
			continue
		}

		if err != nil {
			failed++
			g.Log().Warningf(ctx, "[OAuthRefresh] Refresh failed for key %d (%s): %v", key.ID, oauthData.Platform, err)
			continue
		}

		// 保留平台专属字段
		newToken.Platform = oauthData.Platform
		if newToken.RefreshToken == "" {
			newToken.RefreshToken = oauthData.RefreshToken
		}
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

		jsonData, err := json.Marshal(newToken)
		if err != nil {
			g.Log().Warningf(ctx, "[OAuthRefresh] Marshal failed for key %d: %v", key.ID, err)
			continue
		}

		encrypted, err := crypto.EncryptString(encKey, string(jsonData))
		if err != nil {
			g.Log().Warningf(ctx, "[OAuthRefresh] Encrypt failed for key %d: %v", key.ID, err)
			continue
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
			g.Log().Warningf(ctx, "[OAuthRefresh] Update failed for key %d: %v", key.ID, err)
			continue
		}

		refreshed++
	}

	g.Log().Infof(ctx, "[OAuthRefresh] Completed: refreshed=%d, failed=%d", refreshed, failed)
	return nil
}
