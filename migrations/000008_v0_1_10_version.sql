-- +goose Up

-- 模型分组增加"默认分组"标记，标记为默认的分组在新租户注册时自动关联

ALTER TABLE mdl_model_groups ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT FALSE;
COMMENT ON COLUMN mdl_model_groups.is_default IS '是否为新租户默认模型组，注册时自动关联';

-- ============================================================
-- sys_agreements — 用户协议/政策版本管理
-- ============================================================
CREATE TABLE sys_agreements (
    id              BIGSERIAL PRIMARY KEY,
    code            VARCHAR(50)  NOT NULL,
    version         VARCHAR(50)  NOT NULL,
    title           VARCHAR(200) NOT NULL,
    content         TEXT         NOT NULL,
    summary         VARCHAR(500) NOT NULL DEFAULT '',
    status          VARCHAR(20)  NOT NULL DEFAULT 'draft',
    is_current      BOOLEAN      NOT NULL DEFAULT FALSE,
    force_accept    BOOLEAN      NOT NULL DEFAULT TRUE,
    published_at    TIMESTAMPTZ,
    created_by      BIGINT       NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX uk_sys_agreements_current
    ON sys_agreements (code) WHERE is_current = TRUE;

CREATE UNIQUE INDEX uk_sys_agreements_code_version
    ON sys_agreements (code, version);

CREATE INDEX idx_sys_agreements_code_status
    ON sys_agreements (code, status);

COMMENT ON TABLE  sys_agreements IS '用户协议/政策版本管理';
COMMENT ON COLUMN sys_agreements.id IS '主键ID';
COMMENT ON COLUMN sys_agreements.code IS '协议标识码：terms(用户协议) / privacy(隐私政策)';
COMMENT ON COLUMN sys_agreements.version IS '版本号，如 1.0、2.0';
COMMENT ON COLUMN sys_agreements.title IS '协议标题';
COMMENT ON COLUMN sys_agreements.content IS '协议正文（Markdown）';
COMMENT ON COLUMN sys_agreements.summary IS '版本变更摘要';
COMMENT ON COLUMN sys_agreements.status IS '状态：draft(草稿) / published(已发布) / archived(已归档)';
COMMENT ON COLUMN sys_agreements.is_current IS '是否为该标识码的当前生效版本（每个code仅一条）';
COMMENT ON COLUMN sys_agreements.force_accept IS '是否强制用户接受（true=登录后必须接受才能继续）';
COMMENT ON COLUMN sys_agreements.published_at IS '发布时间';
COMMENT ON COLUMN sys_agreements.created_by IS '创建的管理员ID';
COMMENT ON COLUMN sys_agreements.created_at IS '创建时间';
COMMENT ON COLUMN sys_agreements.updated_at IS '更新时间';

-- ============================================================
-- sys_agreement_acceptances — 用户协议接受记录
-- ============================================================
CREATE TABLE sys_agreement_acceptances (
    id              BIGSERIAL PRIMARY KEY,
    agreement_id    BIGINT       NOT NULL,
    user_type       VARCHAR(20)  NOT NULL,
    user_id         BIGINT       NOT NULL,
    ip_address      VARCHAR(45)  NOT NULL DEFAULT '',
    user_agent      TEXT         NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX uk_sys_agreement_acceptances_user
    ON sys_agreement_acceptances (agreement_id, user_type, user_id);

CREATE INDEX idx_sys_agreement_acceptances_user_lookup
    ON sys_agreement_acceptances (user_type, user_id, agreement_id);

CREATE INDEX idx_sys_agreement_acceptances_created_brin
    ON sys_agreement_acceptances USING BRIN (created_at);

COMMENT ON TABLE  sys_agreement_acceptances IS '用户协议接受记录';
COMMENT ON COLUMN sys_agreement_acceptances.id IS '主键ID';
COMMENT ON COLUMN sys_agreement_acceptances.agreement_id IS '关联协议版本ID';
COMMENT ON COLUMN sys_agreement_acceptances.user_type IS '用户类型：admin(管理员) / tenant(租户用户)';
COMMENT ON COLUMN sys_agreement_acceptances.user_id IS '用户ID';
COMMENT ON COLUMN sys_agreement_acceptances.ip_address IS '接受时的IP地址';
COMMENT ON COLUMN sys_agreement_acceptances.user_agent IS '接受时的浏览器User-Agent';
COMMENT ON COLUMN sys_agreement_acceptances.created_at IS '接受时间';

-- +goose Down
DROP TABLE IF EXISTS sys_agreement_acceptances;
DROP TABLE IF EXISTS sys_agreements;

ALTER TABLE mdl_model_groups DROP COLUMN IF EXISTS is_default;
