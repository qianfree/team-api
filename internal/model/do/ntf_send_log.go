// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// NtfSendLog is the golang structure of table ntf_send_log for DAO operations like Where/Data.
type NtfSendLog struct {
	g.Meta       `orm:"table:ntf_send_log, do:true"`
	Id           any         // 主键ID
	TenantId     any         // 租户ID（系统级通知为 NULL）
	UserId       any         // 目标用户ID
	TemplateCode any         // 使用的通知模板编码
	Channel      any         // 发送渠道：email / sms / webhook
	Recipient    any         // 接收方（邮箱地址/手机号/Webhook URL）
	Subject      any         // 发送主题
	Body         any         // 发送内容（渲染后的最终内容）
	Status       any         // 状态：pending（待发送）/ sent（已发送）/ failed（发送失败）
	ErrorMessage any         // 失败时的错误信息
	SentAt       *gtime.Time // 实际发送时间
	RetryCount   any         // 重试次数（最多重试 3 次）
	CreatedAt    *gtime.Time // 创建时间
	UpdatedAt    *gtime.Time // 更新时间
}
