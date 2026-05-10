// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// OpnWebhookDeliveryLogs is the golang structure for table opn_webhook_delivery_logs.
type OpnWebhookDeliveryLogs struct {
	Id              int64       `json:"id"                orm:"id"                description:"主键ID"`             // 主键ID
	TenantId        int64       `json:"tenant_id"         orm:"tenant_id"         description:"所属租户ID"`           // 所属租户ID
	WebhookConfigId int64       `json:"webhook_config_id" orm:"webhook_config_id" description:"Webhook 配置ID"`     // Webhook 配置ID
	EventId         int64       `json:"event_id"          orm:"event_id"          description:"关联的事件ID"`          // 关联的事件ID
	Attempt         int         `json:"attempt"           orm:"attempt"           description:"第几次尝试"`            // 第几次尝试
	RequestUrl      string      `json:"request_url"       orm:"request_url"       description:"请求 URL"`           // 请求 URL
	RequestHeaders  string      `json:"request_headers"   orm:"request_headers"   description:"请求头（JSON）"`        // 请求头（JSON）
	ResponseStatus  int         `json:"response_status"   orm:"response_status"   description:"HTTP 响应状态码"`       // HTTP 响应状态码
	ResponseBody    string      `json:"response_body"     orm:"response_body"     description:"响应体（截断到 2000 字符）"` // 响应体（截断到 2000 字符）
	ResponseTimeMs  int         `json:"response_time_ms"  orm:"response_time_ms"  description:"响应时间（毫秒）"`         // 响应时间（毫秒）
	ErrorMessage    string      `json:"error_message"     orm:"error_message"     description:"错误信息"`             // 错误信息
	CreatedAt       *gtime.Time `json:"created_at"        orm:"created_at"        description:"投递时间"`             // 投递时间
}
