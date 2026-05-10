// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SptTickets is the golang structure for table spt_tickets.
type SptTickets struct {
	Id              int64       `json:"id"                orm:"id"                description:"主键ID"`                                                                       // 主键ID
	TenantId        int64       `json:"tenant_id"         orm:"tenant_id"         description:"所属租户ID"`                                                                     // 所属租户ID
	UserId          int64       `json:"user_id"           orm:"user_id"           description:"创建者用户ID"`                                                                    // 创建者用户ID
	Category        string      `json:"category"          orm:"category"          description:"分类：billing（计费）/ technical（技术）/ feature_request（功能建议）/ other（其他）"`            // 分类：billing（计费）/ technical（技术）/ feature_request（功能建议）/ other（其他）
	Title           string      `json:"title"             orm:"title"             description:"工单标题"`                                                                       // 工单标题
	Description     string      `json:"description"       orm:"description"       description:"工单描述"`                                                                       // 工单描述
	Urgency         string      `json:"urgency"           orm:"urgency"           description:"紧急程度：low（低）/ normal（普通）/ high（高）/ urgent（紧急）"`                               // 紧急程度：low（低）/ normal（普通）/ high（高）/ urgent（紧急）
	Status          string      `json:"status"            orm:"status"            description:"状态：pending（待处理）/ processing（处理中）/ replied（已回复）/ closed（已关闭）/ reopened（已重开）"` // 状态：pending（待处理）/ processing（处理中）/ replied（已回复）/ closed（已关闭）/ reopened（已重开）
	AssignedAdminId int64       `json:"assigned_admin_id" orm:"assigned_admin_id" description:"处理人管理员ID"`                                                                   // 处理人管理员ID
	CreatedAt       *gtime.Time `json:"created_at"        orm:"created_at"        description:"创建时间"`                                                                       // 创建时间
	UpdatedAt       *gtime.Time `json:"updated_at"        orm:"updated_at"        description:"更新时间"`                                                                       // 更新时间
}
