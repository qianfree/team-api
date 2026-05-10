// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OpnWebhookEvents is the golang structure of table opn_webhook_events for DAO operations like Where/Data.
type OpnWebhookEvents struct {
	g.Meta          `orm:"table:opn_webhook_events, do:true"`
	Id              any         // 主键ID
	TenantId        any         // 所属租户ID
	WebhookConfigId any         // 关联的 Webhook 配置ID
	EventId         any         // 事件唯一标识
	EventType       any         // 事件类型
	Payload         any         // 事件载荷（JSON）
	Status          any         // 状态：pending / delivered / failed
	Attempts        any         // 已尝试次数
	NextRetryAt     *gtime.Time // 下次重试时间
	CreatedAt       *gtime.Time // 创建时间
	UpdatedAt       *gtime.Time // 更新时间
}
