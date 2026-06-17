package v1

import "github.com/gogf/gf/v2/frame/g"

// TenantSendCodeReq 发送验证码请求
type TenantSendCodeReq struct {
	g.Meta     `path:"/email/send-code" method:"post" mime:"json" tags:"租户控制台-邮箱" summary:"发送验证码" group:"public" middleware:"-"`
	Email      string `json:"email" v:"required|email#请输入邮箱|邮箱格式不正确" dc:"邮箱"`
	Purpose    string `json:"purpose" v:"required|in:register,reset_password,change_email#请选择用途|用途无效" dc:"用途：register / reset_password / change_email"`
	CaptchaKey string `json:"captcha_key" dc:"滑块验证码key（找回密码用途必填）"`
	CaptchaX   int    `json:"captcha_x" dc:"滑块X坐标（找回密码用途必填）"`
}

type TenantSendCodeRes struct{}

// TenantResetPasswordReq 密码重置请求
type TenantResetPasswordReq struct {
	g.Meta     `path:"/email/reset-password" method:"post" mime:"json" tags:"租户控制台-邮箱" summary:"密码重置" group:"public" middleware:"-"`
	Email      string `json:"email" v:"required|email#请输入邮箱|邮箱格式不正确" dc:"邮箱"`
	Code       string `json:"code" v:"required|length:6,6#请输入验证码|验证码为6位" dc:"验证码"`
	Password   string `json:"password" v:"required|length:8,64#请输入新密码|密码长度为8-64位" dc:"新密码"`
	CaptchaKey string `json:"captcha_key" dc:"验证码key（验证码启用时必填）"`
	CaptchaX   int    `json:"captcha_x" dc:"滑块X坐标（验证码启用时必填）"`
}

type TenantResetPasswordRes struct{}

// TenantChangeEmailReq 更换邮箱请求
type TenantChangeEmailReq struct {
	g.Meta   `path:"/email/change-email" method:"post" mime:"json" tags:"租户控制台-邮箱" summary:"更换邮箱"`
	NewEmail string `json:"new_email" v:"required|email#请输入新邮箱|邮箱格式不正确" dc:"新邮箱"`
	Code     string `json:"code" v:"required|length:6,6#请输入验证码|验证码为6位" dc:"验证码"`
}

type TenantChangeEmailRes struct{}

// TenantSendChangeEmailCodeReq 设置/修改邮箱时发送验证码（需登录，发送前校验租户内邮箱唯一性）
type TenantSendChangeEmailCodeReq struct {
	g.Meta   `path:"/email/send-change-email-code" method:"post" mime:"json" tags:"租户控制台-邮箱" summary:"发送设置邮箱验证码"`
	NewEmail string `json:"new_email" v:"required|email#请输入新邮箱|邮箱格式不正确" dc:"新邮箱"`
}

type TenantSendChangeEmailCodeRes struct{}
