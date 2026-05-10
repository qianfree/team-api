// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SptReplies is the golang structure for table spt_replies.
type SptReplies struct {
	Id        int64       `json:"id"         orm:"id"         description:"主键ID"`                           // 主键ID
	TicketId  int64       `json:"ticket_id"  orm:"ticket_id"  description:"工单ID"`                           // 工单ID
	UserId    int64       `json:"user_id"    orm:"user_id"    description:"回复者用户ID"`                        // 回复者用户ID
	UserType  string      `json:"user_type"  orm:"user_type"  description:"回复者类型：admin（管理员）/ tenant（租户用户）"` // 回复者类型：admin（管理员）/ tenant（租户用户）
	Content   string      `json:"content"    orm:"content"    description:"回复内容"`                           // 回复内容
	CreatedAt *gtime.Time `json:"created_at" orm:"created_at" description:"回复时间"`                           // 回复时间
}
