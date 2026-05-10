// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OpnWebhookDeliveryLogs is the golang structure of table opn_webhook_delivery_logs for DAO operations like Where/Data.
type OpnWebhookDeliveryLogs struct {
	g.Meta          `orm:"table:opn_webhook_delivery_logs, do:true"`
	Id              any         // 主键ID
	TenantId        any         // 所属租户ID
	WebhookConfigId any         // Webhook 配置ID
	EventId         any         // 关联的事件ID
	Attempt         any         // 第几次尝试
	RequestUrl      any         // 请求 URL
	RequestHeaders  any         // 请求头（JSON）
	ResponseStatus  any         // HTTP 响应状态码
	ResponseBody    any         // 响应体（截断到 2000 字符）
	ResponseTimeMs  any         // 响应时间（毫秒）
	ErrorMessage    any         // 错误信息
	CreatedAt       *gtime.Time // 投递时间
}
