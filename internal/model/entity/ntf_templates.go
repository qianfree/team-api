// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// NtfTemplates is the golang structure for table ntf_templates.
type NtfTemplates struct {
	Id           int64       `json:"id"            orm:"id"            description:"主键ID"`                                                         // 主键ID
	Code         string      `json:"code"          orm:"code"          description:"模板编码（唯一标识，如 email_verify_code、balance_warning）"`               // 模板编码（唯一标识，如 email_verify_code、balance_warning）
	Channel      string      `json:"channel"       orm:"channel"       description:"发送渠道：email（邮件）/ sms（短信）/ webhook（Webhook）"`                    // 发送渠道：email（邮件）/ sms（短信）/ webhook（Webhook）
	Subject      string      `json:"subject"       orm:"subject"       description:"邮件/消息主题"`                                                      // 邮件/消息主题
	BodyTemplate string      `json:"body_template" orm:"body_template" description:"消息体模板（支持变量占位符，如 {{.code}}）"`                                   // 消息体模板（支持变量占位符，如 {{.code}}）
	Variables    string      `json:"variables"     orm:"variables"     description:"模板变量列表（JSONB 数组，如 [\"username\", \"tenant_name\", \"code\"]）"` // 模板变量列表（JSONB 数组，如 ["username", "tenant_name", "code"]）
	Status       string      `json:"status"        orm:"status"        description:"状态：active（启用）/ disabled（禁用）"`                                  // 状态：active（启用）/ disabled（禁用）
	CreatedAt    *gtime.Time `json:"created_at"    orm:"created_at"    description:"创建时间"`                                                         // 创建时间
	UpdatedAt    *gtime.Time `json:"updated_at"    orm:"updated_at"    description:"更新时间"`                                                         // 更新时间
}
