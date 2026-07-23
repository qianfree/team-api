package v1

import "github.com/gogf/gf/v2/frame/g"

// === 成员管理（管理后台）===

type AdminMemberCreateReq struct {
	g.Meta      `path:"/members" method:"post" mime:"json" tags:"管理后台-成员" summary:"添加成员"`
	TenantID    int64  `json:"tenant_id" v:"required|min:1#请选择租户|租户ID无效" dc:"租户ID"`
	Username    string `json:"username" v:"required|length:3,50#请输入用户名|用户名长度为3-50位" dc:"用户名"`
	Email       string `json:"email" v:"required|email#请输入邮箱|邮箱格式不正确" dc:"邮箱"`
	Password    string `json:"password" v:"required|length:8,64#请输入密码|密码长度为8-64位" dc:"密码"`
	DisplayName string `json:"display_name" dc:"显示名称"`
	Role        string `json:"role" v:"required|in:admin,member#请选择角色|角色无效" dc:"角色：admin/member"`
}

type AdminMemberCreateRes struct {
	Id int64 `json:"id"`
}

type AdminMemberDisableReq struct {
	g.Meta `path:"/members/{id}/disable" method:"put" mime:"json" tags:"管理后台-成员" summary:"禁用成员"`
	Id     int64 `json:"id" in:"path" v:"required|min:1" dc:"成员ID"`
}

type AdminMemberDisableRes struct{}

type AdminMemberEnableReq struct {
	g.Meta `path:"/members/{id}/enable" method:"put" mime:"json" tags:"管理后台-成员" summary:"启用成员"`
	Id     int64 `json:"id" in:"path" v:"required|min:1" dc:"成员ID"`
}

type AdminMemberEnableRes struct{}

type AdminMemberResetPasswordReq struct {
	g.Meta `path:"/members/{id}/reset-password" method:"put" mime:"json" tags:"管理后台-成员" summary:"重置成员密码"`
	Id     int64 `json:"id" in:"path" v:"required|min:1" dc:"成员ID"`
}

type AdminMemberResetPasswordRes struct {
	NewPassword string `json:"new_password"`
}

type AdminMemberUnlockReq struct {
	g.Meta `path:"/members/{id}/unlock" method:"put" mime:"json" tags:"管理后台-成员" summary:"解除成员登录锁定"`
	Id     int64 `json:"id" in:"path" v:"required|min:1" dc:"成员ID"`
}

type AdminMemberUnlockRes struct{}

// === 支付渠道管理（单例配置模式：每种渠道类型只有一个配置） ===

// PaymentChannelListReq 获取所有渠道配置
type PaymentChannelListReq struct {
	g.Meta `path:"/payment-channels" method:"get" mime:"json" tags:"管理后台-支付" summary:"获取所有渠道配置"`
}

type PaymentChannelListRes struct {
	List []map[string]any `json:"list"`
}

// PaymentChannelSaveReq 保存指定渠道的配置（整体覆盖）
type PaymentChannelSaveReq struct {
	g.Meta  `path:"/payment-channels/{channel}" method:"put" mime:"json" tags:"管理后台-支付" summary:"保存渠道配置"`
	Channel string `json:"channel" in:"path" v:"required|in:epay#请指定渠道类型|不支持的渠道类型" dc:"渠道类型"`
	Config  string `json:"config" v:"required#请提供配置" dc:"完整的 JSON 配置（含 is_enabled）"`
}

type PaymentChannelSaveRes struct{}

// === 支付设置 ===

type PaymentSettingsGetReq struct {
	g.Meta `path:"/payment-settings" method:"get" mime:"json" tags:"管理后台-支付" summary:"获取支付设置"`
}

type PaymentSettingsGetRes struct {
	AmountOptions   []int           `json:"amount_options"`
	AmountDiscount  map[int]float64 `json:"amount_discount"`
	MinTopUp        float64         `json:"min_topup"`
	Currency        string          `json:"currency"`
	CallbackBaseURL string          `json:"callback_base_url"`
}

type PaymentSettingsUpdateReq struct {
	g.Meta          `path:"/payment-settings" method:"put" mime:"json" tags:"管理后台-支付" summary:"更新支付设置"`
	AmountOptions   []int           `json:"amount_options" dc:"充值金额选项"`
	AmountDiscount  map[int]float64 `json:"amount_discount" dc:"充值折扣"`
	MinTopUp        float64         `json:"min_topup" dc:"最低充值金额"`
	Currency        string          `json:"currency" dc:"货币"`
	CallbackBaseURL string          `json:"callback_base_url" dc:"回调基础URL"`
}

type PaymentSettingsUpdateRes struct{}
