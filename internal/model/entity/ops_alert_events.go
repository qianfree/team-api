// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// OpsAlertEvents is the golang structure for table ops_alert_events.
type OpsAlertEvents struct {
	Id              int64       `json:"id"               orm:"id"               description:"主键ID"`                                             // 主键ID
	RuleId          int64       `json:"rule_id"          orm:"rule_id"          description:"关联规则ID"`                                           // 关联规则ID
	RuleName        string      `json:"rule_name"        orm:"rule_name"        description:"规则名称（冗余存储）"`                                       // 规则名称（冗余存储）
	MetricType      string      `json:"metric_type"      orm:"metric_type"      description:"指标类型"`                                             // 指标类型
	Level           string      `json:"level"            orm:"level"            description:"告警级别：info / warning / critical"`                   // 告警级别：info / warning / critical
	Status          string      `json:"status"           orm:"status"           description:"状态：firing（触发中）/ acknowledged（已确认）/ resolved（已恢复）"` // 状态：firing（触发中）/ acknowledged（已确认）/ resolved（已恢复）
	TriggerValue    float64     `json:"trigger_value"    orm:"trigger_value"    description:"触发时的实际指标值"`                                        // 触发时的实际指标值
	ThresholdValue  float64     `json:"threshold_value"  orm:"threshold_value"  description:"规则阈值"`                                             // 规则阈值
	TriggerMessage  string      `json:"trigger_message"  orm:"trigger_message"  description:"触发消息描述"`                                           // 触发消息描述
	AcknowledgedBy  int64       `json:"acknowledged_by"  orm:"acknowledged_by"  description:"确认人管理员ID"`                                         // 确认人管理员ID
	AcknowledgedAt  *gtime.Time `json:"acknowledged_at"  orm:"acknowledged_at"  description:"确认时间"`                                             // 确认时间
	ResolveNotes    string      `json:"resolve_notes"    orm:"resolve_notes"    description:"处理备注"`                                             // 处理备注
	ResolvedBy      int64       `json:"resolved_by"      orm:"resolved_by"      description:"解决人管理员ID"`                                         // 解决人管理员ID
	ResolvedAt      *gtime.Time `json:"resolved_at"      orm:"resolved_at"      description:"解决时间"`                                             // 解决时间
	NotifiedMethods []string    `json:"notified_methods" orm:"notified_methods" description:"已发送的通知方式"`                                         // 已发送的通知方式
	CreatedAt       *gtime.Time `json:"created_at"       orm:"created_at"       description:"创建时间"`                                             // 创建时间
	UpdatedAt       *gtime.Time `json:"updated_at"       orm:"updated_at"       description:"更新时间"`                                             // 更新时间
}
