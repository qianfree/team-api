package oauth

import (
	"encoding/json"
	"strings"
)

// OAuthKeyData 存储在 chn_channel_keys.encrypted_key 中的 OAuth 凭证 JSON 结构
// 当 key_type='oauth' 时，encrypted_key 解密后为该结构的 JSON 字符串
type OAuthKeyData struct {
	Platform     string `json:"platform"` // "claude" | "openai" | "gemini"
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    int64  `json:"expires_at"` // Unix 时间戳
	TokenType    string `json:"token_type,omitempty"`
	Scope        string `json:"scope,omitempty"`

	// Claude 专属字段
	OrgUUID      string `json:"org_uuid,omitempty"`
	AccountUUID  string `json:"account_uuid,omitempty"`
	EmailAddress string `json:"email_address,omitempty"` // Claude 和 OpenAI 共用

	// OpenAI 专属字段
	AccountID string `json:"account_id,omitempty"`      // chatgpt_account_id
	UserID    string `json:"user_id,omitempty"`         // chatgpt_user_id
	OrgID     string `json:"organization_id,omitempty"` // OpenAI organization_id
	PlanType  string `json:"plan_type,omitempty"`       // OpenAI plan_type

	// Gemini 专属字段
	ProjectID string `json:"project_id,omitempty"` // GCP project_id
	OAuthType string `json:"oauth_type,omitempty"` // code_assist / ai_studio / google_one
	TierID    string `json:"tier_id,omitempty"`    // Gemini tier
}

// IsOAuthKeyData 检测解密后的密钥字符串是否为 OAuth JSON 格式
func IsOAuthKeyData(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '{' {
		return false
	}
	var probe struct {
		Platform string `json:"platform"`
	}
	if err := json.Unmarshal([]byte(s), &probe); err != nil {
		return false
	}
	return probe.Platform != ""
}

// TokenResponse 通用 OAuth token 响应
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}
