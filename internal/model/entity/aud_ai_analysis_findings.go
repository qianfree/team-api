// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// AudAiAnalysisFindings is the golang structure for table aud_ai_analysis_findings.
type AudAiAnalysisFindings struct {
	Id          int64       `json:"id"            orm:"id"            description:""`                                                                                                                      //
	ReportId    int64       `json:"report_id"     orm:"report_id"     description:"所属报告ID（逻辑关联 aud_ai_analysis_reports.id）"`                                                                               // 所属报告ID（逻辑关联 aud_ai_analysis_reports.id）
	UnitType    string      `json:"unit_type"     orm:"unit_type"     description:"单元类型：session=会话/内容链单元 / standalone=独立请求"`                                                                               // 单元类型：session=会话/内容链单元 / standalone=独立请求
	SessionKey  string      `json:"session_key"   orm:"session_key"   description:"关联的会话稳定键（sk:source:hash，本身即哈希无敏感明文；独立单元为空）"`                                                                            // 关联的会话稳定键（sk:source:hash，本身即哈希无敏感明文；独立单元为空）
	RequestId   string      `json:"request_id"    orm:"request_id"    description:"关联的请求唯一ID（证据所在轮次，回链审计日志详情）"`                                                                                            // 关联的请求唯一ID（证据所在轮次，回链审计日志详情）
	AnchorLogId int64       `json:"anchor_log_id" orm:"anchor_log_id" description:"会话锚点日志ID（用于调 LoadConversationView 还原完整会话）"`                                                                             // 会话锚点日志ID（用于调 LoadConversationView 还原完整会话）
	Dimension   string      `json:"dimension"     orm:"dimension"     description:"判定维度：compliance=合规 / leakage=泄露"`                                                                                       // 判定维度：compliance=合规 / leakage=泄露
	Severity    string      `json:"severity"      orm:"severity"      description:"严重度：high/medium/low"`                                                                                                   // 严重度：high/medium/low
	Category    string      `json:"category"      orm:"category"      description:"细分类目：phishing/fraud/malware/illegal_content/evasion/credential_leak/source_code_leak/pii_leak/internal_doc_leak/other"` // 细分类目：phishing/fraud/malware/illegal_content/evasion/credential_leak/source_code_leak/pii_leak/internal_doc_leak/other
	Evidence    string      `json:"evidence"      orm:"evidence"      description:"证据片段（原文摘录，落库前经 MaskSensitiveData 脱敏）"`                                                                                  // 证据片段（原文摘录，落库前经 MaskSensitiveData 脱敏）
	Explanation string      `json:"explanation"   orm:"explanation"   description:"AI 判定理由"`                                                                                                               // AI 判定理由
	CreatedAt   *gtime.Time `json:"created_at"    orm:"created_at"    description:""`                                                                                                                      //
}
