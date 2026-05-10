// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OpnWebhookConfigs is the golang structure of table opn_webhook_configs for DAO operations like Where/Data.
type OpnWebhookConfigs struct {
	g.Meta                 `orm:"table:opn_webhook_configs, do:true"`
	Id                     any         // 主键ID
	TenantId               any         // 所属租户ID
	Name                   any         // 配置名称
	Url                    any         // 回调地址（必须 HTTPS）
	SecretKey              any         // HMAC-SHA256 签名密钥
	Events                 any         // 订阅的事件类型列表
	IsActive               any         // 是否启用
	RetryPolicy            any         // 重试策略（JSON）
	ConsecutiveFailures    any         // 连续失败次数
	MaxConsecutiveFailures any         // 最大连续失败次数（超过后自动禁用）
	LastDeliveryAt         *gtime.Time // 最后投递时间
	CreatedAt              *gtime.Time // 创建时间
	UpdatedAt              *gtime.Time // 更新时间
}
