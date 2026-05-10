// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// OpnWebhookConfigs is the golang structure for table opn_webhook_configs.
type OpnWebhookConfigs struct {
	Id                     int64       `json:"id"                       orm:"id"                       description:"主键ID"`              // 主键ID
	TenantId               int64       `json:"tenant_id"                orm:"tenant_id"                description:"所属租户ID"`            // 所属租户ID
	Name                   string      `json:"name"                     orm:"name"                     description:"配置名称"`              // 配置名称
	Url                    string      `json:"url"                      orm:"url"                      description:"回调地址（必须 HTTPS）"`    // 回调地址（必须 HTTPS）
	SecretKey              string      `json:"secret_key"               orm:"secret_key"               description:"HMAC-SHA256 签名密钥"`  // HMAC-SHA256 签名密钥
	Events                 string      `json:"events"                   orm:"events"                   description:"订阅的事件类型列表"`         // 订阅的事件类型列表
	IsActive               bool        `json:"is_active"                orm:"is_active"                description:"是否启用"`              // 是否启用
	RetryPolicy            string      `json:"retry_policy"             orm:"retry_policy"             description:"重试策略（JSON）"`        // 重试策略（JSON）
	ConsecutiveFailures    int         `json:"consecutive_failures"     orm:"consecutive_failures"     description:"连续失败次数"`            // 连续失败次数
	MaxConsecutiveFailures int         `json:"max_consecutive_failures" orm:"max_consecutive_failures" description:"最大连续失败次数（超过后自动禁用）"` // 最大连续失败次数（超过后自动禁用）
	LastDeliveryAt         *gtime.Time `json:"last_delivery_at"         orm:"last_delivery_at"         description:"最后投递时间"`            // 最后投递时间
	CreatedAt              *gtime.Time `json:"created_at"               orm:"created_at"               description:"创建时间"`              // 创建时间
	UpdatedAt              *gtime.Time `json:"updated_at"               orm:"updated_at"               description:"更新时间"`              // 更新时间
}
