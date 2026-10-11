package aireqanalyze

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// 插件自有表的数据模型。刻意手写而不走 gf gen dao：生成物落在 internal/dao、
// internal/model（核心生成目录），插件表混入会破坏「插件可整体删除」的隔离性
//（见设计文档 6.4）。操作统一走 g.DB().Model("plg_ai_req_analyze_xxx") + DO 对象。

// ── 报告主表 plg_ai_req_analyze_reports ──

// ReportEntity 报告实体（查询结果映射）。
type ReportEntity struct {
	Id              int64       `json:"id"                orm:"id"`
	TenantId        int64       `json:"tenant_id"         orm:"tenant_id"`
	TargetUserId    int64       `json:"target_user_id"    orm:"target_user_id"`
	Source          string      `json:"source"            orm:"source"`
	TimeStart       *gtime.Time `json:"time_start"        orm:"time_start"`
	TimeEnd         *gtime.Time `json:"time_end"          orm:"time_end"`
	RiskLevel       string      `json:"risk_level"        orm:"risk_level"`
	AuditLevel      string      `json:"audit_level"       orm:"audit_level"`
	LogCount        int         `json:"log_count"         orm:"log_count"`
	SessionCount    int         `json:"session_count"     orm:"session_count"`
	StandaloneCount int         `json:"standalone_count"  orm:"standalone_count"`
	FindingCount    int         `json:"finding_count"     orm:"finding_count"`
	Summary         string      `json:"summary"           orm:"summary"`
	AiModel         string      `json:"ai_model"          orm:"ai_model"`
	AiTokensUsed    int         `json:"ai_tokens_used"    orm:"ai_tokens_used"`
	CreatedBy       int64       `json:"created_by"        orm:"created_by"`
	CreatedAt       *gtime.Time `json:"created_at"        orm:"created_at"`
	UpdatedAt       *gtime.Time `json:"updated_at"        orm:"updated_at"`
}

// ReportDO 报告 DO（写入操作，未设置字段为 nil 自动忽略）。
type ReportDO struct {
	g.Meta          `orm:"table:plg_ai_req_analyze_reports, do:true"`
	Id              any         // 主键ID
	TenantId        any         // 分析目标租户ID
	TargetUserId    any         // 分析目标用户ID（0=租户全员）
	Source          any         // 报告来源：manual=手动分析 / patrol=定时巡查
	TimeStart       any         // 分析窗口起点
	TimeEnd         any         // 分析窗口终点
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
	CreatedAt       *gtime.Time // 创建时间
	UpdatedAt       *gtime.Time // 更新时间
}

// ── 发现明细表 plg_ai_req_analyze_findings ──

// 单元类型与分析维度枚举。
const (
	UnitTypeSession     = "session"    // 会话/内容链单元
	UnitTypeStandalone  = "standalone" // 独立请求
	DimensionCompliance = "compliance" // 合规
	DimensionLeakage    = "leakage"    // 泄露
	SourceManual        = "manual"     // 手动分析
	SourcePatrol        = "patrol"     // 定时巡查
)

// FindingEntity 发现明细实体。
type FindingEntity struct {
	Id          int64       `json:"id"           orm:"id"`
	ReportId    int64       `json:"report_id"    orm:"report_id"`
	UnitType    string      `json:"unit_type"    orm:"unit_type"`
	SessionKey  string      `json:"session_key"  orm:"session_key"`
	RequestId   string      `json:"request_id"   orm:"request_id"`
	AnchorLogId int64       `json:"anchor_log_id" orm:"anchor_log_id"`
	Dimension   string      `json:"dimension"    orm:"dimension"`
	Severity    string      `json:"severity"     orm:"severity"`
	Category    string      `json:"category"     orm:"category"`
	Evidence    string      `json:"evidence"     orm:"evidence"`
	Explanation string      `json:"explanation"  orm:"explanation"`
	CreatedAt   *gtime.Time `json:"created_at"   orm:"created_at"`
}

// FindingDO 发现明细 DO。
type FindingDO struct {
	g.Meta      `orm:"table:plg_ai_req_analyze_findings, do:true"`
	Id          any         // 主键ID
	ReportId    any         // 所属报告ID
	UnitType    any         // 单元类型：session / standalone
	SessionKey  any         // 会话稳定键（独立单元为空）
	RequestId   any         // 请求唯一ID（回链审计日志详情）
	AnchorLogId any         // 会话锚点日志ID（还原完整会话用）
	Dimension   any         // 判定维度：compliance / leakage
	Severity    any         // 严重度：high/medium/low
	Category    any         // 细分类目
	Evidence    any         // 证据片段（脱敏后）
	Explanation any         // AI 判定理由
	CreatedAt   *gtime.Time // 创建时间
}

// ── 任务表 plg_ai_req_analyze_jobs ──

// 任务状态枚举。
const (
	JobStatusPending   = "pending"   // 排队中
	JobStatusRunning   = "running"   // 执行中
	JobStatusCompleted = "completed" // 已完成
	JobStatusFailed    = "failed"    // 已失败
)

// JobEntity 任务实体。
type JobEntity struct {
	Id           int64       `json:"id"             orm:"id"`
	ReportId     int64       `json:"report_id"      orm:"report_id"`
	TenantId     int64       `json:"tenant_id"      orm:"tenant_id"`
	TargetUserId int64       `json:"target_user_id" orm:"target_user_id"`
	Source       string      `json:"source"         orm:"source"`
	TimeStart    *gtime.Time `json:"time_start"     orm:"time_start"`
	TimeEnd      *gtime.Time `json:"time_end"       orm:"time_end"`
	Status       string      `json:"status"         orm:"status"`
	Progress     int         `json:"progress"       orm:"progress"`
	ErrorMsg     string      `json:"error_msg"      orm:"error_msg"`
	CreatedBy    int64       `json:"created_by"     orm:"created_by"`
	CreatedAt    *gtime.Time `json:"created_at"     orm:"created_at"`
	UpdatedAt    *gtime.Time `json:"updated_at"     orm:"updated_at"`
}

// JobDO 任务 DO。
type JobDO struct {
	g.Meta       `orm:"table:plg_ai_req_analyze_jobs, do:true"`
	Id           any         // 主键ID
	ReportId     any         // 完成后回填的报告ID
	TenantId     any         // 分析目标租户ID
	TargetUserId any         // 分析目标用户ID（0=租户全员）
	Source       any         // 任务来源：manual / patrol
	TimeStart    any         // 分析窗口起点
	TimeEnd      any         // 分析窗口终点
	Status       any         // 状态：pending/running/completed/failed
	Progress     any         // 进度百分比
	ErrorMsg     any         // 失败原因
	CreatedBy    any         // 触发人管理员ID（patrol 为 0）
	CreatedAt    *gtime.Time // 创建时间
	UpdatedAt    *gtime.Time // 更新时间
}

// 表名常量（查询统一引用，避免散落字符串拼写错误）。
const (
	tableReports  = "plg_ai_req_analyze_reports"
	tableFindings = "plg_ai_req_analyze_findings"
	tableJobs     = "plg_ai_req_analyze_jobs"
)
