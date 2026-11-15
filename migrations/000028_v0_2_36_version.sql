-- +goose Up
-- ============================================================
-- v0.3 通用更新：计费扩展 + AI请求分析转内置功能
--
-- 一、表达式计费模式（billing_mode=formula）
-- 二、工具调用计费（联网搜索/云端工具按次附加费）
--
-- 计算按次附加费，叠加在任何计费模式之上：
--   附加费 = Σ min(次数, 10000) × 单价（固定单次计费，
--   租户/时段/参数乘数均不作用于工具费，只作用于模型 token 费，
--   见 billing/pricing.go applyToolSurcharge 及 tool_pricing_test.go 行为锁定）
-- 预扣不含工具费（调用次数事前未知），结算补差；明细随计费快照
-- billing_snapshot.tool_surcharges 留痕。
--
-- 为 bil_usage_logs 增加两列（可查询聚合，快照 JSONB 仍是完整真相）：
--   tool_usage            工具调用计数 JSONB（{"web_search": 3}）
--   tool_surcharge_cost   工具附加费合计（已含在 total_cost/actual_cost 中）
--
-- ── 三、AI请求分析：插件 → 核心功能迁移 ───────────────────
--
-- ai-req-analyze 插件（基于请求审计日志与会话关联，用独立分析 AI
-- 按会话单元分析用户 AI 使用行为的合规/泄露风险）转为系统原生功能：
--   · 数据表  plg_ai_req_analyze_*  → aud_ai_analysis_*（aud_ 审计域前缀）
--   · 配置    sys_plugins.config(JSONB) → sys_options 的 ai_analysis_* 键
--             （ai_analysis_api_key 为 AES-256-GCM 密文原样搬运，读取方共用
--              crypto.encryptionKey，v1: 前缀密文直接兼容）
--   · 权限点  ai-req-analyze:view/analyze/config → ai_analysis:view/analyze/config
--
-- 幂等性：覆盖三种部署状态——装过插件（重命名保数据）、从未安装（新建表）、
-- 部分执行重跑（各步均有存在性守卫）。
-- ============================================================

-- ═══ 一、表达式计费模式：更新值域注释 ═════════════════════

COMMENT ON COLUMN mdl_pricing.billing_mode IS '计费模式：token=按量；per_request=按次；tiered=阶梯；per_second=按秒（视频）；special=特殊计费；formula=表达式计费（pricing JSONB formula 键）';

-- ═══ 二、工具调用计费：bil_usage_logs 增列 ════════════════

ALTER TABLE bil_usage_logs
    ADD COLUMN IF NOT EXISTS tool_usage JSONB,
    ADD COLUMN IF NOT EXISTS tool_surcharge_cost NUMERIC(20,10) DEFAULT 0 NOT NULL;

COMMENT ON COLUMN bil_usage_logs.tool_usage IS '服务端/云端工具调用计数（工具名→次数，如 {"web_search": 3}）；无工具调用为 NULL';
COMMENT ON COLUMN bil_usage_logs.tool_surcharge_cost IS '工具调用附加费合计（按次收费，固定单次计费不乘租户/时段乘数，已含在 total_cost 中）';

-- ═══ 三、AI请求分析：插件 → 核心功能迁移 ═══════════════════

-- 3a. 已安装过插件：三表重命名（含索引/序列），数据原样保留
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'plg_ai_req_analyze_reports')
       AND NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'aud_ai_analysis_reports') THEN
        ALTER TABLE plg_ai_req_analyze_reports RENAME TO aud_ai_analysis_reports;
        ALTER INDEX IF EXISTS idx_plg_ai_req_analyze_reports_tenant RENAME TO idx_aud_ai_analysis_reports_tenant;
        ALTER INDEX IF EXISTS idx_plg_ai_req_analyze_reports_created RENAME TO idx_aud_ai_analysis_reports_created;
        ALTER SEQUENCE IF EXISTS plg_ai_req_analyze_reports_id_seq RENAME TO aud_ai_analysis_reports_id_seq;
        COMMENT ON TABLE aud_ai_analysis_reports IS 'AI请求分析-报告主表';
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'plg_ai_req_analyze_findings')
       AND NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'aud_ai_analysis_findings') THEN
        ALTER TABLE plg_ai_req_analyze_findings RENAME TO aud_ai_analysis_findings;
        ALTER INDEX IF EXISTS idx_plg_ai_req_analyze_findings_report RENAME TO idx_aud_ai_analysis_findings_report;
        ALTER INDEX IF EXISTS idx_plg_ai_req_analyze_findings_session RENAME TO idx_aud_ai_analysis_findings_session;
        ALTER SEQUENCE IF EXISTS plg_ai_req_analyze_findings_id_seq RENAME TO aud_ai_analysis_findings_id_seq;
        COMMENT ON TABLE aud_ai_analysis_findings IS 'AI请求分析-发现明细';
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'plg_ai_req_analyze_jobs')
       AND NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'aud_ai_analysis_jobs') THEN
        ALTER TABLE plg_ai_req_analyze_jobs RENAME TO aud_ai_analysis_jobs;
        ALTER INDEX IF EXISTS idx_plg_ai_req_analyze_jobs_status RENAME TO idx_aud_ai_analysis_jobs_status;
        ALTER SEQUENCE IF EXISTS plg_ai_req_analyze_jobs_id_seq RENAME TO aud_ai_analysis_jobs_id_seq;
        COMMENT ON TABLE aud_ai_analysis_jobs IS 'AI请求分析-异步任务';
    END IF;
END
$$;
-- +goose StatementEnd

-- 3b. 从未安装插件（或个别表缺失）：按核心规范建表
CREATE TABLE IF NOT EXISTS aud_ai_analysis_reports (
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
);
COMMENT ON TABLE aud_ai_analysis_reports IS 'AI请求分析-报告主表';
COMMENT ON COLUMN aud_ai_analysis_reports.tenant_id IS '分析目标租户ID';
COMMENT ON COLUMN aud_ai_analysis_reports.target_user_id IS '分析目标用户ID（0=租户全员）';
COMMENT ON COLUMN aud_ai_analysis_reports.source IS '报告来源：manual=手动分析 / patrol=定时巡查';
COMMENT ON COLUMN aud_ai_analysis_reports.time_start IS '分析窗口起点';
COMMENT ON COLUMN aud_ai_analysis_reports.time_end IS '分析窗口终点';
COMMENT ON COLUMN aud_ai_analysis_reports.risk_level IS '整体风险等级：high/medium/low/clean/unknown';
COMMENT ON COLUMN aud_ai_analysis_reports.audit_level IS '分析时数据实际的审计级别（数据完整性声明）';
COMMENT ON COLUMN aud_ai_analysis_reports.log_count IS '覆盖的原始审计日志条数';
COMMENT ON COLUMN aud_ai_analysis_reports.session_count IS '会话/内容链单元数（去重后）';
COMMENT ON COLUMN aud_ai_analysis_reports.standalone_count IS '独立单发单元数（无会话键）';
COMMENT ON COLUMN aud_ai_analysis_reports.finding_count IS '发现总数（冗余计数，列表展示用）';
COMMENT ON COLUMN aud_ai_analysis_reports.summary IS 'AI 生成的整体评估摘要（脱敏后）';
COMMENT ON COLUMN aud_ai_analysis_reports.ai_model IS '使用的分析模型（快照）';
COMMENT ON COLUMN aud_ai_analysis_reports.ai_tokens_used IS '分析消耗的总 token（快照）';
COMMENT ON COLUMN aud_ai_analysis_reports.created_by IS '触发人管理员ID（manual 时有效，patrol 为 0）';
CREATE INDEX IF NOT EXISTS idx_aud_ai_analysis_reports_tenant ON aud_ai_analysis_reports (tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_aud_ai_analysis_reports_created ON aud_ai_analysis_reports USING BRIN (created_at);

CREATE TABLE IF NOT EXISTS aud_ai_analysis_findings (
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
);
COMMENT ON TABLE aud_ai_analysis_findings IS 'AI请求分析-发现明细';
COMMENT ON COLUMN aud_ai_analysis_findings.report_id IS '所属报告ID（逻辑关联 aud_ai_analysis_reports.id）';
COMMENT ON COLUMN aud_ai_analysis_findings.unit_type IS '单元类型：session=会话/内容链单元 / standalone=独立请求';
COMMENT ON COLUMN aud_ai_analysis_findings.session_key IS '关联的会话稳定键（sk:source:hash，本身即哈希无敏感明文；独立单元为空）';
COMMENT ON COLUMN aud_ai_analysis_findings.request_id IS '关联的请求唯一ID（证据所在轮次，回链审计日志详情）';
COMMENT ON COLUMN aud_ai_analysis_findings.anchor_log_id IS '会话锚点日志ID（用于调 LoadConversationView 还原完整会话）';
COMMENT ON COLUMN aud_ai_analysis_findings.dimension IS '判定维度：compliance=合规 / leakage=泄露';
COMMENT ON COLUMN aud_ai_analysis_findings.severity IS '严重度：high/medium/low';
COMMENT ON COLUMN aud_ai_analysis_findings.category IS '细分类目：phishing/fraud/malware/illegal_content/evasion/credential_leak/source_code_leak/pii_leak/internal_doc_leak/other';
COMMENT ON COLUMN aud_ai_analysis_findings.evidence IS '证据片段（原文摘录，落库前经 MaskSensitiveData 脱敏）';
COMMENT ON COLUMN aud_ai_analysis_findings.explanation IS 'AI 判定理由';
CREATE INDEX IF NOT EXISTS idx_aud_ai_analysis_findings_report ON aud_ai_analysis_findings (report_id);
CREATE INDEX IF NOT EXISTS idx_aud_ai_analysis_findings_session ON aud_ai_analysis_findings (session_key);

CREATE TABLE IF NOT EXISTS aud_ai_analysis_jobs (
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
);
COMMENT ON TABLE aud_ai_analysis_jobs IS 'AI请求分析-异步任务';
COMMENT ON COLUMN aud_ai_analysis_jobs.report_id IS '完成后回填的报告ID';
COMMENT ON COLUMN aud_ai_analysis_jobs.tenant_id IS '分析目标租户ID';
COMMENT ON COLUMN aud_ai_analysis_jobs.target_user_id IS '分析目标用户ID（0=租户全员）';
COMMENT ON COLUMN aud_ai_analysis_jobs.source IS '任务来源：manual=手动 / patrol=定时巡查';
COMMENT ON COLUMN aud_ai_analysis_jobs.time_start IS '分析窗口起点';
COMMENT ON COLUMN aud_ai_analysis_jobs.time_end IS '分析窗口终点';
COMMENT ON COLUMN aud_ai_analysis_jobs.status IS '状态：pending/running/completed/failed';
COMMENT ON COLUMN aud_ai_analysis_jobs.progress IS '进度百分比（已处理单元批次/总批次）';
COMMENT ON COLUMN aud_ai_analysis_jobs.error_msg IS '失败原因';
COMMENT ON COLUMN aud_ai_analysis_jobs.created_by IS '触发人管理员ID（patrol 为 0）';
CREATE INDEX IF NOT EXISTS idx_aud_ai_analysis_jobs_status ON aud_ai_analysis_jobs (status, created_at);

-- 3c. 配置迁移：sys_plugins.config → sys_options（仅搬运插件配置中已有的键，空值跳过）
INSERT INTO sys_options ("key", value, category, description, is_public)
SELECT 'ai_analysis_base_url', config->>'ai_base_url', 'ai_analysis', '分析 AI 端点', false
FROM sys_plugins WHERE name = 'ai-req-analyze' AND COALESCE(config->>'ai_base_url', '') <> ''
ON CONFLICT ("key") DO NOTHING;

INSERT INTO sys_options ("key", value, category, description, is_public)
SELECT 'ai_analysis_endpoint_mode', config->>'ai_endpoint_mode', 'ai_analysis', '分析 AI 端点模式', false
FROM sys_plugins WHERE name = 'ai-req-analyze' AND COALESCE(config->>'ai_endpoint_mode', '') <> ''
ON CONFLICT ("key") DO NOTHING;

-- API Key 为 AES-256-GCM 密文（v1: 前缀）或历史明文，均原样搬运，读取方两者兼容
INSERT INTO sys_options ("key", value, category, description, is_public)
SELECT 'ai_analysis_api_key', config->>'ai_api_key', 'ai_analysis', '分析 AI 密钥（加密存储）', false
FROM sys_plugins WHERE name = 'ai-req-analyze' AND COALESCE(config->>'ai_api_key', '') <> ''
ON CONFLICT ("key") DO NOTHING;

INSERT INTO sys_options ("key", value, category, description, is_public)
SELECT 'ai_analysis_model', config->>'ai_model', 'ai_analysis', '分析模型', false
FROM sys_plugins WHERE name = 'ai-req-analyze' AND COALESCE(config->>'ai_model', '') <> ''
ON CONFLICT ("key") DO NOTHING;

INSERT INTO sys_options ("key", value, category, description, is_public)
SELECT 'ai_analysis_timeout_ms', config->>'ai_timeout_ms', 'ai_analysis', '分析 AI 调用超时（毫秒）', false
FROM sys_plugins WHERE name = 'ai-req-analyze' AND COALESCE(config->>'ai_timeout_ms', '') <> ''
ON CONFLICT ("key") DO NOTHING;

INSERT INTO sys_options ("key", value, category, description, is_public)
SELECT 'ai_analysis_max_tokens', config->>'ai_max_tokens', 'ai_analysis', '分析 AI 单次输出上限', false
FROM sys_plugins WHERE name = 'ai-req-analyze' AND COALESCE(config->>'ai_max_tokens', '') <> ''
ON CONFLICT ("key") DO NOTHING;

INSERT INTO sys_options ("key", value, category, description, is_public)
SELECT 'ai_analysis_agent_prompt', config->>'agent_prompt', 'ai_analysis', '分析智能体自定义提示词', false
FROM sys_plugins WHERE name = 'ai-req-analyze' AND COALESCE(config->>'agent_prompt', '') <> ''
ON CONFLICT ("key") DO NOTHING;

INSERT INTO sys_options ("key", value, category, description, is_public)
SELECT 'ai_analysis_units_per_batch', config->>'units_per_batch', 'ai_analysis', '每批分析单元数', false
FROM sys_plugins WHERE name = 'ai-req-analyze' AND COALESCE(config->>'units_per_batch', '') <> ''
ON CONFLICT ("key") DO NOTHING;

INSERT INTO sys_options ("key", value, category, description, is_public)
SELECT 'ai_analysis_snippet_max_len', config->>'snippet_max_len', 'ai_analysis', '单元截断长度（字符）', false
FROM sys_plugins WHERE name = 'ai-req-analyze' AND COALESCE(config->>'snippet_max_len', '') <> ''
ON CONFLICT ("key") DO NOTHING;

INSERT INTO sys_options ("key", value, category, description, is_public)
SELECT 'ai_analysis_custom_rules', config->>'custom_rules', 'ai_analysis', '自定义合规规则', false
FROM sys_plugins WHERE name = 'ai-req-analyze' AND COALESCE(config->>'custom_rules', '') <> ''
ON CONFLICT ("key") DO NOTHING;

INSERT INTO sys_options ("key", value, category, description, is_public)
SELECT 'ai_analysis_patrol_enabled', config->>'patrol_enabled', 'ai_analysis', '启用定时巡查', false
FROM sys_plugins WHERE name = 'ai-req-analyze' AND COALESCE(config->>'patrol_enabled', '') <> ''
ON CONFLICT ("key") DO NOTHING;

INSERT INTO sys_options ("key", value, category, description, is_public)
SELECT 'ai_analysis_patrol_cron', config->>'patrol_cron', 'ai_analysis', '巡查计划（cron）', false
FROM sys_plugins WHERE name = 'ai-req-analyze' AND COALESCE(config->>'patrol_cron', '') <> ''
ON CONFLICT ("key") DO NOTHING;

INSERT INTO sys_options ("key", value, category, description, is_public)
SELECT 'ai_analysis_patrol_scope', config->>'patrol_scope', 'ai_analysis', '巡查范围（租户列表 JSON）', false
FROM sys_plugins WHERE name = 'ai-req-analyze' AND COALESCE(config->>'patrol_scope', '') <> ''
ON CONFLICT ("key") DO NOTHING;

INSERT INTO sys_options ("key", value, category, description, is_public)
SELECT 'ai_analysis_patrol_window_h', config->>'patrol_window_h', 'ai_analysis', '巡查回看窗口（小时）', false
FROM sys_plugins WHERE name = 'ai-req-analyze' AND COALESCE(config->>'patrol_window_h', '') <> ''
ON CONFLICT ("key") DO NOTHING;

INSERT INTO sys_options ("key", value, category, description, is_public)
SELECT 'ai_analysis_retention_days', config->>'retention_days', 'ai_analysis', '报告保留天数', false
FROM sys_plugins WHERE name = 'ai-req-analyze' AND COALESCE(config->>'retention_days', '') <> ''
ON CONFLICT ("key") DO NOTHING;

-- 3d. 权限点改写：已授权角色无缝衔接新权限点
UPDATE sys_admin_role_perms
SET permission_point = 'ai_analysis:' || split_part(permission_point, ':', 2)
WHERE permission_point LIKE 'ai-req-analyze:%';

-- 3e. 清理插件痕迹
DELETE FROM sys_plugins WHERE name = 'ai-req-analyze';
DELETE FROM tnt_tenant_plugins WHERE plugin_name = 'ai-req-analyze';
DELETE FROM sys_options WHERE "key" LIKE 'plugin.ai-req-analyze.%';

-- +goose Down
-- ═══ 回滚顺序与 Up 相反：先撤 AI请求分析（尽力恢复插件形态），再撤计费变更 ═══
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'aud_ai_analysis_reports')
       AND NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'plg_ai_req_analyze_reports') THEN
        ALTER TABLE aud_ai_analysis_reports RENAME TO plg_ai_req_analyze_reports;
        ALTER INDEX IF EXISTS idx_aud_ai_analysis_reports_tenant RENAME TO idx_plg_ai_req_analyze_reports_tenant;
        ALTER INDEX IF EXISTS idx_aud_ai_analysis_reports_created RENAME TO idx_plg_ai_req_analyze_reports_created;
        ALTER SEQUENCE IF EXISTS aud_ai_analysis_reports_id_seq RENAME TO plg_ai_req_analyze_reports_id_seq;
        COMMENT ON TABLE plg_ai_req_analyze_reports IS 'AI请求分析-报告主表（ai-req-analyze 插件）';
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'aud_ai_analysis_findings')
       AND NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'plg_ai_req_analyze_findings') THEN
        ALTER TABLE aud_ai_analysis_findings RENAME TO plg_ai_req_analyze_findings;
        ALTER INDEX IF EXISTS idx_aud_ai_analysis_findings_report RENAME TO idx_plg_ai_req_analyze_findings_report;
        ALTER INDEX IF EXISTS idx_aud_ai_analysis_findings_session RENAME TO idx_plg_ai_req_analyze_findings_session;
        ALTER SEQUENCE IF EXISTS aud_ai_analysis_findings_id_seq RENAME TO plg_ai_req_analyze_findings_id_seq;
        COMMENT ON TABLE plg_ai_req_analyze_findings IS 'AI请求分析-发现明细（ai-req-analyze 插件）';
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'aud_ai_analysis_jobs')
       AND NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'plg_ai_req_analyze_jobs') THEN
        ALTER TABLE aud_ai_analysis_jobs RENAME TO plg_ai_req_analyze_jobs;
        ALTER INDEX IF EXISTS idx_aud_ai_analysis_jobs_status RENAME TO idx_plg_ai_req_analyze_jobs_status;
        ALTER SEQUENCE IF EXISTS aud_ai_analysis_jobs_id_seq RENAME TO plg_ai_req_analyze_jobs_id_seq;
        COMMENT ON TABLE plg_ai_req_analyze_jobs IS 'AI请求分析-异步任务（ai-req-analyze 插件）';
    END IF;
END
$$;
-- +goose StatementEnd

UPDATE sys_admin_role_perms
SET permission_point = 'ai-req-analyze:' || split_part(permission_point, ':', 2)
WHERE permission_point LIKE 'ai_analysis:%';

DELETE FROM sys_options WHERE "key" IN (
    'ai_analysis_base_url', 'ai_analysis_endpoint_mode', 'ai_analysis_api_key', 'ai_analysis_model',
    'ai_analysis_timeout_ms', 'ai_analysis_max_tokens', 'ai_analysis_agent_prompt', 'ai_analysis_custom_rules',
    'ai_analysis_units_per_batch', 'ai_analysis_snippet_max_len', 'ai_analysis_patrol_enabled',
    'ai_analysis_patrol_cron', 'ai_analysis_patrol_scope', 'ai_analysis_patrol_window_h', 'ai_analysis_retention_days'
);

ALTER TABLE bil_usage_logs
    DROP COLUMN IF EXISTS tool_surcharge_cost,
    DROP COLUMN IF EXISTS tool_usage;

COMMENT ON COLUMN mdl_pricing.billing_mode IS '计费模式：token=按量；per_request=按次；tiered=阶梯；per_second=按秒（视频）；special=特殊计费';
