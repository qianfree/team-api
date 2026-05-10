// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// NtfTemplates is the golang structure of table ntf_templates for DAO operations like Where/Data.
type NtfTemplates struct {
	g.Meta       `orm:"table:ntf_templates, do:true"`
	Id           any         // 主键ID
	Code         any         // 模板编码（唯一标识，如 email_verify_code、balance_warning）
	Channel      any         // 发送渠道：email（邮件）/ sms（短信）/ webhook（Webhook）
	Subject      any         // 邮件/消息主题
	BodyTemplate any         // 消息体模板（支持变量占位符，如 {{.code}}）
	Variables    any         // 模板变量列表（JSONB 数组，如 ["username", "tenant_name", "code"]）
	Status       any         // 状态：active（启用）/ disabled（禁用）
	CreatedAt    *gtime.Time // 创建时间
	UpdatedAt    *gtime.Time // 更新时间
}
