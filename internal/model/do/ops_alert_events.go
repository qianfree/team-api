// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OpsAlertEvents is the golang structure of table ops_alert_events for DAO operations like Where/Data.
type OpsAlertEvents struct {
	g.Meta          `orm:"table:ops_alert_events, do:true"`
	Id              any         // 主键ID
	RuleId          any         // 关联规则ID
	RuleName        any         // 规则名称（冗余存储）
	MetricType      any         // 指标类型
	Level           any         // 告警级别：info / warning / critical
	Status          any         // 状态：firing（触发中）/ acknowledged（已确认）/ resolved（已恢复）
	TriggerValue    any         // 触发时的实际指标值
	ThresholdValue  any         // 规则阈值
	TriggerMessage  any         // 触发消息描述
	AcknowledgedBy  any         // 确认人管理员ID
	AcknowledgedAt  *gtime.Time // 确认时间
	ResolveNotes    any         // 处理备注
	ResolvedBy      any         // 解决人管理员ID
	ResolvedAt      *gtime.Time // 解决时间
	NotifiedMethods []string    // 已发送的通知方式
	CreatedAt       *gtime.Time // 创建时间
	UpdatedAt       *gtime.Time // 更新时间
}
