// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// OpnWebhookEvents is the golang structure for table opn_webhook_events.
type OpnWebhookEvents struct {
	Id              int64       `json:"id"                orm:"id"                description:"主键ID"`                            // 主键ID
	TenantId        int64       `json:"tenant_id"         orm:"tenant_id"         description:"所属租户ID"`                          // 所属租户ID
	WebhookConfigId int64       `json:"webhook_config_id" orm:"webhook_config_id" description:"关联的 Webhook 配置ID"`                // 关联的 Webhook 配置ID
	EventId         string      `json:"event_id"          orm:"event_id"          description:"事件唯一标识"`                          // 事件唯一标识
	EventType       string      `json:"event_type"        orm:"event_type"        description:"事件类型"`                            // 事件类型
	Payload         string      `json:"payload"           orm:"payload"           description:"事件载荷（JSON）"`                      // 事件载荷（JSON）
	Status          string      `json:"status"            orm:"status"            description:"状态：pending / delivered / failed"` // 状态：pending / delivered / failed
	Attempts        int         `json:"attempts"          orm:"attempts"          description:"已尝试次数"`                           // 已尝试次数
	NextRetryAt     *gtime.Time `json:"next_retry_at"     orm:"next_retry_at"     description:"下次重试时间"`                          // 下次重试时间
	CreatedAt       *gtime.Time `json:"created_at"        orm:"created_at"        description:"创建时间"`                            // 创建时间
	UpdatedAt       *gtime.Time `json:"updated_at"        orm:"updated_at"        description:"更新时间"`                            // 更新时间
}
