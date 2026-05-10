package v1

import "github.com/gogf/gf/v2/frame/g"

// ============================================================
// 2FA 启用流程
// ============================================================

// Admin2FASetupReq 开启 2FA 设置（生成密钥和 QR Code）
type Admin2FASetupReq struct {
	g.Meta `path:"/security/2fa/setup" method:"post" mime:"json" tags:"管理后台-安全" summary:"开启2FA设置"`
}

type Admin2FASetupRes struct {
	Secret string `json:"secret"` // TOTP 密钥（用于手动输入）
	URI    string `json:"uri"`    // otpauth:// URI
}

// Admin2FAEnableReq 确认启用 2FA（验证 TOTP 码）
type Admin2FAEnableReq struct {
	g.Meta   `path:"/security/2fa/enable" method:"post" mime:"json" tags:"管理后台-安全" summary:"确认启用2FA"`
	Code     string `json:"code" v:"required|length:6,6#请输入验证码|验证码为6位" dc:"TOTP 验证码"`
	Password string `json:"password" v:"required#请输入密码" dc:"当前密码（安全确认）"`
}

type Admin2FAEnableRes struct {
	BackupCodes []string `json:"backup_codes"` // 备用恢复码（仅展示一次）
}

// Admin2FADisableReq 禁用 2FA
type Admin2FADisableReq struct {
	g.Meta `path:"/security/2fa/disable" method:"post" mime:"json" tags:"管理后台-安全" summary:"禁用2FA"`
	Code   string `json:"code" v:"required#请输入验证码或恢复码" dc:"TOTP 验证码或恢复码"`
}

type Admin2FADisableRes struct{}

// Admin2FARegenerateBackupCodesReq 重新生成备用恢复码
type Admin2FARegenerateBackupCodesReq struct {
	g.Meta `path:"/security/2fa/backup-codes" method:"post" mime:"json" tags:"管理后台-安全" summary:"重新生成备用恢复码"`
	Code   string `json:"code" v:"required#请输入验证码" dc:"TOTP 验证码"`
}

type Admin2FARegenerateBackupCodesRes struct {
	BackupCodes []string `json:"backup_codes"`
}

// ============================================================
// 2FA 登录验证
// ============================================================

// Admin2FAVerifyReq 登录时 2FA 验证
type Admin2FAVerifyReq struct {
	g.Meta      `path:"/auth/2fa/verify" method:"post" mime:"json" tags:"管理后台-认证" summary:"2FA登录验证" group:"public" middleware:"-"`
	Provisional string `json:"provisional_token" v:"required#缺少临时令牌" dc:"登录时返回的临时令牌"`
	Code        string `json:"code" v:"required#请输入验证码" dc:"TOTP 验证码或恢复码"`
}

type Admin2FAVerifyRes struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
	User         struct {
		ID          int64  `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	} `json:"user"`
}

// ============================================================
// 高风险操作二次验证
// ============================================================

// Admin2FAConfirmReq 高风险操作二次验证
type Admin2FAConfirmReq struct {
	g.Meta `path:"/security/2fa/confirm" method:"post" mime:"json" tags:"管理后台-安全" summary:"高风险操作二次验证"`
	Code   string `json:"code" v:"required#请输入验证码" dc:"TOTP 验证码"`
}

type Admin2FAConfirmRes struct {
	ConfirmToken string `json:"confirm_token"` // 确认令牌（5分钟有效，用于高风险操作请求头）
}

// ============================================================
// 登录历史
// ============================================================

// AdminLoginHistoryReq 登录历史列表（查看所有管理员）
type AdminLoginHistoryReq struct {
	g.Meta      `path:"/security/login-history" method:"get" tags:"管理后台-安全" summary:"登录历史"`
	Page        int    `json:"page" in:"query" d:"1" dc:"页码"`
	PageSize    int    `json:"page_size" in:"query" d:"20" dc:"每页数量"`
	Username    string `json:"username" in:"query" dc:"用户名（模糊搜索）"`
	IpAddress   string `json:"ip_address" in:"query" dc:"IP地址（模糊搜索）"`
	Success     *bool  `json:"success" in:"query" dc:"登录状态：true成功/false失败"`
	LoginMethod string `json:"login_method" in:"query" dc:"登录方式：password/totp/sso/backup_code"`
	StartTime   string `json:"start_time" in:"query" dc:"开始时间（格式：2006-01-02）"`
	EndTime     string `json:"end_time" in:"query" dc:"结束时间（格式：2006-01-02）"`
}

type AdminLoginHistoryRes struct {
	List     []LoginHistoryItem `json:"list"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

type LoginHistoryItem struct {
	ID          int64  `json:"id"`
	UserId      int64  `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	LoginMethod string `json:"login_method"`
	IpAddress   string `json:"ip_address"`
	UserAgent   string `json:"user_agent"`
	Location    string `json:"location"`
	IsNewDevice bool   `json:"is_new_device"`
	Success     bool   `json:"success"`
	FailReason  string `json:"fail_reason"`
	CreatedAt   string `json:"created_at"`
}
