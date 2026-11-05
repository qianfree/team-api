package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

// AI请求分析：基于请求审计日志与会话关联，用独立配置的分析 AI 按会话单元分析
// 用户 AI 使用行为的合规/泄露风险（原 ai-req-analyze 插件转为系统原生功能）。
// 权限点：ai_analysis:view（查看）/ ai_analysis:analyze（发起与删除）/ ai_analysis:config（配置）。

// ── 发起按需分析 ──

type AiAnalysisCreateReq struct {
	g.Meta    `path:"/ai-analysis/analyze" method:"post" mime:"json" tags:"管理后台-AI请求分析" summary:"发起按需分析（异步任务）"`
	TenantID  int64  `json:"tenant_id" v:"required|min:1" dc:"目标租户ID"`
	UserID    int64  `json:"user_id" dc:"目标用户ID（0=租户全员）"`
	TimeStart string `json:"time_start" v:"required" dc:"开始时间（YYYY-MM-DD 或 YYYY-MM-DD HH:mm:ss）"`
	TimeEnd   string `json:"time_end" v:"required" dc:"结束时间（YYYY-MM-DD 或 YYYY-MM-DD HH:mm:ss），窗口一次最多 31 天"`
}

type AiAnalysisCreateRes struct {
	JobId int64 `json:"job_id" dc:"异步任务ID，进度与结果经任务列表查询"`
}

// ── 报告 ──

type AiAnalysisReportListReq struct {
	g.Meta    `path:"/ai-analysis/reports" method:"get" mime:"json" tags:"管理后台-AI请求分析" summary:"分析报告列表"`
	Page      int    `json:"page" d:"1"`
	PageSize  int    `json:"page_size" d:"20"`
	TenantID  int64  `json:"tenant_id" dc:"按租户过滤"`
	Source    string `json:"source" dc:"来源过滤：manual=手动 / patrol=定时巡查"`
	RiskLevel string `json:"risk_level" dc:"风险等级过滤：high/medium/low/clean/unknown"`
	StartDate string `json:"start_date" dc:"创建日期起（YYYY-MM-DD）"`
	EndDate   string `json:"end_date" dc:"创建日期止（YYYY-MM-DD）"`
}

type AiAnalysisReportListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type AiAnalysisReportDetailReq struct {
	g.Meta `path:"/ai-analysis/reports/{id}" method:"get" mime:"json" tags:"管理后台-AI请求分析" summary:"分析报告详情（含发现明细）"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

// AiAnalysisReportDetailRes 详情平铺（json:"-" + MarshalJSON 避免双层 data）。
// 报告证据含用户对话内容，访问即落敏感访问日志。
type AiAnalysisReportDetailRes struct {
	Data map[string]any `json:"-"`
}

func (r *AiAnalysisReportDetailRes) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Data)
}

type AiAnalysisReportConversationReq struct {
	g.Meta    `path:"/ai-analysis/reports/{id}/conversation" method:"get" mime:"json" tags:"管理后台-AI请求分析" summary:"按报告发现还原完整会话"`
	Id        int64 `json:"id" in:"path" v:"required|min:1" dc:"报告ID"`
	FindingId int64 `json:"finding_id" v:"required|min:1" dc:"定位锚点日志的发现ID"`
}

type AiAnalysisReportConversationRes struct {
	Data map[string]any `json:"-"`
}

func (r *AiAnalysisReportConversationRes) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Data)
}

type AiAnalysisReportDeleteReq struct {
	g.Meta `path:"/ai-analysis/reports/{id}" method:"delete" mime:"json" tags:"管理后台-AI请求分析" summary:"删除分析报告"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type AiAnalysisReportDeleteRes struct{}

// ── 任务 ──

type AiAnalysisJobListReq struct {
	g.Meta   `path:"/ai-analysis/jobs" method:"get" mime:"json" tags:"管理后台-AI请求分析" summary:"分析任务列表与进度"`
	Page     int    `json:"page" d:"1"`
	PageSize int    `json:"page_size" d:"20"`
	Status   string `json:"status" dc:"状态过滤：pending/running/completed/failed"`
	TenantID int64  `json:"tenant_id" dc:"按租户过滤"`
}

type AiAnalysisJobListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// ── 配置 ──

type AiAnalysisConfigGetReq struct {
	g.Meta `path:"/ai-analysis/config" method:"get" mime:"json" tags:"管理后台-AI请求分析" summary:"读取分析配置（敏感字段掩码）"`
}

// AiAnalysisConfigGetRes 与设置页读取同构（key/value/type/label/description），
// 敏感字段值为掩码（已配置显示 ******，未配置为空），掩码原样提交表示保持现值。
type AiAnalysisConfigGetRes struct {
	List []AdminSettingItem `json:"list"`
}

type AiAnalysisConfigUpdateReq struct {
	g.Meta   `path:"/ai-analysis/config" method:"put" mime:"json" tags:"管理后台-AI请求分析" summary:"更新分析配置"`
	Settings map[string]any `json:"settings" v:"required" dc:"配置键值对；敏感字段掩码/空值=保持现值"`
}

type AiAnalysisConfigUpdateRes struct{}

// ── 连通性测试 ──

type AiAnalysisTestReq struct {
	g.Meta `path:"/ai-analysis/test" method:"post" mime:"json" tags:"管理后台-AI请求分析" summary:"测试分析 AI 连通性（用已保存配置发起最小补全）"`
}

type AiAnalysisTestRes struct {
	OK         bool   `json:"ok"`
	Model      string `json:"model"`
	LatencyMs  int64  `json:"latency_ms"`
	TokensUsed int    `json:"tokens_used"`
	Reply      string `json:"reply"`
	Error      string `json:"error,omitempty" dc:"失败原因（连通性测试的失败本身就是有效结果）"`
}
