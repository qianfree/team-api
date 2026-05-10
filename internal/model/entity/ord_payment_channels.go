// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// OrdPaymentChannels is the golang structure for table ord_payment_channels.
type OrdPaymentChannels struct {
	Id          int64       `json:"id"           orm:"id"           description:"主键ID"`                               // 主键ID
	Channel     string      `json:"channel"      orm:"channel"      description:"渠道标识（alipay/wechat/stripe/mock）"`    // 渠道标识（alipay/wechat/stripe/mock）
	Name        string      `json:"name"         orm:"name"         description:"显示名称"`                               // 显示名称
	Config      string      `json:"config"       orm:"config"       description:"渠道配置（JSONB，含 API 密钥等敏感信息）"`          // 渠道配置（JSONB，含 API 密钥等敏感信息）
	IsEnabled   bool        `json:"is_enabled"   orm:"is_enabled"   description:"是否启用"`                               // 是否启用
	SortOrder   int         `json:"sort_order"   orm:"sort_order"   description:"排序权重"`                               // 排序权重
	CreatedAt   *gtime.Time `json:"created_at"   orm:"created_at"   description:"创建时间"`                               // 创建时间
	UpdatedAt   *gtime.Time `json:"updated_at"   orm:"updated_at"   description:"更新时间"`                               // 更新时间
	PaymentType string      `json:"payment_type" orm:"payment_type" description:"子支付方式（alipay/wxpay 等，空表示该渠道支持所有方式）"` // 子支付方式（alipay/wxpay 等，空表示该渠道支持所有方式）
	CallbackUrl string      `json:"callback_url" orm:"callback_url" description:"支付回调地址覆盖（为空则使用系统默认）"`                // 支付回调地址覆盖（为空则使用系统默认）
	ReturnUrl   string      `json:"return_url"   orm:"return_url"   description:"支付完成后前端跳转地址覆盖"`                      // 支付完成后前端跳转地址覆盖
}
