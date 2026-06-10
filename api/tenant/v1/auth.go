package v1

import "github.com/gogf/gf/v2/frame/g"

// TenantRegisterReq 租户注册请求
type TenantRegisterReq struct {
	g.Meta         `path:"/auth/register" method:"post" mime:"json" tags:"租户控制台-认证" summary:"租户注册" group:"public" middleware:"-"`
	Email          string `json:"email" v:"required|email#请输入邮箱|邮箱格式不正确" dc:"邮箱"`
	Code           string `json:"code" dc:"邮箱验证码（邮箱验证开启时必填）"`
	Password       string `json:"password" v:"required|length:8,64#请输入密码|密码长度为8-64位" dc:"密码"`
	TenantName     string `json:"tenant_name" v:"required|length:2,100#请输入组织名称|组织名称长度为2-100位" dc:"组织名称"`
	TenantCode     string `json:"tenant_code" v:"required|length:3,30|regex:^[a-z0-9][a-z0-9-]*[a-z0-9]$#请输入组织代码|组织代码为3-30位小写字母数字|组织代码格式不正确" dc:"组织代码"`
	Username       string `json:"username" v:"required|length:3,50#请输入用户名|用户名长度为3-50位" dc:"用户名"`
	CaptchaKey     string `json:"captcha_key" dc:"滑块验证码key（Turnstile关闭时必填）"`
	CaptchaX       int    `json:"captcha_x" dc:"滑块X坐标（Turnstile关闭时必填）"`
	TurnstileToken string `json:"turnstile_token" dc:"Cloudflare Turnstile验证token（Turnstile启用时必填）"`
}

type TenantRegisterRes struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
	Tenant       struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Code string `json:"code"`
	} `json:"tenant"`
	User struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
	} `json:"user"`
}

// TenantLoginReq 租户登录请求
type TenantLoginReq struct {
	g.Meta         `path:"/auth/login" method:"post" mime:"json" tags:"租户控制台-认证" summary:"租户登录" group:"public" middleware:"-"`
	Account        string `json:"account" v:"required#请输入账号" dc:"账号（username@tenant_code 或邮箱）"`
	Password       string `json:"password" v:"required#请输入密码" dc:"密码"`
	Type           string `json:"type" v:"required|in:ram,admin#请选择登录方式|登录方式无效" dc:"登录方式：ram（RAM账号）/ admin（管理员邮箱）"`
	CaptchaKey     string `json:"captcha_key" dc:"验证码key（Turnstile关闭时必填）"`
	CaptchaX       int    `json:"captcha_x" dc:"滑块X坐标（Turnstile关闭时必填）"`
	TurnstileToken string `json:"turnstile_token" dc:"Cloudflare Turnstile验证token（Turnstile启用时必填）"`
}

type TenantLoginRes struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresAt        string `json:"expires_at"`
	TotpRequired     bool   `json:"totp_required"`               // 是否需要 2FA 验证
	ProvisionalToken string `json:"provisional_token,omitempty"` // 2FA 临时令牌
	Tenant           struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Code string `json:"code"`
	} `json:"tenant"`
	User struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
	} `json:"user"`
	MaintenanceInfo *LoginMaintenanceInfo `json:"maintenance_info,omitempty"`
}

// LoginMaintenanceInfo 维护模式信息
type LoginMaintenanceInfo struct {
	Enabled  bool   `json:"enabled"`
	Message  string `json:"message"`
	Duration string `json:"duration"`
}

// TenantLogoutReq 租户登出请求
type TenantLogoutReq struct {
	g.Meta `path:"/auth/logout" method:"post" mime:"json" tags:"租户控制台-认证" summary:"租户登出"`
}

type TenantLogoutRes struct{}

// TenantRefreshReq 刷新 Token 请求
type TenantRefreshReq struct {
	g.Meta       `path:"/auth/refresh" method:"post" mime:"json" tags:"租户控制台-认证" summary:"刷新 Token" group:"public" middleware:"-"`
	RefreshToken string `json:"refresh_token" v:"required#请提供刷新令牌" dc:"刷新令牌"`
}

type TenantRefreshRes struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
}

// TenantChangePasswordReq 修改密码请求
type TenantChangePasswordReq struct {
	g.Meta      `path:"/auth/change-password" method:"put" mime:"json" tags:"租户控制台-认证" summary:"修改密码"`
	OldPassword string `json:"old_password" v:"required#请输入原密码" dc:"原密码"`
	NewPassword string `json:"new_password" v:"required|length:8,64#请输入新密码|密码长度为8-64位" dc:"新密码"`
}

type TenantChangePasswordRes struct{}

// TenantSessionListReq 活跃会话列表请求
type TenantSessionListReq struct {
	g.Meta `path:"/auth/sessions" method:"get" mime:"json" tags:"租户控制台-认证" summary:"活跃会话列表"`
}

type TenantSessionListRes struct {
	List []TenantSessionItem `json:"list"`
}

type TenantSessionItem struct {
	ID         int64  `json:"id"`
	IpAddress  string `json:"ip_address"`
	DeviceInfo string `json:"device_info"`
	ExpiresAt  string `json:"expires_at"`
	CreatedAt  string `json:"created_at"`
	IsCurrent  bool   `json:"is_current"`
}

// TenantRevokeSessionReq 踢出指定会话请求
type TenantRevokeSessionReq struct {
	g.Meta `path:"/auth/sessions/{id}" method:"delete" mime:"json" tags:"租户控制台-认证" summary:"踢出指定会话"`
	Id     int64 `json:"id" in:"path" v:"required#请指定会话ID"`
}

type TenantRevokeSessionRes struct{}
