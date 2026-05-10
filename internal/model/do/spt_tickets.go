// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SptTickets is the golang structure of table spt_tickets for DAO operations like Where/Data.
type SptTickets struct {
	g.Meta          `orm:"table:spt_tickets, do:true"`
	Id              any         // 主键ID
	TenantId        any         // 所属租户ID
	UserId          any         // 创建者用户ID
	Category        any         // 分类：billing（计费）/ technical（技术）/ feature_request（功能建议）/ other（其他）
	Title           any         // 工单标题
	Description     any         // 工单描述
	Urgency         any         // 紧急程度：low（低）/ normal（普通）/ high（高）/ urgent（紧急）
	Status          any         // 状态：pending（待处理）/ processing（处理中）/ replied（已回复）/ closed（已关闭）/ reopened（已重开）
	AssignedAdminId any         // 处理人管理员ID
	CreatedAt       *gtime.Time // 创建时间
	UpdatedAt       *gtime.Time // 更新时间
}
