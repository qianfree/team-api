// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// AudAiAnalysisReports is the golang structure of table aud_ai_analysis_reports for DAO operations like Where/Data.
type AudAiAnalysisReports struct {
	g.Meta          `orm:"table:aud_ai_analysis_reports, do:true"`
	Id              any         //
	TenantId        any         // 分析目标租户ID
	TargetUserId    any         // 分析目标用户ID（0=租户全员）
	Source          any         // 报告来源：manual=手动分析 / patrol=定时巡查
	TimeStart       *gtime.Time // 分析窗口起点
	TimeEnd         *gtime.Time // 分析窗口终点
	RiskLevel       any         // 整体风险等级：high/medium/low/clean/unknown
	AuditLevel      any         // 分析时数据实际的审计级别（数据完整性声明）
	LogCount        any         // 覆盖的原始审计日志条数
	SessionCount    any         // 会话/内容链单元数（去重后）
	StandaloneCount any         // 独立单发单元数（无会话键）
	FindingCount    any         // 发现总数（冗余计数，列表展示用）
	Summary         any         // AI 生成的整体评估摘要（脱敏后）
	AiModel         any         // 使用的分析模型（快照）
	AiTokensUsed    any         // 分析消耗的总 token（快照）
	CreatedBy       any         // 触发人管理员ID（manual 时有效，patrol 为 0）
	CreatedAt       *gtime.Time //
	UpdatedAt       *gtime.Time //
}
