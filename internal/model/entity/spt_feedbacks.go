// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SptFeedbacks is the golang structure for table spt_feedbacks.
type SptFeedbacks struct {
	Id           int64       `json:"id"             orm:"id"             description:"主键ID"`                                                        // 主键ID
	TenantId     int64       `json:"tenant_id"      orm:"tenant_id"      description:"所属租户ID"`                                                      // 所属租户ID
	UserId       int64       `json:"user_id"        orm:"user_id"        description:"提交用户ID"`                                                      // 提交用户ID
	Category     string      `json:"category"       orm:"category"       description:"反馈类型：bug_report / feature_request / suggestion / complaint"`  // 反馈类型：bug_report / feature_request / suggestion / complaint
	Title        string      `json:"title"          orm:"title"          description:"反馈标题"`                                                        // 反馈标题
	Description  string      `json:"description"    orm:"description"    description:"反馈详细描述"`                                                      // 反馈详细描述
	Status       string      `json:"status"         orm:"status"         description:"状态：pending / acknowledged / in_progress / resolved / closed"` // 状态：pending / acknowledged / in_progress / resolved / closed
	Priority     string      `json:"priority"       orm:"priority"       description:"优先级：low / normal / high / critical"`                          // 优先级：low / normal / high / critical
	AdminReply   string      `json:"admin_reply"    orm:"admin_reply"    description:"管理员回复"`                                                       // 管理员回复
	AdminReplyBy int64       `json:"admin_reply_by" orm:"admin_reply_by" description:"回复管理员ID"`                                                     // 回复管理员ID
	AdminReplyAt *gtime.Time `json:"admin_reply_at" orm:"admin_reply_at" description:"回复时间"`                                                        // 回复时间
	Resolution   string      `json:"resolution"     orm:"resolution"     description:"解决方案摘要"`                                                      // 解决方案摘要
	Tags         string      `json:"tags"           orm:"tags"           description:"自定义标签（JSON 数组）"`                                              // 自定义标签（JSON 数组）
	Metadata     string      `json:"metadata"       orm:"metadata"       description:"元数据（环境信息、截图链接等）"`                                             // 元数据（环境信息、截图链接等）
	CreatedAt    *gtime.Time `json:"created_at"     orm:"created_at"     description:""`                                                            //
	UpdatedAt    *gtime.Time `json:"updated_at"     orm:"updated_at"     description:""`                                                            //
}
