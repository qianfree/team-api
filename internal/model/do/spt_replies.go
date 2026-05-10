// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SptReplies is the golang structure of table spt_replies for DAO operations like Where/Data.
type SptReplies struct {
	g.Meta    `orm:"table:spt_replies, do:true"`
	Id        any         // 主键ID
	TicketId  any         // 工单ID
	UserId    any         // 回复者用户ID
	UserType  any         // 回复者类型：admin（管理员）/ tenant（租户用户）
	Content   any         // 回复内容
	CreatedAt *gtime.Time // 回复时间
}
