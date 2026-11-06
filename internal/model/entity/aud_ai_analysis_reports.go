// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// AudAiAnalysisReports is the golang structure for table aud_ai_analysis_reports.
type AudAiAnalysisReports struct {
	Id              int64       `json:"id"               orm:"id"               description:""`                                     //
	TenantId        int64       `json:"tenant_id"        orm:"tenant_id"        description:"分析目标租户ID"`                             // 分析目标租户ID
	TargetUserId    int64       `json:"target_user_id"   orm:"target_user_id"   description:"分析目标用户ID（0=租户全员）"`                     // 分析目标用户ID（0=租户全员）
	Source          string      `json:"source"           orm:"source"           description:"报告来源：manual=手动分析 / patrol=定时巡查"`       // 报告来源：manual=手动分析 / patrol=定时巡查
	TimeStart       *gtime.Time `json:"time_start"       orm:"time_start"       description:"分析窗口起点"`                               // 分析窗口起点
	TimeEnd         *gtime.Time `json:"time_end"         orm:"time_end"         description:"分析窗口终点"`                               // 分析窗口终点
	RiskLevel       string      `json:"risk_level"       orm:"risk_level"       description:"整体风险等级：high/medium/low/clean/unknown"` // 整体风险等级：high/medium/low/clean/unknown
	AuditLevel      string      `json:"audit_level"      orm:"audit_level"      description:"分析时数据实际的审计级别（数据完整性声明）"`                // 分析时数据实际的审计级别（数据完整性声明）
	LogCount        int         `json:"log_count"        orm:"log_count"        description:"覆盖的原始审计日志条数"`                          // 覆盖的原始审计日志条数
	SessionCount    int         `json:"session_count"    orm:"session_count"    description:"会话/内容链单元数（去重后）"`                       // 会话/内容链单元数（去重后）
	StandaloneCount int         `json:"standalone_count" orm:"standalone_count" description:"独立单发单元数（无会话键）"`                        // 独立单发单元数（无会话键）
	FindingCount    int         `json:"finding_count"    orm:"finding_count"    description:"发现总数（冗余计数，列表展示用）"`                     // 发现总数（冗余计数，列表展示用）
	Summary         string      `json:"summary"          orm:"summary"          description:"AI 生成的整体评估摘要（脱敏后）"`                    // AI 生成的整体评估摘要（脱敏后）
	AiModel         string      `json:"ai_model"         orm:"ai_model"         description:"使用的分析模型（快照）"`                          // 使用的分析模型（快照）
	AiTokensUsed    int         `json:"ai_tokens_used"   orm:"ai_tokens_used"   description:"分析消耗的总 token（快照）"`                     // 分析消耗的总 token（快照）
	CreatedBy       int64       `json:"created_by"       orm:"created_by"       description:"触发人管理员ID（manual 时有效，patrol 为 0）"`      // 触发人管理员ID（manual 时有效，patrol 为 0）
	CreatedAt       *gtime.Time `json:"created_at"       orm:"created_at"       description:""`                                     //
	UpdatedAt       *gtime.Time `json:"updated_at"       orm:"updated_at"       description:""`                                     //
}
