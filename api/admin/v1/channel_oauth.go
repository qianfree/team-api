package v1

import "github.com/gogf/gf/v2/frame/g"

// ChannelOAuthAuthURLReq 生成 OAuth 授权链接请求
type ChannelOAuthAuthURLReq struct {
	g.Meta     `path:"/channels/oauth/auth-url" method:"post" mime:"json" tags:"管理后台-渠道" summary:"生成 OAuth 授权链接"`
	Platform   string `json:"platform" v:"required|in:claude,openai,gemini" dc:"平台：claude/openai/gemini"`
	AuthMode   string `json:"auth_mode" d:"browser" v:"in:browser,cookie" dc:"授权模式：browser=浏览器跳转，cookie=Cookie自动授权（仅Claude）"`
	Scope      string `json:"scope" d:"full" v:"in:full,inference" dc:"授权范围（仅Claude）"`
	OAuthType  string `json:"oauth_type" d:"code_assist" v:"in:code_assist" dc:"OAuth 类型（仅Gemini）"`
	SessionKey string `json:"session_key" dc:"Claude sessionKey cookie（仅 cookie 模式）"`
}

type ChannelOAuthAuthURLRes struct {
	AuthURL   string `json:"auth_url"`
	SessionID string `json:"session_id"`
	KeyID     int64  `json:"key_id,omitempty"`     // cookie 模式完成后的 key_id
	ChannelID int64  `json:"channel_id,omitempty"` // cookie 模式完成后的 channel_id
}

// ChannelOAuthExchangeReq OAuth 授权码换取令牌请求
type ChannelOAuthExchangeReq struct {
	g.Meta    `path:"/channels/oauth/exchange" method:"post" mime:"json" tags:"管理后台-渠道" summary:"OAuth 授权码换取令牌"`
	SessionID string `json:"session_id" v:"required" dc:"授权会话ID"`
	Code      string `json:"code" dc:"授权码（浏览器模式必填）"`
	State     string `json:"state" dc:"state 参数"`
	KeyName   string `json:"key_name" d:"OAuth 官方账号" dc:"存储的 Key 名称"`
}

type ChannelOAuthExchangeRes struct {
	ChannelID int64 `json:"channel_id"`
	KeyID     int64 `json:"key_id"`
}

// ChannelOAuthRefreshReq 手动刷新 OAuth 令牌请求
type ChannelOAuthRefreshReq struct {
	g.Meta `path:"/channels/oauth/refresh" method:"post" mime:"json" tags:"管理后台-渠道" summary:"手动刷新 OAuth 令牌"`
	KeyID  int64 `json:"key_id" v:"required" dc:"渠道 Key ID"`
}

type ChannelOAuthRefreshRes struct {
	ExpiresAt string `json:"expires_at"`
}
