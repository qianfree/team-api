package v1

import "github.com/gogf/gf/v2/frame/g"

// AdminLoginReq 管理后台登录请求
type AdminLoginReq struct {
	g.Meta     `path:"/auth/login" method:"post" mime:"json" tags:"管理后台-认证" summary:"管理员登录" group:"public" middleware:"-"`
	Username   string `json:"username" v:"required#请输入用户名" dc:"用户名"`
	Password   string `json:"password" v:"required#请输入密码" dc:"密码"`
	CaptchaKey string `json:"captcha_key" dc:"验证码key（验证码启用时必填）"`
	CaptchaX   int    `json:"captcha_x" dc:"滑块X坐标（验证码启用时必填）"`
}

type AdminLoginRes struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresAt        string `json:"expires_at"`
	TotpRequired     bool   `json:"totp_required"`               // 是否需要 2FA 验证
	ProvisionalToken string `json:"provisional_token,omitempty"` // 2FA 临时令牌（totp_required=true 时返回）
	User             struct {
		ID          int64  `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	} `json:"user"`
	PendingAgreements []*LoginPendingAgreement `json:"pending_agreements,omitempty"` // 待接受协议列表
}

// LoginPendingAgreement 登录时返回的待接受协议信息
type LoginPendingAgreement struct {
	Id      int64  `json:"id"`
	Code    string `json:"code"`
	Title   string `json:"title"`
	Version string `json:"version"`
}

// AdminLogoutReq 管理后台登出请求
type AdminLogoutReq struct {
	g.Meta `path:"/auth/logout" method:"post" mime:"json" tags:"管理后台-认证" summary:"管理员登出"`
}

type AdminLogoutRes struct{}

// AdminRefreshReq 刷新 Token 请求
type AdminRefreshReq struct {
	g.Meta       `path:"/auth/refresh" method:"post" mime:"json" tags:"管理后台-认证" summary:"刷新 Token" group:"public" middleware:"-"`
	RefreshToken string `json:"refresh_token" v:"required#请提供刷新令牌" dc:"刷新令牌"`
}

type AdminRefreshRes struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
}

// AdminSessionListReq 活跃会话列表请求（查看所有管理员）
type AdminSessionListReq struct {
	g.Meta    `path:"/auth/sessions" method:"get" mime:"json" tags:"管理后台-认证" summary:"活跃会话列表"`
	Page      int    `json:"page" in:"query" d:"1" dc:"页码"`
	PageSize  int    `json:"page_size" in:"query" d:"20" dc:"每页数量"`
	Username  string `json:"username" in:"query" dc:"用户名（模糊搜索）"`
	IpAddress string `json:"ip_address" in:"query" dc:"IP地址（模糊搜索）"`
}

type AdminSessionListRes struct {
	List     []AdminSessionItem `json:"list"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

type AdminSessionItem struct {
	ID          int64  `json:"id"`
	UserId      int64  `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	IpAddress   string `json:"ip_address"`
	DeviceInfo  string `json:"device_info"`
	ExpiresAt   string `json:"expires_at"`
	CreatedAt   string `json:"created_at"`
	IsCurrent   bool   `json:"is_current"`
}

// AdminRevokeSessionReq 踢出指定会话请求
type AdminRevokeSessionReq struct {
	g.Meta `path:"/auth/sessions/{id}" method:"delete" mime:"json" tags:"管理后台-认证" summary:"踢出指定会话"`
	Id     int64 `json:"id" in:"path" v:"required#请指定会话ID"`
}

type AdminRevokeSessionRes struct{}

// AdminForceLogoutReq 强制下线指定用户请求
type AdminForceLogoutReq struct {
	g.Meta `path:"/auth/sessions/user/{id}" method:"delete" mime:"json" tags:"管理后台-认证" summary:"强制用户下线"`
	Id     int64 `json:"id" in:"path" v:"required#请指定用户ID"`
}

type AdminForceLogoutRes struct{}

// AdminChangePasswordReq 修改密码请求
type AdminChangePasswordReq struct {
	g.Meta      `path:"/auth/change-password" method:"put" mime:"json" tags:"管理后台-认证" summary:"修改密码"`
	OldPassword string `json:"old_password" v:"required#请输入原密码" dc:"原密码"`
	NewPassword string `json:"new_password" v:"required|length:8,64#请输入新密码|密码长度为8-64位" dc:"新密码"`
}

type AdminChangePasswordRes struct{}
