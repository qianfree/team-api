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

// === 支付渠道管理 ===

type PaymentChannelListReq struct {
	g.Meta `path:"/payment-channels" method:"get" mime:"json" tags:"管理后台-支付" summary:"支付渠道列表"`
}

type PaymentChannelListRes struct {
	List []map[string]any `json:"list"`
}

type PaymentChannelCreateReq struct {
	g.Meta      `path:"/payment-channels" method:"post" mime:"json" tags:"管理后台-支付" summary:"创建支付渠道"`
	Channel     string `json:"channel" v:"required#请输入渠道类型" dc:"渠道类型"`
	Name        string `json:"name" v:"required#请输入渠道名称" dc:"渠道名称"`
	PaymentType string `json:"payment_type" dc:"支付类型"`
	Config      string `json:"config" dc:"配置(JSON)"`
	SortOrder   int    `json:"sort_order" dc:"排序"`
}

type PaymentChannelCreateRes struct {
	ID int64 `json:"id"`
}

type PaymentChannelDetailReq struct {
	g.Meta `path:"/payment-channels/{id}" method:"get" mime:"json" tags:"管理后台-支付" summary:"支付渠道详情"`
	Id     int64 `json:"id" in:"path" v:"required|min:1" dc:"渠道ID"`
}

type PaymentChannelDetailRes struct {
	Data map[string]any `json:"data"`
}

type PaymentChannelUpdateReq struct {
	g.Meta `path:"/payment-channels/{id}" method:"put" mime:"json" tags:"管理后台-支付" summary:"更新支付渠道"`
	Id     int64                  `json:"id" in:"path" v:"required|min:1" dc:"渠道ID"`
	Update map[string]interface{} `json:"update" dc:"更新字段"`
}

type PaymentChannelUpdateRes struct{}

type PaymentChannelDeleteReq struct {
	g.Meta `path:"/payment-channels/{id}" method:"delete" mime:"json" tags:"管理后台-支付" summary:"删除支付渠道"`
	Id     int64 `json:"id" in:"path" v:"required|min:1" dc:"渠道ID"`
}

type PaymentChannelDeleteRes struct{}

type PaymentChannelToggleReq struct {
	g.Meta `path:"/payment-channels/{id}/toggle" method:"put" mime:"json" tags:"管理后台-支付" summary:"切换支付渠道状态"`
	Id     int64 `json:"id" in:"path" v:"required|min:1" dc:"渠道ID"`
}

type PaymentChannelToggleRes struct {
	IsEnabled bool `json:"is_enabled"`
}

// === 支付设置 ===

type PaymentSettingsGetReq struct {
	g.Meta `path:"/payment-settings" method:"get" mime:"json" tags:"管理后台-支付" summary:"获取支付设置"`
}

type PaymentSettingsGetRes struct {
	AmountOptions   []int           `json:"amount_options"`
	AmountDiscount  map[int]float64 `json:"amount_discount"`
	MinTopUp        int             `json:"min_topup"`
	Currency        string          `json:"currency"`
	CallbackBaseURL string          `json:"callback_base_url"`
}

type PaymentSettingsUpdateReq struct {
	g.Meta          `path:"/payment-settings" method:"put" mime:"json" tags:"管理后台-支付" summary:"更新支付设置"`
	AmountOptions   []int           `json:"amount_options" dc:"充值金额选项"`
	AmountDiscount  map[int]float64 `json:"amount_discount" dc:"充值折扣"`
	MinTopUp        int             `json:"min_topup" dc:"最低充值金额"`
	Currency        string          `json:"currency" dc:"货币"`
	CallbackBaseURL string          `json:"callback_base_url" dc:"回调基础URL"`
}

type PaymentSettingsUpdateRes struct{}
