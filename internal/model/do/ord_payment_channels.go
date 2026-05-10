// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OrdPaymentChannels is the golang structure of table ord_payment_channels for DAO operations like Where/Data.
type OrdPaymentChannels struct {
	g.Meta      `orm:"table:ord_payment_channels, do:true"`
	Id          any         // 主键ID
	Channel     any         // 渠道标识（alipay/wechat/stripe/mock）
	Name        any         // 显示名称
	Config      any         // 渠道配置（JSONB，含 API 密钥等敏感信息）
	IsEnabled   any         // 是否启用
	SortOrder   any         // 排序权重
	CreatedAt   *gtime.Time // 创建时间
	UpdatedAt   *gtime.Time // 更新时间
	PaymentType any         // 子支付方式（alipay/wxpay 等，空表示该渠道支持所有方式）
	CallbackUrl any         // 支付回调地址覆盖（为空则使用系统默认）
	ReturnUrl   any         // 支付完成后前端跳转地址覆盖
}
