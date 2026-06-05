package relay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/qianfree/team-api/internal/dao"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/grand"

	"github.com/qianfree/team-api/internal/consts"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	uc "github.com/qianfree/team-api/internal/utility/crypto"
	"github.com/qianfree/team-api/relay/common"
)

// ApiKeyInfo API Key 验证结果（本地定义，避免导入 relay/common）
type ApiKeyInfo struct {
	ID        int64
	TenantID  int64
	UserID    int64
	Scope     string
	Status    string
	KeyType   string // personal 或 project
	ProjectID int64  // 项目密钥关联的项目 ID，个人密钥为 0
	KeyHash   string // SHA-256(rawKey)，缓存命中时用于全键校验
}

// apiKeyCache API Key 缓存实例（TTL 60s）
var apiKeyCache = lcommon.NewCache("apikey", 60*time.Second)

// DefaultChannelSettings 返回默认渠道配置
func DefaultChannelSettings() common.ChannelSettings {
	return common.ChannelSettings{
		TimeoutSeconds: 60,
		RetryCount:     1,
	}
}

// GetEncryptionKey 获取加密密钥
func GetEncryptionKey() []byte {
	key := g.Cfg().MustGet(context.Background(), "crypto.encryptionKey").String()
	if key == "" {
		key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}
	return uc.MustGetEncryptionKey(key)
}

// apiKeyHash 计算 API Key 的 SHA-256 哈希
func apiKeyHash(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(h[:])
}

// ValidateApiKey 验证 API Key 并返回认证信息
func ValidateApiKey(ctx context.Context, rawKey string) (*ApiKeyInfo, error) {
	if len(rawKey) < 12 {
		g.Log().Debug(ctx, "[ApiKeyAuth] key too short", "len", len(rawKey))
		return nil, consts.ErrUnauthorized
	}

	prefix := rawKey[:12]
	keyHash := apiKeyHash(rawKey)

	// 尝试从缓存获取（需校验完整 key 哈希，防止前缀碰撞）
	var cached ApiKeyInfo
	if apiKeyCache.GetJSON(ctx, prefix, &cached) && cached.KeyHash == keyHash {
		if cached.Status == "active" {
			return &cached, nil
		}
		if cached.Status == "expired" {
			return nil, consts.ErrKeyExpired
		}
		return nil, consts.ErrKeyDisabled
	}

	// 查数据库
	type apiKeyRow struct {
		ID           int64      `json:"id"`
		TenantID     int64      `json:"tenant_id"`
		UserID       int64      `json:"user_id"`
		EncryptedKey string     `json:"encrypted_key"`
		KeyPrefix    string     `json:"key_prefix"`
		Scope        string     `json:"scope"`
		Status       string     `json:"status"`
		KeyType      string     `json:"key_type"`
		ProjectID    int64      `json:"project_id"`
		ExpiresAt    *time.Time `json:"expires_at"`
	}

	var keys []apiKeyRow
	err := dao.ApiKeys.Ctx(ctx).
		Where("key_prefix", prefix).
		Where("status", "active").
		Fields("id, tenant_id, user_id, encrypted_key, key_prefix, scope, status, key_type, project_id, expires_at").
		Scan(&keys)
	if err != nil {
		g.Log().Errorf(ctx, "[ApiKeyAuth] DB query failed: prefix=%s, error=%v", prefix, err)
		return nil, err
	}

	encKey := GetEncryptionKey()

	for _, k := range keys {
		decrypted, err := uc.DecryptString(encKey, k.EncryptedKey)
		if err != nil {
			g.Log().Debugf(ctx, "[ApiKeyAuth] decrypt failed: keyID=%d", k.ID)
			continue
		}
		if decrypted != rawKey {
			continue
		}

		// 检查过期
		if k.ExpiresAt != nil && k.ExpiresAt.Before(time.Now()) {
			info := &ApiKeyInfo{
				ID:        k.ID,
				TenantID:  k.TenantID,
				UserID:    k.UserID,
				Scope:     k.Scope,
				Status:    "expired",
				KeyType:   k.KeyType,
				ProjectID: k.ProjectID,
				KeyHash:   keyHash,
			}
			apiKeyCache.Set(ctx, prefix, info)
			return nil, consts.ErrKeyExpired
		}

		// 检查租户状态
		var tenant *struct {
			Status string `json:"status"`
		}
		err = dao.TntTenants.Ctx(ctx).
			Where("id", k.TenantID).
			Fields("status").
			Scan(&tenant)
		if err != nil || tenant == nil || tenant.Status != "active" {
			return nil, consts.ErrTenantSuspended
		}

		info := &ApiKeyInfo{
			ID:        k.ID,
			TenantID:  k.TenantID,
			UserID:    k.UserID,
			Scope:     k.Scope,
			Status:    "active",
			KeyType:   k.KeyType,
			ProjectID: k.ProjectID,
			KeyHash:   keyHash,
		}

		apiKeyCache.Set(ctx, prefix, info)
		return info, nil
	}

	g.Log().Debugf(ctx, "[ApiKeyAuth] no matching key found: prefix=%s, rowsScanned=%d", prefix, len(keys))
	return nil, consts.ErrUnauthorized
}

// GenerateApiKey 生成新的 API Key
func GenerateApiKey(ctx context.Context) (rawKey string, prefix string, encryptedKey string, err error) {
	rawKey = "sk-" + grand.S(48)
	prefix = rawKey[:12]

	encKey := GetEncryptionKey()
	encrypted, err := uc.EncryptString(encKey, rawKey)
	if err != nil {
		return "", "", "", err
	}

	return rawKey, prefix, encrypted, nil
}

// DecryptChannelKey 解密渠道 Key（供 channel_test.go 使用）
func DecryptChannelKey(encKey []byte, encrypted string) (string, error) {
	return uc.DecryptString(encKey, encrypted)
}

// ParseChannelSettings 解析渠道 JSONB 设置
func ParseChannelSettings(settingsJSON string) common.ChannelSettings {
	s := DefaultChannelSettings()
	if settingsJSON == "" || settingsJSON == "{}" || settingsJSON == "null" {
		return s
	}
	if err := json.Unmarshal([]byte(settingsJSON), &s); err != nil {
		g.Log().Warningf(context.Background(), "ParseChannelSettings: failed to parse JSON: %v", err)
		return s
	}
	// 确保默认值
	if s.TimeoutSeconds == 0 {
		s.TimeoutSeconds = 60
	}
	if s.RetryCount == 0 {
		s.RetryCount = 1
	}
	return s
}
