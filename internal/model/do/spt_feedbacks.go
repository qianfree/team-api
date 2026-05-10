// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SptFeedbacks is the golang structure of table spt_feedbacks for DAO operations like Where/Data.
type SptFeedbacks struct {
	g.Meta       `orm:"table:spt_feedbacks, do:true"`
	Id           any         // 主键ID
	TenantId     any         // 所属租户ID
	UserId       any         // 提交用户ID
	Category     any         // 反馈类型：bug_report / feature_request / suggestion / complaint
	Title        any         // 反馈标题
	Description  any         // 反馈详细描述
	Status       any         // 状态：pending / acknowledged / in_progress / resolved / closed
	Priority     any         // 优先级：low / normal / high / critical
	AdminReply   any         // 管理员回复
	AdminReplyBy any         // 回复管理员ID
	AdminReplyAt *gtime.Time // 回复时间
	Resolution   any         // 解决方案摘要
	Tags         any         // 自定义标签（JSON 数组）
	Metadata     any         // 元数据（环境信息、截图链接等）
	CreatedAt    *gtime.Time //
	UpdatedAt    *gtime.Time //
}
