// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// AudAiAnalysisFindings is the golang structure of table aud_ai_analysis_findings for DAO operations like Where/Data.
type AudAiAnalysisFindings struct {
	g.Meta      `orm:"table:aud_ai_analysis_findings, do:true"`
	Id          any         //
	ReportId    any         // 所属报告ID（逻辑关联 aud_ai_analysis_reports.id）
	UnitType    any         // 单元类型：session=会话/内容链单元 / standalone=独立请求
	SessionKey  any         // 关联的会话稳定键（sk:source:hash，本身即哈希无敏感明文；独立单元为空）
	RequestId   any         // 关联的请求唯一ID（证据所在轮次，回链审计日志详情）
	AnchorLogId any         // 会话锚点日志ID（用于调 LoadConversationView 还原完整会话）
	Dimension   any         // 判定维度：compliance=合规 / leakage=泄露
	Severity    any         // 严重度：high/medium/low
	Category    any         // 细分类目：phishing/fraud/malware/illegal_content/evasion/credential_leak/source_code_leak/pii_leak/internal_doc_leak/other
	Evidence    any         // 证据片段（原文摘录，落库前经 MaskSensitiveData 脱敏）
	Explanation any         // AI 判定理由
	CreatedAt   *gtime.Time //
}
