// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// NtfSendLog is the golang structure for table ntf_send_log.
type NtfSendLog struct {
	Id           int64       `json:"id"            orm:"id"            description:"主键ID"`                                     // 主键ID
	TenantId     int64       `json:"tenant_id"     orm:"tenant_id"     description:"租户ID（系统级通知为 NULL）"`                        // 租户ID（系统级通知为 NULL）
	UserId       int64       `json:"user_id"       orm:"user_id"       description:"目标用户ID"`                                   // 目标用户ID
	TemplateCode string      `json:"template_code" orm:"template_code" description:"使用的通知模板编码"`                                // 使用的通知模板编码
	Channel      string      `json:"channel"       orm:"channel"       description:"发送渠道：email / sms / webhook"`               // 发送渠道：email / sms / webhook
	Recipient    string      `json:"recipient"     orm:"recipient"     description:"接收方（邮箱地址/手机号/Webhook URL）"`                // 接收方（邮箱地址/手机号/Webhook URL）
	Subject      string      `json:"subject"       orm:"subject"       description:"发送主题"`                                     // 发送主题
	Body         string      `json:"body"          orm:"body"          description:"发送内容（渲染后的最终内容）"`                           // 发送内容（渲染后的最终内容）
	Status       string      `json:"status"        orm:"status"        description:"状态：pending（待发送）/ sent（已发送）/ failed（发送失败）"` // 状态：pending（待发送）/ sent（已发送）/ failed（发送失败）
	ErrorMessage string      `json:"error_message" orm:"error_message" description:"失败时的错误信息"`                                 // 失败时的错误信息
	SentAt       *gtime.Time `json:"sent_at"       orm:"sent_at"       description:"实际发送时间"`                                   // 实际发送时间
	RetryCount   int         `json:"retry_count"   orm:"retry_count"   description:"重试次数（最多重试 3 次）"`                           // 重试次数（最多重试 3 次）
	CreatedAt    *gtime.Time `json:"created_at"    orm:"created_at"    description:"创建时间"`                                     // 创建时间
	UpdatedAt    *gtime.Time `json:"updated_at"    orm:"updated_at"    description:"更新时间"`                                     // 更新时间
}
