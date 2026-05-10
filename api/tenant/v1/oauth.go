package v1

import "github.com/gogf/gf/v2/frame/g"

// === OAuth 登录（租户端） ===

type OAuthAuthorizeReq struct {
	g.Meta   `path:"/oauth/authorize" method:"get" mime:"json" tags:"租户-OAuth" summary:"获取 OAuth 授权跳转 URL" group:"public" middleware:"-"`
	Provider string `json:"provider" in:"query" v:"required|in:github,google"`
}

type OAuthAuthorizeRes struct {
	AuthorizeUrl string `json:"authorize_url"`
	State        string `json:"state"`
}

type OAuthCallbackReq struct {
	g.Meta   `path:"/oauth/{provider}/callback" method:"get" mime:"json" tags:"租户-OAuth" summary:"OAuth 回调处理" group:"public" middleware:"-"`
	Provider string `json:"provider" in:"path" v:"required|in:github,google"`
	Code     string `json:"code" in:"query" v:"required"`
	State    string `json:"state" in:"query" v:"required"`
}

type OAuthCallbackRes struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
	IsNewUser    bool   `json:"is_new_user"`
}

type OAuthLinkReq struct {
	g.Meta   `path:"/oauth/link" method:"post" mime:"json" tags:"租户-OAuth" summary:"绑定 OAuth 账号"`
	Provider string `json:"provider" v:"required|in:github,google"`
	Code     string `json:"code" v:"required"`
}

type OAuthLinkRes struct{}

type OAuthUnlinkReq struct {
	g.Meta   `path:"/oauth/unlink" method:"post" mime:"json" tags:"租户-OAuth" summary:"解绑 OAuth 账号"`
	Provider string `json:"provider" v:"required|in:github,google"`
}

type OAuthUnlinkRes struct{}

type OAuthListProvidersReq struct {
	g.Meta `path:"/oauth/providers" method:"get" mime:"json" tags:"租户-OAuth" summary:"获取已绑定的 OAuth 供应商列表"`
}

type OAuthProviderItem struct {
	Provider         string `json:"provider"`
	ProviderUserID   string `json:"provider_user_id"`
	ProviderUsername string `json:"provider_username"`
	LinkedAt         string `json:"linked_at,omitempty"`
}

type OAuthListProvidersRes struct {
	List []OAuthProviderItem `json:"list"`
}
