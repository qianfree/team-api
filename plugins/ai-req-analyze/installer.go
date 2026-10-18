package aireqanalyze

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// 插件自有表的建表语句（与主项目 goose 迁移通道完全隔离，见设计文档第六节）。
// 全部 IF NOT EXISTS，安装重试/并发安全；逐条执行而非拼成一个大 Exec，
// 每条失败都能精确定位。Upgrade 按 sys_plugins.version 与 PluginVersion 的
// 差异做增量迁移，首个版本无需处理。
var installDDL = []string{
	// ── 报告主表 ──
	`CREATE TABLE IF NOT EXISTS plg_ai_req_analyze_reports (
		id             BIGSERIAL PRIMARY KEY,
		tenant_id      BIGINT       NOT NULL,
		target_user_id BIGINT       NOT NULL DEFAULT 0,
		source         VARCHAR(16)  NOT NULL DEFAULT 'manual',
		time_start     TIMESTAMPTZ  NOT NULL,
		time_end       TIMESTAMPTZ  NOT NULL,
		risk_level     VARCHAR(16)  NOT NULL DEFAULT 'unknown',
		audit_level    VARCHAR(32)  NOT NULL DEFAULT '',
		log_count      INT          NOT NULL DEFAULT 0,
		session_count  INT          NOT NULL DEFAULT 0,
		standalone_count INT        NOT NULL DEFAULT 0,
		finding_count  INT          NOT NULL DEFAULT 0,
		summary        TEXT         NOT NULL DEFAULT '',
		ai_model       VARCHAR(128) NOT NULL DEFAULT '',
		ai_tokens_used INT          NOT NULL DEFAULT 0,
		created_by     BIGINT       NOT NULL DEFAULT 0,
		created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
		updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
	)`,
	`COMMENT ON TABLE plg_ai_req_analyze_reports IS 'AI请求分析-报告主表（ai-req-analyze 插件）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_reports.tenant_id IS '分析目标租户ID'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_reports.target_user_id IS '分析目标用户ID（0=租户全员）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_reports.source IS '报告来源：manual=手动分析 / patrol=定时巡查'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_reports.time_start IS '分析窗口起点'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_reports.time_end IS '分析窗口终点'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_reports.risk_level IS '整体风险等级：high/medium/low/clean/unknown'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_reports.audit_level IS '分析时数据实际的审计级别（数据完整性声明）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_reports.log_count IS '覆盖的原始审计日志条数'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_reports.session_count IS '会话/内容链单元数（去重后）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_reports.standalone_count IS '独立单发单元数（无会话键）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_reports.finding_count IS '发现总数（冗余计数，列表展示用）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_reports.summary IS 'AI 生成的整体评估摘要（脱敏后）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_reports.ai_model IS '使用的分析模型（快照）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_reports.ai_tokens_used IS '分析消耗的总 token（快照）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_reports.created_by IS '触发人管理员ID（manual 时有效，patrol 为 0）'`,
	`CREATE INDEX IF NOT EXISTS idx_plg_ai_req_analyze_reports_tenant ON plg_ai_req_analyze_reports (tenant_id, created_at)`,
	`CREATE INDEX IF NOT EXISTS idx_plg_ai_req_analyze_reports_created ON plg_ai_req_analyze_reports USING BRIN (created_at)`,

	// ── 发现明细表 ──
	`CREATE TABLE IF NOT EXISTS plg_ai_req_analyze_findings (
		id            BIGSERIAL PRIMARY KEY,
		report_id     BIGINT      NOT NULL,
		unit_type     VARCHAR(16) NOT NULL DEFAULT 'standalone',
		session_key   VARCHAR(80) NOT NULL DEFAULT '',
		request_id    VARCHAR(64) NOT NULL DEFAULT '',
		anchor_log_id BIGINT      NOT NULL DEFAULT 0,
		dimension     VARCHAR(16) NOT NULL,
		severity      VARCHAR(16) NOT NULL,
		category      VARCHAR(64) NOT NULL DEFAULT 'other',
		evidence      TEXT        NOT NULL DEFAULT '',
		explanation   TEXT        NOT NULL DEFAULT '',
		created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`COMMENT ON TABLE plg_ai_req_analyze_findings IS 'AI请求分析-发现明细（ai-req-analyze 插件）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_findings.report_id IS '所属报告ID（逻辑关联 plg_ai_req_analyze_reports.id）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_findings.unit_type IS '单元类型：session=会话/内容链单元 / standalone=独立请求'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_findings.session_key IS '关联的会话稳定键（sk:source:hash，本身即哈希无敏感明文；独立单元为空）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_findings.request_id IS '关联的请求唯一ID（证据所在轮次，回链审计日志详情）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_findings.anchor_log_id IS '会话锚点日志ID（用于调 LoadConversationView 还原完整会话）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_findings.dimension IS '判定维度：compliance=合规 / leakage=泄露'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_findings.severity IS '严重度：high/medium/low'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_findings.category IS '细分类目：phishing/fraud/malware/illegal_content/evasion/credential_leak/source_code_leak/pii_leak/internal_doc_leak/other'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_findings.evidence IS '证据片段（原文摘录，落库前经 MaskSensitiveData 脱敏）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_findings.explanation IS 'AI 判定理由'`,
	`CREATE INDEX IF NOT EXISTS idx_plg_ai_req_analyze_findings_report ON plg_ai_req_analyze_findings (report_id)`,
	`CREATE INDEX IF NOT EXISTS idx_plg_ai_req_analyze_findings_session ON plg_ai_req_analyze_findings (session_key)`,

	// ── 任务表 ──
	`CREATE TABLE IF NOT EXISTS plg_ai_req_analyze_jobs (
		id             BIGSERIAL PRIMARY KEY,
		report_id      BIGINT      NOT NULL DEFAULT 0,
		tenant_id      BIGINT      NOT NULL,
		target_user_id BIGINT      NOT NULL DEFAULT 0,
		source         VARCHAR(16) NOT NULL DEFAULT 'manual',
		time_start     TIMESTAMPTZ NOT NULL,
		time_end       TIMESTAMPTZ NOT NULL,
		status         VARCHAR(16) NOT NULL DEFAULT 'pending',
		progress       INT         NOT NULL DEFAULT 0,
		error_msg      TEXT        NOT NULL DEFAULT '',
		created_by     BIGINT      NOT NULL DEFAULT 0,
		created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`COMMENT ON TABLE plg_ai_req_analyze_jobs IS 'AI请求分析-异步任务（ai-req-analyze 插件）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_jobs.report_id IS '完成后回填的报告ID'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_jobs.tenant_id IS '分析目标租户ID'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_jobs.target_user_id IS '分析目标用户ID（0=租户全员）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_jobs.source IS '任务来源：manual=手动 / patrol=定时巡查'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_jobs.time_start IS '分析窗口起点'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_jobs.time_end IS '分析窗口终点'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_jobs.status IS '状态：pending/running/completed/failed'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_jobs.progress IS '进度百分比（已处理单元批次/总批次）'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_jobs.error_msg IS '失败原因'`,
	`COMMENT ON COLUMN plg_ai_req_analyze_jobs.created_by IS '触发人管理员ID（patrol 为 0）'`,
	`CREATE INDEX IF NOT EXISTS idx_plg_ai_req_analyze_jobs_status ON plg_ai_req_analyze_jobs (status, created_at)`,
}

// 卸载时清理的表（与 installDDL 的表一一对应）
var uninstallTables = []string{
	"plg_ai_req_analyze_findings",
	"plg_ai_req_analyze_jobs",
	"plg_ai_req_analyze_reports",
}

// Install 实现 Plugin 基础接口：逐条执行建表 DDL。
func (p *aiReqAnalyzePlugin) Install(ctx context.Context) error {
	for _, stmt := range installDDL {
		if _, err := g.DB().Exec(ctx, stmt); err != nil {
			return gerror.Wrapf(err, "ai-req-analyze 建表失败: %.120s", firstLine(stmt))
		}
	}
	return nil
}

// Upgrade 实现 Plugin 基础接口：首个版本无增量迁移。
func (p *aiReqAnalyzePlugin) Upgrade(ctx context.Context) error {
	return nil
}

// Uninstall 实现 Plugin 基础接口：只 DROP 自己前缀的表。
// keepData 语义由框架生命周期控制（keepData=true 时框架不会调用本方法）。
func (p *aiReqAnalyzePlugin) Uninstall(ctx context.Context) error {
	for _, table := range uninstallTables {
		if _, err := g.DB().Exec(ctx, "DROP TABLE IF EXISTS "+table); err != nil {
			return gerror.Wrapf(err, "ai-req-analyze 清理表失败: %s", table)
		}
	}
	return nil
}

// firstLine 取语句首行用于错误信息定位。
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i > 0 {
		return s[:i]
	}
	return s
}
