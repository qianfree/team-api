-- +goose Up
-- Consolidated initial schema for open-source release.
-- Generated on 2026-05-10.

-- ============================================================
-- Functions
-- ============================================================

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION cleanup_channel_references()
 RETURNS trigger
 LANGUAGE plpgsql
AS $$
DECLARE
    channel_id_int INTEGER := OLD.id;
BEGIN
    -- 清理租户默认渠道范围
    UPDATE tnt_tenants
    SET default_channel_scope = (
        SELECT jsonb_agg(elem::int)
        FROM jsonb_array_elements_text(default_channel_scope) elem
        WHERE elem::int != channel_id_int
    )
    WHERE default_channel_scope IS NOT NULL
      AND jsonb_array_length(default_channel_scope) > 0
      AND default_channel_scope::text LIKE '%' || channel_id_int::text || '%';

    -- 清理租户-模型渠道范围
    UPDATE mdl_tenant_models
    SET channel_scope = (
        SELECT jsonb_agg(elem::int)
        FROM jsonb_array_elements_text(channel_scope) elem
        WHERE elem::int != channel_id_int
    )
    WHERE channel_scope IS NOT NULL
      AND jsonb_array_length(channel_scope) > 0
      AND channel_scope::text LIKE '%' || channel_id_int::text || '%';

    RETURN OLD;
END;
$$
;
-- +goose StatementEnd

-- ============================================================
-- Tables
-- ============================================================

CREATE TABLE api_key_model_scopes (
    id                                       BIGSERIAL PRIMARY KEY,
    api_key_id                               BIGINT NOT NULL,
    model_name                               VARCHAR(100) NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_api_key_model_scopes UNIQUE (api_key_id, model_name)
);

CREATE TABLE api_keys (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    name                                     VARCHAR(100) NOT NULL,
    encrypted_key                            TEXT NOT NULL,
    key_prefix                               VARCHAR(12) NOT NULL,
    scope                                    VARCHAR(30) DEFAULT 'full' NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'active' NOT NULL,
    expires_at                               TIMESTAMPTZ,
    rate_limit_qps                           INTEGER,
    rate_limit_concurrency                   INTEGER,
    ip_whitelist                             TEXT[],
    total_quota                              NUMERIC(20,10),
    used_quota                               NUMERIC(20,10) DEFAULT 0 NOT NULL,
    project_id                               BIGINT,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    key_type                                 VARCHAR(20) DEFAULT 'personal' NOT NULL
);

CREATE TABLE aud_login_history (
    id                                       BIGSERIAL PRIMARY KEY,
    user_type                                VARCHAR(20) NOT NULL,
    user_id                                  BIGINT NOT NULL,
    tenant_id                                BIGINT,
    login_method                             VARCHAR(30) DEFAULT 'password' NOT NULL,
    ip_address                               VARCHAR(45) NOT NULL,
    user_agent                               TEXT,
    device_fingerprint                       VARCHAR(128),
    location                                 VARCHAR(200),
    is_new_device                            BOOLEAN DEFAULT false NOT NULL,
    success                                  BOOLEAN DEFAULT true NOT NULL,
    fail_reason                              VARCHAR(200),
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE aud_operation_logs (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT,
    user_id                                  BIGINT NOT NULL,
    user_type                                VARCHAR(20) NOT NULL,
    action                                   VARCHAR(100) NOT NULL,
    resource_type                            VARCHAR(50),
    resource_id                              BIGINT,
    detail                                   JSONB,
    ip_address                               VARCHAR(45),
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    changes_json                             JSONB
);

CREATE TABLE aud_request_logs (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    api_key_id                               BIGINT,
    request_id                               VARCHAR(64) NOT NULL,
    method                                   VARCHAR(10) NOT NULL,
    path                                     VARCHAR(500) NOT NULL,
    query_params                             TEXT,
    status_code                              INTEGER,
    client_ip                                VARCHAR(45),
    user_agent                               TEXT,
    request_body                             TEXT,
    response_body                            TEXT,
    latency_ms                               INTEGER,
    audit_level                              VARCHAR(20) DEFAULT 'full' NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    tenant_request_body                      TEXT,
    tenant_response_body                     TEXT,
    tenant_audit_level                       VARCHAR(20) DEFAULT 'full' NOT NULL,
    project_id                               BIGINT,
    first_token_ms                           INTEGER,
    request_headers                          JSONB,
    response_headers                         JSONB,
    forwarding_trace                         JSONB
);

CREATE TABLE aud_sensitive_access_logs (
    id                                       BIGSERIAL PRIMARY KEY,
    user_id                                  BIGINT NOT NULL,
    user_type                                VARCHAR(20) NOT NULL,
    resource_type                            VARCHAR(50) NOT NULL,
    resource_id                              BIGINT,
    action                                   VARCHAR(50) NOT NULL,
    reason                                   TEXT,
    ip_address                               VARCHAR(45),
    user_agent                               TEXT,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE bil_daily_revenue_summary (
    id                                       BIGSERIAL PRIMARY KEY,
    date                                     DATE NOT NULL,
    total_recharge                           NUMERIC(20,10) DEFAULT 0 NOT NULL,
    total_consumption                        NUMERIC(20,10) DEFAULT 0 NOT NULL,
    net_revenue                              NUMERIC(20,10) DEFAULT 0 NOT NULL,
    new_orders                               INTEGER DEFAULT 0 NOT NULL,
    paid_orders                              INTEGER DEFAULT 0 NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE bil_daily_usage_summary (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    date                                     DATE NOT NULL,
    total_requests                           BIGINT DEFAULT 0 NOT NULL,
    total_tokens                             BIGINT DEFAULT 0 NOT NULL,
    total_cost                               NUMERIC(20,10) DEFAULT 0 NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE bil_monthly_revenue_summary (
    id                                       BIGSERIAL PRIMARY KEY,
    month                                    DATE NOT NULL,
    total_recharge                           NUMERIC(20,10) DEFAULT 0 NOT NULL,
    total_consumption                        NUMERIC(20,10) DEFAULT 0 NOT NULL,
    net_revenue                              NUMERIC(20,10) DEFAULT 0 NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE bil_monthly_usage_summary (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    month                                    DATE NOT NULL,
    total_requests                           BIGINT DEFAULT 0 NOT NULL,
    total_tokens                             BIGINT DEFAULT 0 NOT NULL,
    total_cost                               NUMERIC(20,10) DEFAULT 0 NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE bil_records (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    api_key_id                               BIGINT,
    channel_id                               BIGINT,
    model_name                               VARCHAR(100) NOT NULL,
    request_id                               VARCHAR(64) NOT NULL,
    relay_mode                               VARCHAR(30),
    input_tokens                             INTEGER DEFAULT 0 NOT NULL,
    output_tokens                            INTEGER DEFAULT 0 NOT NULL,
    input_price                              NUMERIC(20,10),
    output_price                             NUMERIC(20,10),
    total_cost                               NUMERIC(20,10) NOT NULL,
    currency                                 VARCHAR(3) DEFAULT 'USD' NOT NULL,
    status                                   VARCHAR(20) NOT NULL,
    settled_at                               TIMESTAMPTZ,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    billing_mode                             VARCHAR(20) DEFAULT 'token' NOT NULL,
    effective_input_price                    NUMERIC(20,10),
    effective_output_price                   NUMERIC(20,10),
    discount_ratio                           NUMERIC(5,4),
    billing_input_multiplier                 NUMERIC(10,4),
    billing_output_multiplier                NUMERIC(10,4),
    cache_creation_tokens                    INTEGER DEFAULT 0 NOT NULL,
    cache_read_tokens                        INTEGER DEFAULT 0 NOT NULL,
    cache_creation_cost                      NUMERIC(20,10) DEFAULT 0 NOT NULL,
    cache_read_cost                          NUMERIC(20,10) DEFAULT 0 NOT NULL,
    model_multiplier                         NUMERIC(10,4) DEFAULT 1.0000 NOT NULL,
    tenant_multiplier                        NUMERIC(10,4) DEFAULT 1.0000 NOT NULL,
    base_input_price                         NUMERIC(20,10) DEFAULT 0 NOT NULL,
    base_output_price                        NUMERIC(20,10) DEFAULT 0 NOT NULL,
    billing_snapshot                         JSONB
);


CREATE TABLE bil_transactions (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    wallet_id                                BIGINT NOT NULL,
    type                                     VARCHAR(30) NOT NULL,
    amount                                   NUMERIC(20,10) NOT NULL,
    balance_after                            NUMERIC(20,10) NOT NULL,
    frozen_after                             NUMERIC(20,10) DEFAULT 0 NOT NULL,
    related_id                               BIGINT,
    related_type                             VARCHAR(30),
    description                              TEXT,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE bil_wallets (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    balance                                  NUMERIC(20,10) DEFAULT 0 NOT NULL,
    frozen_balance                           NUMERIC(20,10) DEFAULT 0 NOT NULL,
    warning_threshold                        NUMERIC(20,10),
    currency                                 VARCHAR(3) DEFAULT 'USD' NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_bil_wallets_tenant UNIQUE (tenant_id)
);

CREATE TABLE chn_abilities (
    id                                       BIGSERIAL PRIMARY KEY,
    channel_id                               BIGINT NOT NULL,
    model_name                               VARCHAR(100) NOT NULL,
    upstream_model                           VARCHAR(100),
    enabled                                  BOOLEAN DEFAULT true NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_chn_abilities UNIQUE (channel_id, model_name)
);

CREATE TABLE chn_channel_affinities (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    model_name                               VARCHAR(100) NOT NULL,
    channel_id                               BIGINT NOT NULL,
    hit_count                                INTEGER DEFAULT 1 NOT NULL,
    expires_at                               TIMESTAMPTZ NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_chn_affinities UNIQUE (tenant_id, user_id, model_name)
);

CREATE TABLE chn_channel_keys (
    id                                       BIGSERIAL PRIMARY KEY,
    channel_id                               BIGINT NOT NULL,
    name                                     VARCHAR(100),
    encrypted_key                            TEXT NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'active' NOT NULL,
    last_used_at                             TIMESTAMPTZ,
    last_error                               TEXT,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    key_type                                 VARCHAR(20) NOT NULL DEFAULT 'apikey',
    token_expires_at                         TIMESTAMPTZ,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE chn_channels (
    id                                       BIGSERIAL PRIMARY KEY,
    name                                     VARCHAR(100) NOT NULL,
    type                                     INTEGER NOT NULL,
    base_url                                 VARCHAR(500) NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'active' NOT NULL,
    priority                                 INTEGER DEFAULT 0 NOT NULL,
    weight                                   INTEGER DEFAULT 100 NOT NULL,
    max_concurrency                          INTEGER DEFAULT 100 NOT NULL,
    settings                                 JSONB DEFAULT '{}'::jsonb,
    test_model                               VARCHAR(100),
    remark                                   TEXT,
    created_by                               BIGINT,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    is_vip                                   BOOLEAN DEFAULT false NOT NULL,
    sharing_threshold                        NUMERIC(5,2) DEFAULT 0.6,
    preemption_threshold                     NUMERIC(5,2) DEFAULT 0.8,
    borrowing_cooldown_seconds               INTEGER DEFAULT 30,
    auto_disabled                            SMALLINT DEFAULT 0 NOT NULL
);

CREATE TABLE chn_health_scores (
    id                                       BIGSERIAL PRIMARY KEY,
    channel_id                               BIGINT NOT NULL,
    success_rate                             NUMERIC(5,2),
    latency_ms                               NUMERIC(10,2),
    stability_score                          NUMERIC(5,2),
    consecutive_failures                     INTEGER DEFAULT 0 NOT NULL,
    health_score                             NUMERIC(5,2),
    calculated_at                            TIMESTAMPTZ DEFAULT now() NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_chn_health_scores UNIQUE (channel_id)
);

CREATE TABLE chn_health_snapshots (
    id                                       BIGSERIAL PRIMARY KEY,
    channel_id                               BIGINT NOT NULL,
    health_score                             NUMERIC(6,2) NOT NULL,
    success_rate                             NUMERIC(6,2) NOT NULL,
    latency_ms                               NUMERIC(10,2) NOT NULL,
    stability_score                          NUMERIC(6,2) NOT NULL,
    consecutive_failures                     INTEGER DEFAULT 0 NOT NULL,
    snapshot_at                              TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE clg_changelogs (
    id                                       BIGSERIAL PRIMARY KEY,
    version                                  VARCHAR(50) NOT NULL,
    title                                    VARCHAR(200) NOT NULL,
    content                                  TEXT NOT NULL,
    type                                     VARCHAR(20) DEFAULT 'feature' NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'draft' NOT NULL,
    published_at                             TIMESTAMPTZ,
    created_by                               BIGINT NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);


CREATE TABLE ord_promo_code_usages (
    id                                       BIGSERIAL PRIMARY KEY,
    promo_code_id                            BIGINT NOT NULL,
    tenant_id                                BIGINT NOT NULL,
    order_id                                 BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    discount_amount                          NUMERIC(20,10) NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE ord_promo_codes (
    id                                       BIGSERIAL PRIMARY KEY,
    code                                     VARCHAR(50) NOT NULL,
    name                                     VARCHAR(100) NOT NULL,
    type                                     VARCHAR(20) NOT NULL,
    discount_value                           NUMERIC(20,10) NOT NULL,
    min_amount                               NUMERIC(20,10) DEFAULT 0 NOT NULL,
    max_discount                             NUMERIC(20,10) DEFAULT 0 NOT NULL,
    total_count                              INTEGER DEFAULT 0 NOT NULL,
    used_count                               INTEGER DEFAULT 0 NOT NULL,
    per_user_limit                           INTEGER DEFAULT 1 NOT NULL,
    valid_from                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    valid_to                                 TIMESTAMPTZ DEFAULT (now() + '1 year'::interval) NOT NULL,
    plan_ids                                 BIGINT[],
    status                                   VARCHAR(20) DEFAULT 'active' NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_ord_promo_codes_code UNIQUE (code)
);

CREATE TABLE ord_redemptions (
    id                                       BIGSERIAL PRIMARY KEY,
    code                                     VARCHAR(50) NOT NULL,
    type                                     VARCHAR(20) NOT NULL,
    value                                    NUMERIC(20,10) DEFAULT 0 NOT NULL,
    plan_id                                  BIGINT,
    duration_days                            INTEGER DEFAULT 0 NOT NULL,
    max_uses                                 INTEGER DEFAULT 1 NOT NULL,
    used_count                               INTEGER DEFAULT 0 NOT NULL,
    redeemed_by                              BIGINT,
    redeemed_at                              TIMESTAMPTZ,
    expires_at                               TIMESTAMPTZ,
    status                                   VARCHAR(20) DEFAULT 'active' NOT NULL,
    batch_no                                 VARCHAR(50),
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_ord_redemptions_code UNIQUE (code)
);

CREATE TABLE spt_feedbacks (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    category                                 VARCHAR(30) DEFAULT 'suggestion' NOT NULL,
    title                                    VARCHAR(200) NOT NULL,
    description                              TEXT NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'pending' NOT NULL,
    priority                                 VARCHAR(10) DEFAULT 'normal' NOT NULL,
    admin_reply                              TEXT,
    admin_reply_by                           BIGINT,
    admin_reply_at                           TIMESTAMPTZ,
    resolution                               VARCHAR(200),
    tags                                     JSONB DEFAULT '[]'::jsonb,
    metadata                                 JSONB DEFAULT '{}'::jsonb,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE fil_files (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT,
    user_id                                  BIGINT NOT NULL,
    filename                                 VARCHAR(255) NOT NULL,
    original_name                            VARCHAR(255),
    mime_type                                VARCHAR(100),
    size                                     BIGINT NOT NULL,
    storage_provider                         VARCHAR(20) NOT NULL,
    storage_path                             VARCHAR(500) NOT NULL,
    virus_scan_status                        VARCHAR(20) DEFAULT 'pending' NOT NULL,
    checksum                                 VARCHAR(64),
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE spt_articles (
    id                                       BIGSERIAL PRIMARY KEY,
    category_id                              BIGINT NOT NULL,
    title                                    VARCHAR(200) NOT NULL,
    slug                                     VARCHAR(200) NOT NULL,
    content                                  TEXT NOT NULL,
    summary                                  VARCHAR(500),
    status                                   VARCHAR(20) DEFAULT 'draft' NOT NULL,
    author_id                                BIGINT NOT NULL,
    view_count                               INTEGER DEFAULT 0 NOT NULL,
    sort_order                               INTEGER DEFAULT 0 NOT NULL,
    keywords                                 JSONB DEFAULT '[]'::jsonb,
    published_at                             TIMESTAMPTZ,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_spt_articles_slug UNIQUE (slug)
);

CREATE TABLE spt_categories (
    id                                       BIGSERIAL PRIMARY KEY,
    parent_id                                BIGINT DEFAULT 0 NOT NULL,
    name                                     VARCHAR(100) NOT NULL,
    slug                                     VARCHAR(100) NOT NULL,
    description                              VARCHAR(500),
    sort_order                               INTEGER DEFAULT 0 NOT NULL,
    icon                                     VARCHAR(50),
    is_visible                               BOOLEAN DEFAULT true NOT NULL,
    article_count                            INTEGER DEFAULT 0 NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_spt_categories_slug UNIQUE (slug)
);


CREATE TABLE mdl_models (
    id                                       BIGSERIAL PRIMARY KEY,
    model_id                                 VARCHAR(100) NOT NULL,
    model_name                               VARCHAR(200),
    category                                 VARCHAR(30) NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'active' NOT NULL,
    max_context_tokens                       INTEGER,
    max_output_tokens                        INTEGER,
    description                              TEXT,
    tags                                     TEXT[],
    capabilities                             JSONB DEFAULT '{}'::jsonb,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    deprecated_at                            TIMESTAMPTZ,
    sunset_date                              DATE,
    replacement_model                        VARCHAR(100),
    CONSTRAINT uk_mdl_models_model_id UNIQUE (model_id)
);

CREATE TABLE mdl_pricing (
    id                                       BIGSERIAL PRIMARY KEY,
    model_id                                 BIGINT NOT NULL,
    billing_mode                             VARCHAR(20) DEFAULT 'token' NOT NULL,
    min_tokens                               BIGINT DEFAULT 0 NOT NULL,
    max_tokens                               BIGINT,
    input_price                              NUMERIC(20,10) DEFAULT 0 NOT NULL,
    output_price                             NUMERIC(20,10) DEFAULT 0 NOT NULL,
    per_request_price                        NUMERIC(20,10),
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    cache_read_price                         NUMERIC(20,10) DEFAULT 0 NOT NULL,
    cache_creation_price                     NUMERIC(20,10) DEFAULT 0 NOT NULL
);

CREATE TABLE mdl_tenant_models (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    model_id                                 BIGINT NOT NULL,
    enabled                                  BOOLEAN DEFAULT true NOT NULL,
    custom_input_price                       NUMERIC(20,10),
    custom_output_price                      NUMERIC(20,10),
    multiplier                               NUMERIC(10,4) DEFAULT 1.0 NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    billing_mode                             VARCHAR(20) DEFAULT NULL,
    per_request_price                        NUMERIC(20,10) DEFAULT NULL::numeric,
    discount_ratio                           NUMERIC(5,4) DEFAULT NULL::numeric,
    max_concurrency                          INTEGER,
    channel_scope                            JSONB,
    CONSTRAINT chk_discount_ratio_range CHECK (((discount_ratio IS NULL) OR ((discount_ratio > (0)::numeric) AND (discount_ratio <= 1.0)))),
    CONSTRAINT uk_mdl_tenant_models UNIQUE (tenant_id, model_id)
);

CREATE TABLE ntf_announcements (
    id                                       BIGSERIAL PRIMARY KEY,
    title                                    VARCHAR(255) NOT NULL,
    type                                     VARCHAR(50) DEFAULT 'info' NOT NULL,
    content                                  TEXT NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'draft' NOT NULL,
    is_pinned                                SMALLINT DEFAULT 0 NOT NULL,
    display_position                         VARCHAR(50) DEFAULT 'console' NOT NULL,
    effective_at                             TIMESTAMPTZ,
    expires_at                               TIMESTAMPTZ,
    created_by                               BIGINT NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE ntf_messages (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    user_id                                  BIGINT,
    type                                     VARCHAR(50) NOT NULL,
    title                                    VARCHAR(255) NOT NULL,
    content                                  TEXT NOT NULL,
    channel                                  VARCHAR(20) DEFAULT 'in_app' NOT NULL,
    is_read                                  SMALLINT DEFAULT 0 NOT NULL,
    is_broadcast                             SMALLINT DEFAULT 0 NOT NULL,
    metadata                                 JSONB,
    target_roles                             VARCHAR(255) DEFAULT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE ntf_preferences (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT,
    user_id                                  BIGINT,
    scope                                    VARCHAR(20) DEFAULT 'user' NOT NULL,
    preferences                              JSONB DEFAULT '{}'::jsonb NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_ntf_preferences UNIQUE (tenant_id, user_id, scope)
);

CREATE TABLE ntf_read_status (
    id                                       BIGSERIAL PRIMARY KEY,
    message_id                               BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    read_at                                  TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_ntf_read_status UNIQUE (message_id, user_id)
);

CREATE TABLE ntf_send_log (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT,
    user_id                                  BIGINT,
    template_code                            VARCHAR(50) NOT NULL,
    channel                                  VARCHAR(20) NOT NULL,
    recipient                                VARCHAR(200) NOT NULL,
    subject                                  VARCHAR(200),
    body                                     TEXT,
    status                                   VARCHAR(20) NOT NULL,
    error_message                            TEXT,
    sent_at                                  TIMESTAMPTZ,
    retry_count                              INTEGER DEFAULT 0 NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE ntf_templates (
    id                                       BIGSERIAL PRIMARY KEY,
    code                                     VARCHAR(50) NOT NULL,
    channel                                  VARCHAR(20) NOT NULL,
    subject                                  VARCHAR(200),
    body_template                            TEXT NOT NULL,
    variables                                JSONB DEFAULT '[]'::jsonb,
    status                                   VARCHAR(20) DEFAULT 'active' NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_ntf_templates_code UNIQUE (code)
);

CREATE TABLE opn_apps (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    name                                     VARCHAR(100) NOT NULL,
    description                              VARCHAR(500),
    app_id                                   VARCHAR(32) NOT NULL,
    app_secret_hash                          VARCHAR(255) NOT NULL,
    permissions                              JSONB DEFAULT '[]'::jsonb NOT NULL,
    ip_whitelist                             JSONB DEFAULT '[]'::jsonb,
    callback_url                             VARCHAR(500),
    is_sandbox                               BOOLEAN DEFAULT false NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'active' NOT NULL,
    rate_limit                               INTEGER DEFAULT 60 NOT NULL,
    last_used_at                             TIMESTAMPTZ,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_opn_apps_app_id UNIQUE (app_id)
);


CREATE TABLE ops_alert_events (
    id                                       BIGSERIAL PRIMARY KEY,
    rule_id                                  BIGINT NOT NULL,
    rule_name                                VARCHAR(100) NOT NULL,
    metric_type                              VARCHAR(50) NOT NULL,
    level                                    VARCHAR(20) NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'firing' NOT NULL,
    trigger_value                            NUMERIC(20,10),
    threshold_value                          NUMERIC(20,10),
    trigger_message                          TEXT,
    acknowledged_by                          BIGINT,
    acknowledged_at                          TIMESTAMPTZ,
    resolve_notes                            TEXT,
    resolved_by                              BIGINT,
    resolved_at                              TIMESTAMPTZ,
    notified_methods                         TEXT[],
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE ops_alert_rules (
    id                                       BIGSERIAL PRIMARY KEY,
    name                                     VARCHAR(100) NOT NULL,
    metric_type                              VARCHAR(50) NOT NULL,
    condition                                VARCHAR(20) NOT NULL,
    threshold                                NUMERIC(20,10) NOT NULL,
    duration_seconds                         INTEGER DEFAULT 0 NOT NULL,
    notification_methods                     TEXT[] DEFAULT ARRAY['email','in_app'] NOT NULL,
    webhook_url                              VARCHAR(500),
    level                                    VARCHAR(20) DEFAULT 'warning' NOT NULL,
    is_enabled                               BOOLEAN DEFAULT true NOT NULL,
    cooldown_seconds                         INTEGER DEFAULT 300 NOT NULL,
    last_triggered_at                        TIMESTAMPTZ,
    notify_user_ids                          BIGINT[],
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE sys_options (
    id                                       BIGSERIAL PRIMARY KEY,
    key                                      VARCHAR(100) NOT NULL,
    value                                    TEXT,
    description                              TEXT,
    category                                 VARCHAR(50),
    is_public                                BOOLEAN DEFAULT false NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_sys_options_key UNIQUE (key)
);

CREATE TABLE ord_orders (
    id                                       BIGSERIAL PRIMARY KEY,
    order_no                                 VARCHAR(64) NOT NULL,
    tenant_id                                BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    order_type                               VARCHAR(20) NOT NULL,
    plan_id                                  BIGINT,
    amount                                   NUMERIC(20,10) NOT NULL,
    discount_amount                          NUMERIC(20,10) DEFAULT 0 NOT NULL,
    final_amount                             NUMERIC(20,10) NOT NULL,
    currency                                 VARCHAR(3) DEFAULT 'USD' NOT NULL,
    payment_channel                          VARCHAR(20) DEFAULT 'mock' NOT NULL,
    payment_method                           VARCHAR(100),
    payment_no                               VARCHAR(200),
    status                                   VARCHAR(20) DEFAULT 'pending' NOT NULL,
    paid_at                                  TIMESTAMPTZ,
    fulfilled_at                             TIMESTAMPTZ,
    expired_at                               TIMESTAMPTZ,
    cancelled_at                             TIMESTAMPTZ,
    related_order_id                         BIGINT,
    description                              TEXT,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_ord_orders_order_no UNIQUE (order_no)
);

CREATE TABLE ord_payment_channels (
    id                                       BIGSERIAL PRIMARY KEY,
    channel                                  VARCHAR(20) NOT NULL,
    name                                     VARCHAR(100) NOT NULL,
    config                                   JSONB DEFAULT '{}'::jsonb NOT NULL,
    is_enabled                               BOOLEAN DEFAULT false NOT NULL,
    sort_order                               INTEGER DEFAULT 0 NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    payment_type                             VARCHAR(20) DEFAULT '' NOT NULL,
    callback_url                             VARCHAR(500) DEFAULT '' NOT NULL,
    return_url                               VARCHAR(500) DEFAULT '' NOT NULL
);

CREATE TABLE ord_refunds (
    id                                       BIGSERIAL PRIMARY KEY,
    order_id                                 BIGINT NOT NULL,
    tenant_id                                BIGINT NOT NULL,
    amount                                   NUMERIC(20,10) NOT NULL,
    reason                                   TEXT,
    status                                   VARCHAR(20) DEFAULT 'pending' NOT NULL,
    payment_channel                          VARCHAR(20) NOT NULL,
    payment_refund_id                        VARCHAR(200),
    approved_by                              BIGINT,
    approved_at                              TIMESTAMPTZ,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE pln_feature_flags (
    id                                       BIGSERIAL PRIMARY KEY,
    feature_key                              VARCHAR(100) NOT NULL,
    description                              TEXT,
    default_enabled                          BOOLEAN DEFAULT false NOT NULL,
    enabled                                  BOOLEAN DEFAULT false NOT NULL,
    source                                   VARCHAR(20) DEFAULT 'manual' NOT NULL,
    source_id                                BIGINT,
    tenant_id                                BIGINT,
    plan_id                                  BIGINT,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE pln_plans (
    id                                       BIGSERIAL PRIMARY KEY,
    name                                     VARCHAR(100) NOT NULL,
    identifier                               VARCHAR(50) NOT NULL,
    description                              TEXT,
    monthly_price                            NUMERIC(20,10) DEFAULT 0 NOT NULL,
    yearly_price                             NUMERIC(20,10) DEFAULT 0 NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'active' NOT NULL,
    monthly_quota_tokens                     BIGINT DEFAULT 0 NOT NULL,
    allowed_models                           TEXT[],
    is_recommended                           BOOLEAN DEFAULT false NOT NULL,
    sort_order                               INTEGER DEFAULT 0 NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_pln_plans_identifier UNIQUE (identifier)
);

CREATE TABLE pln_tenant_plans (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    plan_id                                  BIGINT NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'active' NOT NULL,
    start_at                                 TIMESTAMPTZ NOT NULL,
    end_at                                   TIMESTAMPTZ NOT NULL,
    auto_renew                               BOOLEAN DEFAULT false NOT NULL,
    monthly_quota_tokens                     BIGINT DEFAULT 0 NOT NULL,
    used_tokens                              BIGINT DEFAULT 0 NOT NULL,
    last_reset_at                            TIMESTAMPTZ,
    cancelled_at                             TIMESTAMPTZ,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE tnt_projects (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    name                                     VARCHAR(100) NOT NULL,
    description                              TEXT,
    status                                   VARCHAR(20) DEFAULT 'active' NOT NULL,
    budget                                   NUMERIC(20,10),
    created_by                               BIGINT NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);


CREATE TABLE sys_admin_data_scopes (
    id                                       BIGSERIAL PRIMARY KEY,
    admin_user_id                            BIGINT NOT NULL,
    scope_type                               VARCHAR(20) NOT NULL,
    scope_value                              TEXT,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE sys_admin_role_perms (
    id                                       BIGSERIAL PRIMARY KEY,
    admin_user_id                            BIGINT NOT NULL,
    permission_point                         VARCHAR(100) NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_sys_admin_role_perms UNIQUE (admin_user_id, permission_point)
);

CREATE TABLE sys_admin_users (
    id                                       BIGSERIAL PRIMARY KEY,
    username                                 VARCHAR(50) NOT NULL,
    password_hash                            VARCHAR(255) NOT NULL,
    email                                    VARCHAR(100),
    display_name                             VARCHAR(100),
    role                                     VARCHAR(20) DEFAULT 'admin' NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'active' NOT NULL,
    last_login_at                            TIMESTAMPTZ,
    last_login_ip                            VARCHAR(45),
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    totp_secret                              VARCHAR(64),
    totp_enabled                             BOOLEAN DEFAULT false NOT NULL,
    backup_codes                             JSONB,
    CONSTRAINT uk_sys_admin_users_email UNIQUE (email),
    CONSTRAINT uk_sys_admin_users_username UNIQUE (username)
);

CREATE TABLE sys_email_verify_codes (
    id                                       BIGSERIAL PRIMARY KEY,
    email                                    VARCHAR(100) NOT NULL,
    code                                     VARCHAR(10) NOT NULL,
    purpose                                  VARCHAR(20) NOT NULL,
    expires_at                               TIMESTAMPTZ NOT NULL,
    used_at                                  TIMESTAMPTZ,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE sys_idempotency_records (
    id                                       BIGSERIAL PRIMARY KEY,
    idempotency_key                          VARCHAR(255) NOT NULL,
    request_hash                             VARCHAR(64),
    response_body                            TEXT,
    status                                   VARCHAR(20) NOT NULL,
    expires_at                               TIMESTAMPTZ NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_sys_idempotency_key UNIQUE (idempotency_key)
);

CREATE TABLE sys_sessions (
    id                                       BIGSERIAL PRIMARY KEY,
    user_type                                VARCHAR(20) NOT NULL,
    user_id                                  BIGINT NOT NULL,
    tenant_id                                BIGINT,
    refresh_token_hash                       VARCHAR(255) NOT NULL,
    device_info                              JSONB,
    ip_address                               VARCHAR(45),
    expires_at                               TIMESTAMPTZ NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_sys_sessions_refresh_token UNIQUE (refresh_token_hash)
);

CREATE TABLE spt_attachments (
    id                                       BIGSERIAL PRIMARY KEY,
    ticket_id                                BIGINT NOT NULL,
    reply_id                                 BIGINT,
    file_name                                VARCHAR(255) NOT NULL,
    file_url                                 VARCHAR(500) NOT NULL,
    file_size                                INTEGER DEFAULT 0 NOT NULL,
    content_type                             VARCHAR(100),
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE spt_replies (
    id                                       BIGSERIAL PRIMARY KEY,
    ticket_id                                BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    user_type                                VARCHAR(20) NOT NULL,
    content                                  TEXT NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE spt_tickets (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    category                                 VARCHAR(50) NOT NULL,
    title                                    VARCHAR(255) NOT NULL,
    description                              TEXT NOT NULL,
    urgency                                  VARCHAR(20) DEFAULT 'normal' NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'pending' NOT NULL,
    assigned_admin_id                        BIGINT,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE tnt_invitations (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    code                                     VARCHAR(64) NOT NULL,
    invited_email                            VARCHAR(100),
    role                                     VARCHAR(20) DEFAULT 'member' NOT NULL,
    expires_at                               TIMESTAMPTZ,
    used_by_user_id                          BIGINT,
    used_at                                  TIMESTAMPTZ,
    created_by                               BIGINT NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    max_uses                                 INT DEFAULT 0 NOT NULL,
    use_count                                INT DEFAULT 0 NOT NULL,
    CONSTRAINT uk_tnt_invitations_code UNIQUE (code)
);

CREATE TABLE tnt_member_imports (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    filename                                 VARCHAR(255) NOT NULL,
    total_count                              INTEGER DEFAULT 0 NOT NULL,
    success_count                            INTEGER DEFAULT 0 NOT NULL,
    fail_count                               INTEGER DEFAULT 0 NOT NULL,
    skip_count                               INTEGER DEFAULT 0 NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'pending' NOT NULL,
    error_message                            TEXT,
    result_json                              JSONB,
    created_by                               BIGINT NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE tnt_member_model_scopes (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    model_id                                 BIGINT NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_tnt_member_model_scopes UNIQUE (tenant_id, user_id, model_id)
);

CREATE TABLE tnt_oauth_identities (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    provider                                 VARCHAR(20) NOT NULL,
    provider_user_id                         VARCHAR(128) NOT NULL,
    provider_username                        VARCHAR(100),
    email                                    VARCHAR(255),
    avatar_url                               VARCHAR(500),
    access_token                             TEXT,
    refresh_token                            TEXT,
    token_expires_at                         TIMESTAMPTZ,
    raw_data                                 JSONB DEFAULT '{}'::jsonb,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_tnt_oauth_identity UNIQUE (provider, provider_user_id)
);

CREATE TABLE tnt_tenants (
    id                                       BIGSERIAL PRIMARY KEY,
    name                                     VARCHAR(100) NOT NULL,
    code                                     VARCHAR(30) NOT NULL,
    logo_url                                 VARCHAR(500),
    owner_user_id                            BIGINT,
    status                                   VARCHAR(20) DEFAULT 'active' NOT NULL,
    settings                                 JSONB DEFAULT '{}'::jsonb,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    trial_ends_at                            TIMESTAMPTZ,
    grace_period_ends_at                     TIMESTAMPTZ,
    frozen_at                                TIMESTAMPTZ,
    closing_requested_at                     TIMESTAMPTZ,
    data_removal_at                          TIMESTAMPTZ,
    max_concurrency                          INTEGER DEFAULT 0 NOT NULL,
    default_channel_scope                    JSONB,
    CONSTRAINT uk_tnt_tenants_code UNIQUE (code)
);

CREATE TABLE tnt_users (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    username                                 VARCHAR(50) NOT NULL,
    email                                    VARCHAR(100) NOT NULL,
    password_hash                            VARCHAR(255) NOT NULL,
    display_name                             VARCHAR(100),
    role                                     VARCHAR(20) DEFAULT 'member' NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'active' NOT NULL,
    last_login_at                            TIMESTAMPTZ,
    last_login_ip                            VARCHAR(45),
    failed_attempts                          INTEGER DEFAULT 0 NOT NULL,
    locked_until                             TIMESTAMPTZ,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    totp_secret                              VARCHAR(64),
    totp_enabled                             BOOLEAN DEFAULT false NOT NULL,
    backup_codes                             JSONB,
    quota_type                               VARCHAR(10) DEFAULT 'none' NOT NULL,
    quota_limit                              NUMERIC(20,10) DEFAULT 0 NOT NULL,
    quota_used                               NUMERIC(20,10) DEFAULT 0 NOT NULL,
    quota_period                             VARCHAR(10),
    quota_reset_at                           TIMESTAMPTZ,
    CONSTRAINT uk_tnt_users_tenant_email UNIQUE (tenant_id, email),
    CONSTRAINT uk_tnt_users_tenant_username UNIQUE (tenant_id, username)
);

CREATE TABLE tsk_async_tasks (
    id                                       BIGSERIAL PRIMARY KEY,
    public_task_id                           VARCHAR(64) NOT NULL,
    platform                                 VARCHAR(30) NOT NULL,
    action                                   VARCHAR(40) NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'NOT_START' NOT NULL,
    progress                                 VARCHAR(20) DEFAULT '0%' NOT NULL,
    fail_reason                              TEXT,
    tenant_id                                BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    api_key_id                               BIGINT NOT NULL,
    channel_id                               BIGINT NOT NULL,
    model_name                               VARCHAR(100) NOT NULL,
    upstream_model                           VARCHAR(100),
    pre_deduct_amount                        NUMERIC(16,6) DEFAULT 0 NOT NULL,
    actual_cost                              NUMERIC(16,6) DEFAULT 0 NOT NULL,
    billing_settled                          BOOLEAN DEFAULT false NOT NULL,
    result_url                               TEXT,
    data                                     JSONB,
    private_data                             JSONB,
    submit_time                              TIMESTAMPTZ,
    start_time                               TIMESTAMPTZ,
    finish_time                              TIMESTAMPTZ,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_tsk_async_tasks_public_id UNIQUE (public_task_id)
);


CREATE TABLE tsk_task_logs (
    id                                       BIGSERIAL PRIMARY KEY,
    task_id                                  BIGINT NOT NULL,
    level                                    VARCHAR(10) NOT NULL,
    message                                  TEXT NOT NULL,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE tsk_tasks (
    id                                       BIGSERIAL PRIMARY KEY,
    name                                     VARCHAR(100) NOT NULL,
    handler                                  VARCHAR(200) NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'pending' NOT NULL,
    payload                                  JSONB,
    result                                   JSONB,
    max_retries                              INTEGER DEFAULT 3 NOT NULL,
    retry_count                              INTEGER DEFAULT 0 NOT NULL,
    started_at                               TIMESTAMPTZ,
    finished_at                              TIMESTAMPTZ,
    scheduled_at                             TIMESTAMPTZ,
    error_message                            TEXT,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE opn_webhook_configs (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    name                                     VARCHAR(100) NOT NULL,
    url                                      VARCHAR(500) NOT NULL,
    secret_key                               VARCHAR(64) NOT NULL,
    events                                   JSONB DEFAULT '[]'::jsonb NOT NULL,
    is_active                                BOOLEAN DEFAULT true NOT NULL,
    retry_policy                             JSONB DEFAULT '{"intervals": [60, 300, 900, 3600, 21600], "max_attempts": 5}'::jsonb,
    consecutive_failures                     INTEGER DEFAULT 0 NOT NULL,
    max_consecutive_failures                 INTEGER DEFAULT 10 NOT NULL,
    last_delivery_at                         TIMESTAMPTZ,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT chk_opn_webhook_configs_url CHECK (((url)::text ~~ 'https://%'::text))
);

CREATE TABLE opn_webhook_delivery_logs (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    webhook_config_id                        BIGINT NOT NULL,
    event_id                                 BIGINT NOT NULL,
    attempt                                  INTEGER DEFAULT 1 NOT NULL,
    request_url                              VARCHAR(500) NOT NULL,
    request_headers                          JSONB,
    response_status                          INTEGER,
    response_body                            TEXT,
    response_time_ms                         INTEGER,
    error_message                            VARCHAR(500),
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE opn_webhook_events (
    id                                       BIGSERIAL PRIMARY KEY,
    tenant_id                                BIGINT NOT NULL,
    webhook_config_id                        BIGINT NOT NULL,
    event_id                                 VARCHAR(64) NOT NULL,
    event_type                               VARCHAR(100) NOT NULL,
    payload                                  JSONB DEFAULT '{}'::jsonb NOT NULL,
    status                                   VARCHAR(20) DEFAULT 'pending' NOT NULL,
    attempts                                 INTEGER DEFAULT 0 NOT NULL,
    next_retry_at                            TIMESTAMPTZ,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE bil_usage_logs (
    id                                       BIGSERIAL,
    tenant_id                                BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    api_key_id                               BIGINT,
    channel_id                               BIGINT,
    model_name                               VARCHAR(100) NOT NULL,
    request_id                               VARCHAR(64) NOT NULL,
    relay_mode                               VARCHAR(30),
    input_tokens                             INTEGER,
    output_tokens                            INTEGER,
    total_cost                               NUMERIC(20,10),
    currency                                 VARCHAR(3) DEFAULT 'USD' NOT NULL,
    latency_ms                               INTEGER,
    status                                   VARCHAR(20) NOT NULL,
    error_message                            TEXT,
    client_ip                                VARCHAR(45),
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    cache_creation_tokens                    INTEGER DEFAULT 0 NOT NULL,
    cache_read_tokens                        INTEGER DEFAULT 0 NOT NULL,
    input_cost                               NUMERIC(20,10) DEFAULT 0 NOT NULL,
    output_cost                              NUMERIC(20,10) DEFAULT 0 NOT NULL,
    cache_creation_cost                      NUMERIC(20,10) DEFAULT 0 NOT NULL,
    cache_read_cost                          NUMERIC(20,10) DEFAULT 0 NOT NULL,
    actual_cost                              NUMERIC(20,10) DEFAULT 0 NOT NULL,
    requested_model                          VARCHAR(100),
    upstream_model                           VARCHAR(100),
    request_type                             SMALLINT DEFAULT 1 NOT NULL,
    user_agent                               VARCHAR(512),
    first_token_ms                           INTEGER,
    service_tier                             VARCHAR(16),
    reasoning_effort                         VARCHAR(20),
    channel_name                             VARCHAR(100),
    channel_type                             INTEGER,
    billing_mode                             VARCHAR(20),
    billing_source                           VARCHAR(20),
    rate_multiplier                          NUMERIC(10,4) DEFAULT 1.0000 NOT NULL,
    pre_deduct_amount                        NUMERIC(20,10) DEFAULT 0 NOT NULL,
    refund_amount                            NUMERIC(20,10) DEFAULT 0 NOT NULL,
    supplement_amount                        NUMERIC(20,10) DEFAULT 0 NOT NULL,
    image_count                              INTEGER DEFAULT 0 NOT NULL,
    image_size                               VARCHAR(10),
    stream_end_reason                        VARCHAR(20),
    retry_index                              INTEGER DEFAULT 0 NOT NULL,
    billing_summary                          TEXT,
    cache_creation_5m_tokens                 INTEGER DEFAULT 0 NOT NULL,
    cache_creation_1h_tokens                 INTEGER DEFAULT 0 NOT NULL,
    audio_input_tokens                       INTEGER DEFAULT 0 NOT NULL,
    audio_output_tokens                      INTEGER DEFAULT 0 NOT NULL,
    image_output_tokens                      INTEGER DEFAULT 0 NOT NULL,
    reasoning_tokens                         INTEGER DEFAULT 0 NOT NULL,
    account_cost                             NUMERIC(20,10) DEFAULT 0 NOT NULL,
    inbound_endpoint                         VARCHAR(128),
    upstream_endpoint                        VARCHAR(128),
    billing_snapshot                         JSONB,
    project_id                               BIGINT,
    PRIMARY KEY (id, created_at)
)
PARTITION BY RANGE (created_at);

CREATE TABLE ops_system_metrics (
    id                                       BIGSERIAL,
    metric_type                              VARCHAR(30) NOT NULL,
    metric_data                              JSONB DEFAULT '{}'::jsonb NOT NULL,
    collected_at                             TIMESTAMPTZ DEFAULT now() NOT NULL,
    PRIMARY KEY (id, collected_at)
)
PARTITION BY RANGE (collected_at);


-- ============================================================
-- Additional Tables
-- ============================================================

CREATE TABLE ord_redemption_usages (
    id                                       BIGSERIAL PRIMARY KEY,
    redemption_id                            BIGINT NOT NULL,
    tenant_id                                BIGINT NOT NULL,
    user_id                                  BIGINT NOT NULL,
    type                                     VARCHAR(20) NOT NULL,
    value                                    NUMERIC(20,10) DEFAULT 0 NOT NULL,
    transaction_id                           BIGINT,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE sys_error_logs (
    id                                       BIGSERIAL PRIMARY KEY,
    request_id                               VARCHAR(64),
    error_code                               INTEGER NOT NULL,
    error_message                            TEXT NOT NULL,
    stack_trace                              TEXT,
    http_method                              VARCHAR(10),
    request_path                             VARCHAR(500),
    request_body                             TEXT,
    source                                   VARCHAR(50) NOT NULL DEFAULT 'api',
    resolved                                 BOOLEAN NOT NULL DEFAULT FALSE,
    resolved_by                              BIGINT,
    resolved_at                              TIMESTAMPTZ,
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE TABLE sys_cron_job_executions (
    id                                       BIGSERIAL PRIMARY KEY,
    job_name                                 VARCHAR(100) NOT NULL,
    status                                   VARCHAR(20) NOT NULL,
    started_at                               TIMESTAMPTZ NOT NULL,
    finished_at                              TIMESTAMPTZ NOT NULL,
    duration_ms                              INTEGER NOT NULL,
    error_message                            TEXT,
    triggered_by                             VARCHAR(20) NOT NULL DEFAULT 'auto',
    created_at                               TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at                               TIMESTAMPTZ DEFAULT now() NOT NULL
);

-- ============================================================
-- Indexes
-- ============================================================

CREATE INDEX idx_api_keys_prefix ON api_keys USING btree (key_prefix);
CREATE INDEX idx_api_keys_project ON api_keys USING btree (project_id) WHERE (project_id IS NOT NULL);
CREATE INDEX idx_api_keys_tenant ON api_keys USING btree (tenant_id);
CREATE INDEX idx_api_keys_tenant_type ON api_keys USING btree (tenant_id, key_type);
CREATE INDEX idx_aud_login_history_tenant ON aud_login_history USING btree (tenant_id, created_at DESC) WHERE (tenant_id IS NOT NULL);
CREATE INDEX idx_aud_login_history_user ON aud_login_history USING btree (user_type, user_id, created_at DESC);
CREATE INDEX idx_aud_operation_logs_resource ON aud_operation_logs USING btree (resource_type, resource_id);
CREATE INDEX idx_aud_operation_logs_user ON aud_operation_logs USING btree (user_type, user_id, created_at);
CREATE INDEX idx_aud_request_logs_created_brin ON aud_request_logs USING brin (created_at);
CREATE INDEX idx_aud_request_logs_tenant ON aud_request_logs USING btree (tenant_id, created_at);
CREATE INDEX idx_aud_sensitive_access_logs_created ON aud_sensitive_access_logs USING btree (created_at);
CREATE INDEX idx_aud_sensitive_access_logs_resource ON aud_sensitive_access_logs USING btree (resource_type, resource_id);
CREATE INDEX idx_aud_sensitive_access_logs_user ON aud_sensitive_access_logs USING btree (user_id, user_type);
CREATE UNIQUE INDEX uk_bil_daily_revenue_date ON bil_daily_revenue_summary USING btree (date);
CREATE UNIQUE INDEX uk_bil_daily_usage_tenant_date ON bil_daily_usage_summary USING btree (tenant_id, date);
CREATE UNIQUE INDEX uk_bil_monthly_revenue_month ON bil_monthly_revenue_summary USING btree (month);
CREATE UNIQUE INDEX uk_bil_monthly_usage_tenant_month ON bil_monthly_usage_summary USING btree (tenant_id, month);
CREATE INDEX idx_bil_records_request ON bil_records USING btree (request_id);
CREATE INDEX idx_bil_records_tenant ON bil_records USING btree (tenant_id, created_at);
CREATE INDEX idx_bil_records_tenant_model ON bil_records USING btree (tenant_id, model_name, created_at);
CREATE INDEX idx_bil_records_user ON bil_records USING btree (user_id, created_at);

CREATE INDEX idx_bil_transactions_tenant ON bil_transactions USING btree (tenant_id, created_at);
CREATE INDEX idx_bil_transactions_wallet ON bil_transactions USING btree (wallet_id, created_at);
CREATE INDEX idx_bil_usage_logs_channel_created ON ONLY bil_usage_logs USING btree (channel_id, created_at);
CREATE INDEX idx_bil_usage_logs_created_brin ON ONLY bil_usage_logs USING brin (created_at);
CREATE INDEX idx_bil_usage_logs_model_created ON ONLY bil_usage_logs USING btree (model_name, created_at);
CREATE INDEX idx_bil_usage_logs_request ON ONLY bil_usage_logs USING btree (request_id);
CREATE INDEX idx_bil_usage_logs_status_created ON ONLY bil_usage_logs USING btree (status, created_at);
CREATE INDEX idx_bil_usage_logs_tenant ON ONLY bil_usage_logs USING btree (tenant_id, created_at);
CREATE INDEX idx_chn_abilities_model ON chn_abilities USING btree (model_name);
CREATE INDEX idx_chn_affinities_expires ON chn_channel_affinities USING btree (expires_at);
CREATE INDEX idx_chn_channel_keys_channel ON chn_channel_keys USING btree (channel_id);
CREATE INDEX idx_chn_health_snapshots_brin ON chn_health_snapshots USING brin (snapshot_at);
CREATE INDEX idx_chn_health_snapshots_channel_time ON chn_health_snapshots USING btree (channel_id, snapshot_at);
CREATE INDEX idx_clg_changelogs_status ON clg_changelogs USING btree (status, created_at DESC);


CREATE INDEX idx_ord_promo_code_usages_promo ON ord_promo_code_usages USING btree (promo_code_id);
CREATE INDEX idx_ord_promo_code_usages_tenant ON ord_promo_code_usages USING btree (tenant_id);
CREATE INDEX idx_ord_promo_codes_status ON ord_promo_codes USING btree (status);
CREATE INDEX idx_ord_redemptions_batch ON ord_redemptions USING btree (batch_no);
CREATE INDEX idx_ord_redemptions_status ON ord_redemptions USING btree (status);
CREATE INDEX idx_spt_feedbacks_category ON spt_feedbacks USING btree (category);
CREATE INDEX idx_spt_feedbacks_status ON spt_feedbacks USING btree (status);
CREATE INDEX idx_spt_feedbacks_tenant ON spt_feedbacks USING btree (tenant_id, created_at DESC);
CREATE INDEX idx_fil_files_tenant ON fil_files USING btree (tenant_id, created_at);
CREATE INDEX idx_spt_articles_category ON spt_articles USING btree (category_id);
CREATE INDEX idx_spt_articles_search ON spt_articles USING gin (to_tsvector('simple'::regconfig, (((COALESCE(title, ''::character varying))::text || ' '::text) || COALESCE(content, ''::text))));
CREATE INDEX idx_spt_articles_status ON spt_articles USING btree (status, published_at DESC);
CREATE INDEX idx_spt_categories_parent_sort ON spt_categories USING btree (parent_id, sort_order);


CREATE INDEX idx_mdl_pricing_model_billing ON mdl_pricing USING btree (model_id, billing_mode);
CREATE INDEX idx_mdl_pricing_model_id ON mdl_pricing USING btree (model_id);
CREATE INDEX idx_mdl_tenant_models_tenant ON mdl_tenant_models USING btree (tenant_id);
CREATE INDEX idx_ntf_announcements_effective ON ntf_announcements USING btree (effective_at, expires_at);
CREATE INDEX idx_ntf_announcements_status ON ntf_announcements USING btree (status);
CREATE INDEX idx_ntf_messages_created ON ntf_messages USING btree (created_at);
CREATE INDEX idx_ntf_messages_read ON ntf_messages USING btree (tenant_id, user_id, is_read);
CREATE INDEX idx_ntf_messages_tenant_user ON ntf_messages USING btree (tenant_id, user_id);
CREATE INDEX idx_ntf_read_status_message ON ntf_read_status USING btree (message_id);
CREATE INDEX idx_ntf_read_status_user ON ntf_read_status USING btree (user_id);
CREATE INDEX idx_ntf_send_log_status ON ntf_send_log USING btree (status, created_at);
CREATE INDEX idx_opn_apps_tenant ON opn_apps USING btree (tenant_id);

CREATE INDEX idx_ops_alert_events_created_brin ON ops_alert_events USING brin (created_at);
CREATE INDEX idx_ops_alert_events_level ON ops_alert_events USING btree (level);
CREATE INDEX idx_ops_alert_events_rule ON ops_alert_events USING btree (rule_id);
CREATE INDEX idx_ops_alert_events_status ON ops_alert_events USING btree (status);
CREATE INDEX idx_ops_alert_rules_enabled ON ops_alert_rules USING btree (is_enabled);
CREATE INDEX idx_ops_alert_rules_metric ON ops_alert_rules USING btree (metric_type);
CREATE INDEX idx_ops_system_metrics_type_brin ON ONLY ops_system_metrics USING brin (metric_type, collected_at);
CREATE INDEX idx_ord_orders_no ON ord_orders USING btree (order_no);
CREATE INDEX idx_ord_orders_status ON ord_orders USING btree (status, created_at);
CREATE INDEX idx_ord_orders_tenant ON ord_orders USING btree (tenant_id, created_at);
CREATE INDEX idx_ord_payment_channels_channel ON ord_payment_channels USING btree (channel);
CREATE INDEX idx_ord_refunds_order ON ord_refunds USING btree (order_id);
CREATE INDEX idx_ord_refunds_tenant ON ord_refunds USING btree (tenant_id, created_at);
CREATE INDEX idx_pln_feature_flags_key ON pln_feature_flags USING btree (feature_key);
CREATE INDEX idx_pln_feature_flags_tenant ON pln_feature_flags USING btree (tenant_id, feature_key);
CREATE INDEX idx_pln_tenant_plans_status ON pln_tenant_plans USING btree (status, end_at);
CREATE INDEX idx_pln_tenant_plans_tenant ON pln_tenant_plans USING btree (tenant_id);
CREATE INDEX idx_tnt_projects_tenant ON tnt_projects USING btree (tenant_id);


CREATE INDEX idx_sys_email_verify_codes_lookup ON sys_email_verify_codes USING btree (email, purpose, expires_at);
CREATE INDEX idx_sys_sessions_user ON sys_sessions USING btree (user_type, user_id, expires_at);
CREATE INDEX idx_spt_attachments_ticket ON spt_attachments USING btree (ticket_id);
CREATE INDEX idx_spt_replies_created ON spt_replies USING btree (created_at);
CREATE INDEX idx_spt_replies_ticket ON spt_replies USING btree (ticket_id);
CREATE INDEX idx_spt_tickets_assigned ON spt_tickets USING btree (assigned_admin_id);
CREATE INDEX idx_spt_tickets_created ON spt_tickets USING btree (created_at);
CREATE INDEX idx_spt_tickets_status ON spt_tickets USING btree (status);
CREATE INDEX idx_spt_tickets_tenant ON spt_tickets USING btree (tenant_id, user_id);
CREATE INDEX idx_tnt_invitations_tenant ON tnt_invitations USING btree (tenant_id);
CREATE INDEX idx_tnt_member_imports_status ON tnt_member_imports USING btree (status);
CREATE INDEX idx_tnt_member_imports_tenant ON tnt_member_imports USING btree (tenant_id);
CREATE INDEX idx_tnt_member_model_scopes_user ON tnt_member_model_scopes USING btree (tenant_id, user_id);
CREATE INDEX idx_tnt_oauth_identities_provider ON tnt_oauth_identities USING btree (provider, provider_user_id);
CREATE INDEX idx_tnt_oauth_identities_user ON tnt_oauth_identities USING btree (tenant_id, user_id);
CREATE INDEX idx_tnt_users_tenant ON tnt_users USING btree (tenant_id);
CREATE INDEX idx_tsk_async_tasks_active ON tsk_async_tasks USING btree (status, platform) WHERE ((status)::text <> ALL ((ARRAY['SUCCESS'::character varying, 'FAILURE'::character varying])::text[]));
CREATE INDEX idx_tsk_async_tasks_status ON tsk_async_tasks USING btree (status);
CREATE INDEX idx_tsk_async_tasks_submit_time ON tsk_async_tasks USING btree (submit_time);
CREATE INDEX idx_tsk_async_tasks_user ON tsk_async_tasks USING btree (tenant_id, user_id);

CREATE INDEX idx_tsk_task_logs_task ON tsk_task_logs USING btree (task_id, created_at);
CREATE INDEX idx_tsk_tasks_scheduled ON tsk_tasks USING btree (scheduled_at) WHERE ((status)::text = 'pending'::text);
CREATE INDEX idx_tsk_tasks_status ON tsk_tasks USING btree (status, created_at);
CREATE INDEX idx_opn_webhook_configs_tenant ON opn_webhook_configs USING btree (tenant_id);
CREATE INDEX idx_opn_webhook_delivery_logs_event ON opn_webhook_delivery_logs USING btree (event_id);
CREATE INDEX idx_opn_webhook_delivery_logs_tenant ON opn_webhook_delivery_logs USING btree (tenant_id, created_at DESC);
CREATE INDEX idx_opn_webhook_events_status ON opn_webhook_events USING btree (status, next_retry_at) WHERE ((status)::text = ANY ((ARRAY['pending'::character varying, 'failed'::character varying])::text[]));
CREATE INDEX idx_opn_webhook_events_tenant ON opn_webhook_events USING btree (tenant_id, created_at DESC);


CREATE INDEX idx_bil_usage_logs_project ON ONLY bil_usage_logs USING btree (project_id) WHERE project_id IS NOT NULL;
CREATE INDEX idx_aud_request_logs_project ON aud_request_logs USING btree (project_id) WHERE project_id IS NOT NULL;
CREATE UNIQUE INDEX uk_chn_channel_keys_channel_id ON chn_channel_keys USING btree (channel_id);
CREATE INDEX idx_chn_channel_keys_oauth_expiring ON chn_channel_keys (token_expires_at) WHERE key_type = 'oauth' AND status = 'active';


CREATE INDEX idx_ord_redemption_usages_redemption ON ord_redemption_usages USING btree (redemption_id);
CREATE INDEX idx_ord_redemption_usages_tenant ON ord_redemption_usages USING btree (tenant_id, created_at DESC);
CREATE INDEX idx_sys_error_logs_resolved ON sys_error_logs (resolved) WHERE resolved = FALSE;
CREATE INDEX idx_sys_error_logs_source ON sys_error_logs (source);
CREATE INDEX idx_sys_error_logs_error_code ON sys_error_logs (error_code);
CREATE INDEX idx_sys_error_logs_created_at ON sys_error_logs USING BRIN (created_at);
CREATE INDEX idx_sys_cron_job_exec_name_time ON sys_cron_job_executions (job_name, created_at DESC);
CREATE INDEX idx_sys_cron_job_exec_created_brin ON sys_cron_job_executions USING BRIN (created_at);

-- ============================================================
-- Triggers
-- ============================================================

CREATE TRIGGER tr_cleanup_channel_references BEFORE DELETE ON chn_channels FOR EACH ROW EXECUTE FUNCTION cleanup_channel_references();

-- ============================================================
-- Comments
-- ============================================================

COMMENT ON TABLE api_key_model_scopes IS 'API Key 可用模型范围';
COMMENT ON COLUMN api_key_model_scopes.id IS '主键ID';
COMMENT ON COLUMN api_key_model_scopes.api_key_id IS '关联 API Key ID';
COMMENT ON COLUMN api_key_model_scopes.model_name IS '允许调用的模型名';
COMMENT ON COLUMN api_key_model_scopes.created_at IS '创建时间';
COMMENT ON COLUMN api_key_model_scopes.updated_at IS '更新时间';
COMMENT ON TABLE api_keys IS 'API Key（租户调用 AI 接口的凭证）';
COMMENT ON COLUMN api_keys.id IS '主键ID';
COMMENT ON COLUMN api_keys.tenant_id IS '所属租户ID';
COMMENT ON COLUMN api_keys.user_id IS '创建者用户ID';
COMMENT ON COLUMN api_keys.name IS 'Key 名称（如 "生产环境"、"测试用"）';
COMMENT ON COLUMN api_keys.encrypted_key IS '加密存储的完整 API Key（AES-256）';
COMMENT ON COLUMN api_keys.key_prefix IS 'Key 前缀（用于快速查找，明文存储，如 sk-a1b2c3d4）';
COMMENT ON COLUMN api_keys.scope IS '权限范围：full（全部）/ chat_only（仅对话）/ embeddings_only（仅嵌入）/ images_only（仅图像）/ read_only（只读）/ custom（自定义）';
COMMENT ON COLUMN api_keys.status IS '状态：active（正常）/ disabled（禁用）/ expired（已过期）';
COMMENT ON COLUMN api_keys.expires_at IS '过期时间（NULL 表示永不过期）';
COMMENT ON COLUMN api_keys.rate_limit_qps IS 'QPS 限流阈值（NULL 表示使用默认值）';
COMMENT ON COLUMN api_keys.rate_limit_concurrency IS '并发限制阈值（NULL 表示使用默认值）';
COMMENT ON COLUMN api_keys.ip_whitelist IS 'IP 白名单数组（NULL 或空数组表示不限制）';
COMMENT ON COLUMN api_keys.total_quota IS '额度上限（NULL 表示不限制）';
COMMENT ON COLUMN api_keys.used_quota IS '已使用额度';
COMMENT ON COLUMN api_keys.project_id IS '关联项目ID（NULL 表示不属于任何项目）';
COMMENT ON COLUMN api_keys.created_at IS '创建时间';
COMMENT ON COLUMN api_keys.updated_at IS '更新时间';
COMMENT ON COLUMN api_keys.key_type IS '密钥类型：personal（个人密钥）/ project（项目密钥）';
COMMENT ON TABLE aud_login_history IS '登录历史记录（管理后台 + 租户控制台共用）';
COMMENT ON COLUMN aud_login_history.id IS '主键ID';
COMMENT ON COLUMN aud_login_history.user_type IS '用户类型：admin（管理后台）/ tenant（租户控制台）';
COMMENT ON COLUMN aud_login_history.user_id IS '用户ID';
COMMENT ON COLUMN aud_login_history.tenant_id IS '租户ID（仅 tenant 类型用户有值）';
COMMENT ON COLUMN aud_login_history.login_method IS '登录方式：password（密码）/ totp（双因素）/ sso（单点登录）/ backup_code（恢复码）';
COMMENT ON COLUMN aud_login_history.ip_address IS '登录IP地址';
COMMENT ON COLUMN aud_login_history.user_agent IS '浏览器 User-Agent';
COMMENT ON COLUMN aud_login_history.device_fingerprint IS '设备指纹（用于检测新设备登录）';
COMMENT ON COLUMN aud_login_history.location IS 'IP 地理位置';
COMMENT ON COLUMN aud_login_history.is_new_device IS '是否为新设备';
COMMENT ON COLUMN aud_login_history.success IS '登录是否成功';
COMMENT ON COLUMN aud_login_history.fail_reason IS '登录失败原因';
COMMENT ON COLUMN aud_login_history.created_at IS '登录时间';
COMMENT ON TABLE aud_operation_logs IS '操作日志（记录管理后台和租户控制台的所有写操作）';
COMMENT ON COLUMN aud_operation_logs.id IS '主键ID';
COMMENT ON COLUMN aud_operation_logs.tenant_id IS '租户ID（管理后台操作时为 NULL）';
COMMENT ON COLUMN aud_operation_logs.user_id IS '操作者用户ID';
COMMENT ON COLUMN aud_operation_logs.user_type IS '操作者类型：admin（管理后台）/ tenant（租户控制台）';
COMMENT ON COLUMN aud_operation_logs.action IS '操作动作（如 create_tenant、update_channel、delete_model）';
COMMENT ON COLUMN aud_operation_logs.resource_type IS '操作资源类型：tenant / channel / model / user / api_key 等';
COMMENT ON COLUMN aud_operation_logs.resource_id IS '操作资源ID';
COMMENT ON COLUMN aud_operation_logs.detail IS '操作详情（JSONB：变更前后的字段差异等）';
COMMENT ON COLUMN aud_operation_logs.ip_address IS '操作者 IP 地址';
COMMENT ON COLUMN aud_operation_logs.created_at IS '创建时间';
COMMENT ON COLUMN aud_operation_logs.updated_at IS '更新时间';
COMMENT ON COLUMN aud_operation_logs.changes_json IS '变更前后数据对比（JSON diff）';
COMMENT ON TABLE aud_request_logs IS '请求审计日志（记录所有 API 代理请求）';
COMMENT ON COLUMN aud_request_logs.id IS '主键ID';
COMMENT ON COLUMN aud_request_logs.tenant_id IS '租户ID';
COMMENT ON COLUMN aud_request_logs.user_id IS '用户ID';
COMMENT ON COLUMN aud_request_logs.api_key_id IS '使用的 API Key ID';
COMMENT ON COLUMN aud_request_logs.request_id IS '请求唯一ID（关联全链路追踪）';
COMMENT ON COLUMN aud_request_logs.method IS 'HTTP 方法（GET/POST/PUT/DELETE）';
COMMENT ON COLUMN aud_request_logs.path IS '请求路径';
COMMENT ON COLUMN aud_request_logs.query_params IS '查询参数（URL Query String）';
COMMENT ON COLUMN aud_request_logs.status_code IS 'HTTP 响应状态码';
COMMENT ON COLUMN aud_request_logs.client_ip IS '客户端 IP';
COMMENT ON COLUMN aud_request_logs.user_agent IS '客户端 User-Agent';
COMMENT ON COLUMN aud_request_logs.request_body IS '请求体（敏感字段脱敏后存储）';
COMMENT ON COLUMN aud_request_logs.response_body IS '响应体（截断后存储）';
COMMENT ON COLUMN aud_request_logs.latency_ms IS '请求延迟（毫秒）';
COMMENT ON COLUMN aud_request_logs.audit_level IS '审计级别：full（完整记录）/ masked（脱敏记录）/ question_only（仅记录提问）/ none（不记录）';
COMMENT ON COLUMN aud_request_logs.created_at IS '创建时间';
COMMENT ON COLUMN aud_request_logs.updated_at IS '更新时间';
COMMENT ON COLUMN aud_request_logs.tenant_request_body IS '租户级请求体（按租户审计级别处理）';
COMMENT ON COLUMN aud_request_logs.tenant_response_body IS '租户级响应体（按租户审计级别处理）';
COMMENT ON COLUMN aud_request_logs.tenant_audit_level IS '租户审计级别：full/full_text/masked/question_only/none';
COMMENT ON TABLE aud_sensitive_access_logs IS '敏感数据访问日志';
COMMENT ON COLUMN aud_sensitive_access_logs.id IS '主键ID';
COMMENT ON COLUMN aud_sensitive_access_logs.user_id IS '访问者用户ID';
COMMENT ON COLUMN aud_sensitive_access_logs.user_type IS '访问者类型：admin（管理后台）/ tenant（租户控制台）';
COMMENT ON COLUMN aud_sensitive_access_logs.resource_type IS '资源类型（如 api_key、channel、wallet 等）';
COMMENT ON COLUMN aud_sensitive_access_logs.resource_id IS '资源ID';
COMMENT ON COLUMN aud_sensitive_access_logs.action IS '访问动作（如 view、export、download）';
COMMENT ON COLUMN aud_sensitive_access_logs.reason IS '访问原因（查看敏感数据时需填写）';
COMMENT ON COLUMN aud_sensitive_access_logs.ip_address IS '访问者 IP 地址';
COMMENT ON COLUMN aud_sensitive_access_logs.user_agent IS '访问者 User-Agent';
COMMENT ON COLUMN aud_sensitive_access_logs.created_at IS '创建时间';
COMMENT ON TABLE bil_daily_revenue_summary IS '日收入汇总';
COMMENT ON TABLE bil_daily_usage_summary IS '日用量汇总';
COMMENT ON TABLE bil_monthly_revenue_summary IS '月收入汇总';
COMMENT ON TABLE bil_monthly_usage_summary IS '月用量汇总';
COMMENT ON TABLE bil_records IS '计费记录（预扣→结算→退差额全流程）';
COMMENT ON COLUMN bil_records.id IS '主键ID';
COMMENT ON COLUMN bil_records.tenant_id IS '租户ID';
COMMENT ON COLUMN bil_records.user_id IS '用户ID';
COMMENT ON COLUMN bil_records.api_key_id IS '使用的 API Key ID';
COMMENT ON COLUMN bil_records.channel_id IS '使用的渠道ID';
COMMENT ON COLUMN bil_records.model_name IS '调用的模型名';
COMMENT ON COLUMN bil_records.request_id IS '请求唯一ID（关联全链路追踪）';
COMMENT ON COLUMN bil_records.relay_mode IS '代理模式：chat_completions / embeddings / images_generations 等';
COMMENT ON COLUMN bil_records.input_tokens IS '输入 token 数';
COMMENT ON COLUMN bil_records.output_tokens IS '输出 token 数';
COMMENT ON COLUMN bil_records.input_price IS '计费时输入单价（快照，防止价格变更影响历史记录）';
COMMENT ON COLUMN bil_records.output_price IS '计费时输出单价（快照）';
COMMENT ON COLUMN bil_records.total_cost IS '最终费用 = 基础价格 × 模型倍率 × 租户倍率';
COMMENT ON COLUMN bil_records.currency IS '货币（USD）';
COMMENT ON COLUMN bil_records.status IS '状态：pre_deducted（已预扣）/ settled（已结算）/ refunded（已退款）';
COMMENT ON COLUMN bil_records.settled_at IS '结算时间';
COMMENT ON COLUMN bil_records.created_at IS '创建时间';
COMMENT ON COLUMN bil_records.updated_at IS '更新时间';
COMMENT ON COLUMN bil_records.billing_mode IS '实际计费模式';
COMMENT ON COLUMN bil_records.effective_input_price IS '实际生效的输入单价（快照）';
COMMENT ON COLUMN bil_records.effective_output_price IS '实际生效的输出单价（快照）';
COMMENT ON COLUMN bil_records.discount_ratio IS '实际折扣比例（快照）';
COMMENT ON COLUMN bil_records.billing_input_multiplier IS '梯度定价输入乘数（快照）';
COMMENT ON COLUMN bil_records.billing_output_multiplier IS '梯度定价输出乘数（快照）';
COMMENT ON COLUMN bil_records.cache_creation_tokens IS '缓存创建 token 数';
COMMENT ON COLUMN bil_records.cache_read_tokens IS '缓存读取 token 数';
COMMENT ON COLUMN bil_records.cache_creation_cost IS '缓存创建费用';
COMMENT ON COLUMN bil_records.cache_read_cost IS '缓存读取费用';
COMMENT ON COLUMN bil_records.model_multiplier IS '模型倍率（快照）';
COMMENT ON COLUMN bil_records.tenant_multiplier IS '租户倍率（快照）';
COMMENT ON COLUMN bil_records.base_input_price IS '基础模型输入单价（快照，应用倍率前）';
COMMENT ON COLUMN bil_records.base_output_price IS '基础模型输出单价（快照，应用倍率前）';
COMMENT ON COLUMN bil_records.billing_snapshot IS '完整计费计算过程快照（JSONB）';
COMMENT ON TABLE bil_transactions IS '交易流水（钱包所有资金变动记录）';
COMMENT ON COLUMN bil_transactions.id IS '主键ID';
COMMENT ON COLUMN bil_transactions.tenant_id IS '租户ID';
COMMENT ON COLUMN bil_transactions.wallet_id IS '关联钱包ID';
COMMENT ON COLUMN bil_transactions.type IS '类型：recharge（充值）/ pre_deduct（预扣）/ settle（结算）/ refund（退款）/ adjust（调整）/ freeze（冻结）/ unfreeze（解冻）';
COMMENT ON COLUMN bil_transactions.amount IS '变动金额（正数=收入，负数=支出）';
COMMENT ON COLUMN bil_transactions.balance_after IS '变动后总余额';
COMMENT ON COLUMN bil_transactions.frozen_after IS '变动后冻结余额';
COMMENT ON COLUMN bil_transactions.related_id IS '关联业务ID（如计费记录ID、订单ID等）';
COMMENT ON COLUMN bil_transactions.related_type IS '关联业务类型：billing_record / order / refund / adjustment / redemption';
COMMENT ON COLUMN bil_transactions.description IS '交易描述';
COMMENT ON COLUMN bil_transactions.created_at IS '创建时间';
COMMENT ON COLUMN bil_transactions.updated_at IS '更新时间';
COMMENT ON TABLE bil_wallets IS '租户钱包';
COMMENT ON COLUMN bil_wallets.id IS '主键ID';
COMMENT ON COLUMN bil_wallets.tenant_id IS '租户ID（每个租户一个钱包）';
COMMENT ON COLUMN bil_wallets.balance IS '总余额';
COMMENT ON COLUMN bil_wallets.frozen_balance IS '冻结余额（支付中/退款中，可用余额 = balance - frozen_balance）';
COMMENT ON COLUMN bil_wallets.warning_threshold IS '余额预警线（低于此值触发通知）';
COMMENT ON COLUMN bil_wallets.currency IS '货币（USD）';
COMMENT ON COLUMN bil_wallets.created_at IS '创建时间';
COMMENT ON COLUMN bil_wallets.updated_at IS '更新时间';
COMMENT ON TABLE chn_abilities IS '渠道能力索引（渠道支持的模型映射）';
COMMENT ON COLUMN chn_abilities.id IS '主键ID';
COMMENT ON COLUMN chn_abilities.channel_id IS '关联渠道ID';
COMMENT ON COLUMN chn_abilities.model_name IS '平台标准模型名（用户请求使用的模型名）';
COMMENT ON COLUMN chn_abilities.upstream_model IS '上游实际模型名（与平台标准名不同时需要映射，如平台名 gpt-4 → 上游名 gpt-4-0314）';
COMMENT ON COLUMN chn_abilities.enabled IS '是否启用该模型能力';
COMMENT ON COLUMN chn_abilities.created_at IS '创建时间';
COMMENT ON COLUMN chn_abilities.updated_at IS '更新时间';
COMMENT ON TABLE chn_channel_affinities IS '渠道亲和性缓存（用户+模型→渠道映射，TTL 1800s）';
COMMENT ON COLUMN chn_channel_affinities.id IS '主键ID';
COMMENT ON COLUMN chn_channel_affinities.tenant_id IS '租户ID';
COMMENT ON COLUMN chn_channel_affinities.user_id IS '用户ID';
COMMENT ON COLUMN chn_channel_affinities.model_name IS '模型名';
COMMENT ON COLUMN chn_channel_affinities.channel_id IS '绑定的渠道ID';
COMMENT ON COLUMN chn_channel_affinities.hit_count IS '命中次数（同一渠道连续成功次数）';
COMMENT ON COLUMN chn_channel_affinities.expires_at IS '过期时间（默认 1800 秒后过期）';
COMMENT ON COLUMN chn_channel_affinities.created_at IS '创建时间';
COMMENT ON COLUMN chn_channel_affinities.updated_at IS '更新时间';
COMMENT ON TABLE chn_channel_keys IS '渠道 API Key（一个渠道可配多个 Key 轮询）';
COMMENT ON COLUMN chn_channel_keys.id IS '主键ID';
COMMENT ON COLUMN chn_channel_keys.channel_id IS '关联渠道ID';
COMMENT ON COLUMN chn_channel_keys.name IS 'Key 别名（用于管理标识，如"主力Key"、"备用Key"）';
COMMENT ON COLUMN chn_channel_keys.encrypted_key IS '加密存储的 API Key 原值（AES-256）';
COMMENT ON COLUMN chn_channel_keys.status IS '状态：active（可用）/ disabled（禁用）/ exhausted（额度耗尽）';
COMMENT ON COLUMN chn_channel_keys.last_used_at IS '最后使用时间';
COMMENT ON COLUMN chn_channel_keys.last_error IS '最后一次错误信息';
COMMENT ON COLUMN chn_channel_keys.created_at IS '创建时间';
COMMENT ON COLUMN chn_channel_keys.updated_at IS '更新时间';
COMMENT ON TABLE chn_channels IS '渠道配置（AI 供应商接入点）';
COMMENT ON COLUMN chn_channels.id IS '主键ID';
COMMENT ON COLUMN chn_channels.name IS '渠道显示名称（如 "OpenAI 主力"、"Claude 备用"）';
COMMENT ON COLUMN chn_channels.type IS '供应商类型：1=OpenAI, 2=Anthropic Claude, 3=Google Gemini, 4=阿里云百炼, 5=百度文心, 6=腾讯混元, 7=智谱AI, 8=DeepSeek, 9=Moonshot, 10=火山引擎, 11=AWS Bedrock, 12=Azure OpenAI, 13=Google Vertex AI, 14=Cohere, 15=Mistral, 16=xAI';
COMMENT ON COLUMN chn_channels.base_url IS 'API 基础地址';
COMMENT ON COLUMN chn_channels.status IS '状态：active（启用）/ disabled（禁用）/ testing（测试中）';
COMMENT ON COLUMN chn_channels.priority IS '优先级（数字越大越优先，调度时优先选择高优先级渠道）';
COMMENT ON COLUMN chn_channels.weight IS '权重（同优先级下按权重随机选择，范围 1-100）';
COMMENT ON COLUMN chn_channels.max_concurrency IS '最大并发请求数';
COMMENT ON COLUMN chn_channels.settings IS '渠道配置（JSONB）：超时时间、重试次数等';
COMMENT ON COLUMN chn_channels.test_model IS '测试使用的模型名';
COMMENT ON COLUMN chn_channels.remark IS '备注';
COMMENT ON COLUMN chn_channels.created_by IS '创建者管理员ID';
COMMENT ON COLUMN chn_channels.created_at IS '创建时间';
COMMENT ON COLUMN chn_channels.updated_at IS '更新时间';
COMMENT ON COLUMN chn_channels.is_vip IS '是否VIP专属渠道';
COMMENT ON COLUMN chn_channels.sharing_threshold IS '允许普通租户借用的利用率阈值（如0.6表示利用率<60%时可借用）';
COMMENT ON COLUMN chn_channels.preemption_threshold IS '触发VIP抢占的利用率阈值（如0.8表示利用率>=80%时VIP可抢占）';
COMMENT ON COLUMN chn_channels.borrowing_cooldown_seconds IS '普通租户被抢占后的冷却时间（秒）';
COMMENT ON COLUMN chn_channels.auto_disabled IS '是否被自动禁用：0=否, 1=是（由连续失败触发）';
COMMENT ON TABLE chn_health_scores IS '渠道健康度评分';
COMMENT ON COLUMN chn_health_scores.id IS '主键ID';
COMMENT ON COLUMN chn_health_scores.channel_id IS '关联渠道ID';
COMMENT ON COLUMN chn_health_scores.success_rate IS '请求成功率（0-100）';
COMMENT ON COLUMN chn_health_scores.latency_ms IS '平均延迟（毫秒）';
COMMENT ON COLUMN chn_health_scores.stability_score IS '稳定性评分（0-100，基于延迟波动计算）';
COMMENT ON COLUMN chn_health_scores.consecutive_failures IS '连续失败次数（成功后归零）';
COMMENT ON COLUMN chn_health_scores.health_score IS '综合健康度（0-100）= 成功率×0.40 + 延迟分×0.25 + 稳定性×0.20 + 连续失败分×0.15';
COMMENT ON COLUMN chn_health_scores.calculated_at IS '最近一次计算时间';
COMMENT ON COLUMN chn_health_scores.created_at IS '创建时间';
COMMENT ON COLUMN chn_health_scores.updated_at IS '更新时间';
COMMENT ON TABLE chn_health_snapshots IS '渠道健康度定时快照（每5分钟采集，用于趋势图表）';
COMMENT ON COLUMN chn_health_snapshots.id IS '主键ID';
COMMENT ON COLUMN chn_health_snapshots.channel_id IS '关联渠道ID';
COMMENT ON COLUMN chn_health_snapshots.health_score IS '综合健康度（0-100）';
COMMENT ON COLUMN chn_health_snapshots.success_rate IS '请求成功率（0-100）';
COMMENT ON COLUMN chn_health_snapshots.latency_ms IS '平均延迟（毫秒）';
COMMENT ON COLUMN chn_health_snapshots.stability_score IS '稳定性评分（0-100）';
COMMENT ON COLUMN chn_health_snapshots.consecutive_failures IS '连续失败次数';
COMMENT ON COLUMN chn_health_snapshots.snapshot_at IS '快照时间';
COMMENT ON TABLE clg_changelogs IS '更新日志';
COMMENT ON COLUMN clg_changelogs.id IS '主键ID';
COMMENT ON COLUMN clg_changelogs.version IS '版本号';
COMMENT ON COLUMN clg_changelogs.title IS '标题';
COMMENT ON COLUMN clg_changelogs.content IS 'Markdown 内容';
COMMENT ON COLUMN clg_changelogs.type IS '类型：feature / fix / improvement / breaking';
COMMENT ON COLUMN clg_changelogs.status IS '状态：draft / published';
COMMENT ON COLUMN clg_changelogs.published_at IS '发布时间';
COMMENT ON COLUMN clg_changelogs.created_by IS '创建的管理员 ID';
COMMENT ON TABLE ord_promo_code_usages IS '优惠码使用记录';
COMMENT ON COLUMN ord_promo_code_usages.discount_amount IS '实际折扣金额';
COMMENT ON TABLE ord_promo_codes IS '优惠码';
COMMENT ON COLUMN ord_promo_codes.code IS '优惠码文本（唯一）';
COMMENT ON COLUMN ord_promo_codes.type IS '类型：percentage（折扣百分比）/ fixed（立减固定金额）';
COMMENT ON COLUMN ord_promo_codes.discount_value IS '折扣值（百分比 0-100，立减为金额）';
COMMENT ON COLUMN ord_promo_codes.min_amount IS '最低订单金额';
COMMENT ON COLUMN ord_promo_codes.max_discount IS '最大折扣金额（0=不限）';
COMMENT ON COLUMN ord_promo_codes.plan_ids IS '适用套餐ID数组（NULL=全部）';
COMMENT ON TABLE ord_redemptions IS '兑换码';
COMMENT ON COLUMN ord_redemptions.type IS '类型：quota（额度）/ plan（套餐时长）/ duration（时长天数）';
COMMENT ON COLUMN ord_redemptions.batch_no IS '批次号（批量生成时，便于管理）';
COMMENT ON TABLE spt_feedbacks IS '用户反馈';
COMMENT ON COLUMN spt_feedbacks.id IS '主键ID';
COMMENT ON COLUMN spt_feedbacks.tenant_id IS '所属租户ID';
COMMENT ON COLUMN spt_feedbacks.user_id IS '提交用户ID';
COMMENT ON COLUMN spt_feedbacks.category IS '反馈类型：bug_report / feature_request / suggestion / complaint';
COMMENT ON COLUMN spt_feedbacks.title IS '反馈标题';
COMMENT ON COLUMN spt_feedbacks.description IS '反馈详细描述';
COMMENT ON COLUMN spt_feedbacks.status IS '状态：pending / acknowledged / in_progress / resolved / closed';
COMMENT ON COLUMN spt_feedbacks.priority IS '优先级：low / normal / high / critical';
COMMENT ON COLUMN spt_feedbacks.admin_reply IS '管理员回复';
COMMENT ON COLUMN spt_feedbacks.admin_reply_by IS '回复管理员ID';
COMMENT ON COLUMN spt_feedbacks.admin_reply_at IS '回复时间';
COMMENT ON COLUMN spt_feedbacks.resolution IS '解决方案摘要';
COMMENT ON COLUMN spt_feedbacks.tags IS '自定义标签（JSON 数组）';
COMMENT ON COLUMN spt_feedbacks.metadata IS '元数据（环境信息、截图链接等）';
COMMENT ON TABLE fil_files IS '文件元数据';
COMMENT ON COLUMN fil_files.id IS '主键ID';
COMMENT ON COLUMN fil_files.tenant_id IS '所属租户ID（系统文件为 NULL）';
COMMENT ON COLUMN fil_files.user_id IS '上传者用户ID';
COMMENT ON COLUMN fil_files.filename IS '存储文件名（UUID 或哈希值命名）';
COMMENT ON COLUMN fil_files.original_name IS '用户上传的原始文件名';
COMMENT ON COLUMN fil_files.mime_type IS 'MIME 类型（如 image/png、application/pdf）';
COMMENT ON COLUMN fil_files.size IS '文件大小（字节）';
COMMENT ON COLUMN fil_files.storage_provider IS '存储供应商：s3 / minio / oss / cos';
COMMENT ON COLUMN fil_files.storage_path IS '存储桶中的完整路径';
COMMENT ON COLUMN fil_files.virus_scan_status IS '病毒扫描状态：pending（待扫描）/ scanning（扫描中）/ clean（安全）/ infected（感染）';
COMMENT ON COLUMN fil_files.checksum IS '文件 SHA-256 校验和';
COMMENT ON COLUMN fil_files.created_at IS '创建时间';
COMMENT ON COLUMN fil_files.updated_at IS '更新时间';
COMMENT ON TABLE spt_articles IS '帮助中心-文章';
COMMENT ON COLUMN spt_articles.id IS '主键ID';
COMMENT ON COLUMN spt_articles.category_id IS '所属分类ID';
COMMENT ON COLUMN spt_articles.title IS '文章标题';
COMMENT ON COLUMN spt_articles.slug IS 'URL 友好标识，唯一';
COMMENT ON COLUMN spt_articles.content IS '文章内容（Markdown）';
COMMENT ON COLUMN spt_articles.summary IS '文章摘要';
COMMENT ON COLUMN spt_articles.status IS '状态：draft / published';
COMMENT ON COLUMN spt_articles.author_id IS '作者（管理员）ID';
COMMENT ON COLUMN spt_articles.view_count IS '浏览次数';
COMMENT ON COLUMN spt_articles.sort_order IS '排序序号，越小越靠前';
COMMENT ON COLUMN spt_articles.keywords IS '关键词（JSON 数组）';
COMMENT ON COLUMN spt_articles.published_at IS '发布时间';
COMMENT ON TABLE spt_categories IS '帮助中心-分类';
COMMENT ON COLUMN spt_categories.id IS '主键ID';
COMMENT ON COLUMN spt_categories.parent_id IS '父分类ID，0表示顶级分类';
COMMENT ON COLUMN spt_categories.name IS '分类名称';
COMMENT ON COLUMN spt_categories.slug IS 'URL 友好标识，唯一';
COMMENT ON COLUMN spt_categories.description IS '分类描述';
COMMENT ON COLUMN spt_categories.sort_order IS '排序序号，越小越靠前';
COMMENT ON COLUMN spt_categories.icon IS '图标名称';
COMMENT ON COLUMN spt_categories.is_visible IS '是否对外可见';
COMMENT ON COLUMN spt_categories.article_count IS '分类下文章数量（冗余计数）';
COMMENT ON TABLE mdl_models IS 'AI 模型定义';
COMMENT ON COLUMN mdl_models.id IS '主键ID';
COMMENT ON COLUMN mdl_models.model_id IS '模型唯一标识（如 gpt-4o、claude-3-5-sonnet）';
COMMENT ON COLUMN mdl_models.model_name IS '模型显示名称（如 GPT-4o、Claude 3.5 Sonnet）';
COMMENT ON COLUMN mdl_models.category IS '模型分类：chat（对话）/ embedding（嵌入）/ image（图像）/ audio（音频）/ rerank（重排序）';
COMMENT ON COLUMN mdl_models.status IS '状态：active（可用）/ deprecated（已废弃）/ offline（已下线）';
COMMENT ON COLUMN mdl_models.max_context_tokens IS '最大上下文 token 数';
COMMENT ON COLUMN mdl_models.max_output_tokens IS '最大输出 token 数';
COMMENT ON COLUMN mdl_models.description IS '模型描述';
COMMENT ON COLUMN mdl_models.tags IS '标签（如 reasoning、vision、function_calling）';
COMMENT ON COLUMN mdl_models.capabilities IS '模型能力特性（如 vision、function_calling、reasoning 等）';
COMMENT ON COLUMN mdl_models.created_at IS '创建时间';
COMMENT ON COLUMN mdl_models.updated_at IS '更新时间';
COMMENT ON COLUMN mdl_models.deprecated_at IS '标记弃用的时间（NULL表示未弃用）';
COMMENT ON COLUMN mdl_models.sunset_date IS '计划下线日期（到达后返回410 Gone，NULL表示未设置）';
COMMENT ON COLUMN mdl_models.replacement_model IS '推荐替代模型名（NULL表示无替代）';
COMMENT ON TABLE mdl_pricing IS '模型统一定价表（按次/按量/阶梯三种计费模式）';
COMMENT ON COLUMN mdl_pricing.model_id IS '关联模型ID';
COMMENT ON COLUMN mdl_pricing.billing_mode IS '计费模式：token（按量）/ per_request（按次）/ tiered（阶梯按量）';
COMMENT ON COLUMN mdl_pricing.min_tokens IS '阶梯起始 token 数（仅 tiered 模式，其他模式为 0）';
COMMENT ON COLUMN mdl_pricing.max_tokens IS '阶梯结束 token 数（NULL=无上限，仅 tiered 模式）';
COMMENT ON COLUMN mdl_pricing.input_price IS '每 1M input token 价格（token/tiered 模式）';
COMMENT ON COLUMN mdl_pricing.output_price IS '每 1M output token 价格（token/tiered 模式）';
COMMENT ON COLUMN mdl_pricing.per_request_price IS '按次计费单价（仅 per_request 模式）';
COMMENT ON COLUMN mdl_pricing.cache_read_price IS '缓存读取每 1M token 价格（直接定价）';
COMMENT ON COLUMN mdl_pricing.cache_creation_price IS '缓存创建每 1M token 价格（直接定价）';
COMMENT ON TABLE mdl_tenant_models IS '租户可用模型配置';
COMMENT ON COLUMN mdl_tenant_models.id IS '主键ID';
COMMENT ON COLUMN mdl_tenant_models.tenant_id IS '租户ID';
COMMENT ON COLUMN mdl_tenant_models.model_id IS '模型ID';
COMMENT ON COLUMN mdl_tenant_models.enabled IS '是否启用（禁用后该租户无法调用此模型）';
COMMENT ON COLUMN mdl_tenant_models.custom_input_price IS '租户自定义输入价格（NULL 表示使用默认定价）';
COMMENT ON COLUMN mdl_tenant_models.custom_output_price IS '租户自定义输出价格（NULL 表示使用默认定价）';
COMMENT ON COLUMN mdl_tenant_models.multiplier IS '租户价格倍率（VIP 折扣等，最终价格 = 基础价格 × 模型倍率 × 租户倍率）';
COMMENT ON COLUMN mdl_tenant_models.created_at IS '创建时间';
COMMENT ON COLUMN mdl_tenant_models.updated_at IS '更新时间';
COMMENT ON COLUMN mdl_tenant_models.billing_mode IS '覆盖模型计费方式（NULL表示跟随模型默认）';
COMMENT ON COLUMN mdl_tenant_models.per_request_price IS '按次计费单价（覆盖模型默认，仅 billing_mode = per_request 时有效）';
COMMENT ON COLUMN mdl_tenant_models.discount_ratio IS '折扣比例（如0.8表示八折，NULL表示不打折，优先于 multiplier 使用）';
COMMENT ON COLUMN mdl_tenant_models.max_concurrency IS '单模型并发上限（NULL表示不限制）';
COMMENT ON COLUMN mdl_tenant_models.channel_scope IS '渠道范围覆盖（NULL跟随租户默认，[]表示全部，数组表示指定渠道ID）';
COMMENT ON TABLE ntf_announcements IS '系统公告';
COMMENT ON COLUMN ntf_announcements.id IS '主键ID';
COMMENT ON COLUMN ntf_announcements.title IS '公告标题';
COMMENT ON COLUMN ntf_announcements.type IS '公告类型：info（通知）/ warning（警告）/ important（重要）';
COMMENT ON COLUMN ntf_announcements.content IS '公告内容';
COMMENT ON COLUMN ntf_announcements.status IS '状态：draft（草稿）/ published（已发布）/ archived（已归档）';
COMMENT ON COLUMN ntf_announcements.is_pinned IS '是否置顶：0=否, 1=是';
COMMENT ON COLUMN ntf_announcements.display_position IS '展示位置：login（登录页）/ console（控制台）/ both（双位置）';
COMMENT ON COLUMN ntf_announcements.effective_at IS '生效时间（NULL=立即生效）';
COMMENT ON COLUMN ntf_announcements.expires_at IS '过期时间（NULL=永不过期）';
COMMENT ON COLUMN ntf_announcements.created_by IS '创建者（管理员ID）';
COMMENT ON COLUMN ntf_announcements.created_at IS '创建时间';
COMMENT ON COLUMN ntf_announcements.updated_at IS '更新时间';
COMMENT ON TABLE ntf_messages IS '站内消息';
COMMENT ON COLUMN ntf_messages.id IS '主键ID';
COMMENT ON COLUMN ntf_messages.tenant_id IS '所属租户ID';
COMMENT ON COLUMN ntf_messages.user_id IS '接收用户ID（广播消息时为 NULL）';
COMMENT ON COLUMN ntf_messages.type IS '消息类型：billing（计费）/ system（系统）/ security（安全）/ invitation（邀请）/ announcement（公告）';
COMMENT ON COLUMN ntf_messages.title IS '消息标题';
COMMENT ON COLUMN ntf_messages.content IS '消息内容';
COMMENT ON COLUMN ntf_messages.channel IS '发送渠道：in_app（站内）/ email（邮件）/ both（双渠道）';
COMMENT ON COLUMN ntf_messages.is_read IS '是否已读：0=未读, 1=已读';
COMMENT ON COLUMN ntf_messages.is_broadcast IS '是否广播消息：0=个人消息, 1=广播消息';
COMMENT ON COLUMN ntf_messages.metadata IS '附加元数据（JSONB，如关联资源ID、跳转链接等）';
COMMENT ON COLUMN ntf_messages.created_at IS '创建时间';
COMMENT ON TABLE ntf_preferences IS '通知偏好设置';
COMMENT ON COLUMN ntf_preferences.id IS '主键ID';
COMMENT ON COLUMN ntf_preferences.tenant_id IS '所属租户ID';
COMMENT ON COLUMN ntf_preferences.user_id IS '用户ID（组织级偏好时为 NULL）';
COMMENT ON COLUMN ntf_preferences.scope IS '偏好范围：user（用户级）/ org（组织级）';
COMMENT ON COLUMN ntf_preferences.preferences IS '偏好配置（JSONB，如 {"billing":{"email":true,"in_app":true},"security":{"email":true,"in_app":true}}）';
COMMENT ON COLUMN ntf_preferences.created_at IS '创建时间';
COMMENT ON COLUMN ntf_preferences.updated_at IS '更新时间';
COMMENT ON TABLE ntf_read_status IS '广播消息已读状态';
COMMENT ON COLUMN ntf_read_status.id IS '主键ID';
COMMENT ON COLUMN ntf_read_status.message_id IS '广播消息ID';
COMMENT ON COLUMN ntf_read_status.user_id IS '已读用户ID';
COMMENT ON COLUMN ntf_read_status.read_at IS '已读时间';
COMMENT ON TABLE ntf_send_log IS '通知发送记录';
COMMENT ON COLUMN ntf_send_log.id IS '主键ID';
COMMENT ON COLUMN ntf_send_log.tenant_id IS '租户ID（系统级通知为 NULL）';
COMMENT ON COLUMN ntf_send_log.user_id IS '目标用户ID';
COMMENT ON COLUMN ntf_send_log.template_code IS '使用的通知模板编码';
COMMENT ON COLUMN ntf_send_log.channel IS '发送渠道：email / sms / webhook';
COMMENT ON COLUMN ntf_send_log.recipient IS '接收方（邮箱地址/手机号/Webhook URL）';
COMMENT ON COLUMN ntf_send_log.subject IS '发送主题';
COMMENT ON COLUMN ntf_send_log.body IS '发送内容（渲染后的最终内容）';
COMMENT ON COLUMN ntf_send_log.status IS '状态：pending（待发送）/ sent（已发送）/ failed（发送失败）';
COMMENT ON COLUMN ntf_send_log.error_message IS '失败时的错误信息';
COMMENT ON COLUMN ntf_send_log.sent_at IS '实际发送时间';
COMMENT ON COLUMN ntf_send_log.retry_count IS '重试次数（最多重试 3 次）';
COMMENT ON COLUMN ntf_send_log.created_at IS '创建时间';
COMMENT ON COLUMN ntf_send_log.updated_at IS '更新时间';
COMMENT ON TABLE ntf_templates IS '通知模板';
COMMENT ON COLUMN ntf_templates.id IS '主键ID';
COMMENT ON COLUMN ntf_templates.code IS '模板编码（唯一标识，如 email_verify_code、balance_warning）';
COMMENT ON COLUMN ntf_templates.channel IS '发送渠道：email（邮件）/ sms（短信）/ webhook（Webhook）';
COMMENT ON COLUMN ntf_templates.subject IS '邮件/消息主题';
COMMENT ON COLUMN ntf_templates.body_template IS '消息体模板（支持变量占位符，如 {{.code}}）';
COMMENT ON COLUMN ntf_templates.variables IS '模板变量列表（JSONB 数组，如 ["username", "tenant_name", "code"]）';
COMMENT ON COLUMN ntf_templates.status IS '状态：active（启用）/ disabled（禁用）';
COMMENT ON COLUMN ntf_templates.created_at IS '创建时间';
COMMENT ON COLUMN ntf_templates.updated_at IS '更新时间';
COMMENT ON TABLE opn_apps IS '开放平台应用';
COMMENT ON COLUMN opn_apps.id IS '主键ID';
COMMENT ON COLUMN opn_apps.tenant_id IS '所属租户ID';
COMMENT ON COLUMN opn_apps.name IS '应用名称';
COMMENT ON COLUMN opn_apps.description IS '应用描述';
COMMENT ON COLUMN opn_apps.app_id IS '应用标识（opn_xxx 格式）';
COMMENT ON COLUMN opn_apps.app_secret_hash IS 'App Secret 哈希（bcrypt）';
COMMENT ON COLUMN opn_apps.permissions IS '权限范围（JSON 数组）';
COMMENT ON COLUMN opn_apps.ip_whitelist IS 'IP 白名单（JSON 数组，为空则不限制）';
COMMENT ON COLUMN opn_apps.callback_url IS 'OAuth 回调 URL';
COMMENT ON COLUMN opn_apps.is_sandbox IS '是否沙箱应用';
COMMENT ON COLUMN opn_apps.status IS '状态：active（启用）/ disabled（禁用）';
COMMENT ON COLUMN opn_apps.rate_limit IS '每分钟请求上限';
COMMENT ON COLUMN opn_apps.last_used_at IS '最后使用时间';
COMMENT ON COLUMN opn_apps.created_at IS '创建时间';
COMMENT ON COLUMN opn_apps.updated_at IS '更新时间';
COMMENT ON TABLE ops_alert_events IS '告警事件';
COMMENT ON COLUMN ops_alert_events.id IS '主键ID';
COMMENT ON COLUMN ops_alert_events.rule_id IS '关联规则ID';
COMMENT ON COLUMN ops_alert_events.rule_name IS '规则名称（冗余存储）';
COMMENT ON COLUMN ops_alert_events.metric_type IS '指标类型';
COMMENT ON COLUMN ops_alert_events.level IS '告警级别：info / warning / critical';
COMMENT ON COLUMN ops_alert_events.status IS '状态：firing（触发中）/ acknowledged（已确认）/ resolved（已恢复）';
COMMENT ON COLUMN ops_alert_events.trigger_value IS '触发时的实际指标值';
COMMENT ON COLUMN ops_alert_events.threshold_value IS '规则阈值';
COMMENT ON COLUMN ops_alert_events.trigger_message IS '触发消息描述';
COMMENT ON COLUMN ops_alert_events.acknowledged_by IS '确认人管理员ID';
COMMENT ON COLUMN ops_alert_events.acknowledged_at IS '确认时间';
COMMENT ON COLUMN ops_alert_events.resolve_notes IS '处理备注';
COMMENT ON COLUMN ops_alert_events.resolved_by IS '解决人管理员ID';
COMMENT ON COLUMN ops_alert_events.resolved_at IS '解决时间';
COMMENT ON COLUMN ops_alert_events.notified_methods IS '已发送的通知方式';
COMMENT ON COLUMN ops_alert_events.created_at IS '创建时间';
COMMENT ON COLUMN ops_alert_events.updated_at IS '更新时间';
COMMENT ON TABLE ops_alert_rules IS '告警规则';
COMMENT ON COLUMN ops_alert_rules.id IS '主键ID';
COMMENT ON COLUMN ops_alert_rules.name IS '规则名称';
COMMENT ON COLUMN ops_alert_rules.metric_type IS '指标类型：api.error_rate / api.p95_latency / api.p99_latency / api.qps / system.cpu_percent / system.memory_percent / system.disk_percent / db.active_connections / redis.used_memory_mb';
COMMENT ON COLUMN ops_alert_rules.condition IS '比较条件：gt / gte / lt / lte / eq';
COMMENT ON COLUMN ops_alert_rules.threshold IS '阈值';
COMMENT ON COLUMN ops_alert_rules.duration_seconds IS '持续时间（秒），0表示立即触发';
COMMENT ON COLUMN ops_alert_rules.notification_methods IS '通知方式数组：email / webhook / in_app';
COMMENT ON COLUMN ops_alert_rules.webhook_url IS 'Webhook回调地址';
COMMENT ON COLUMN ops_alert_rules.level IS '告警级别：info / warning / critical';
COMMENT ON COLUMN ops_alert_rules.is_enabled IS '是否启用';
COMMENT ON COLUMN ops_alert_rules.cooldown_seconds IS '冷却时间（秒），同一规则两次告警最小间隔';
COMMENT ON COLUMN ops_alert_rules.last_triggered_at IS '上次触发时间';
COMMENT ON COLUMN ops_alert_rules.notify_user_ids IS '通知接收人管理员ID列表';
COMMENT ON COLUMN ops_alert_rules.created_at IS '创建时间';
COMMENT ON COLUMN ops_alert_rules.updated_at IS '更新时间';
COMMENT ON TABLE sys_options IS '系统配置（键值对，支持动态更新）';
COMMENT ON COLUMN sys_options.id IS '主键ID';
COMMENT ON COLUMN sys_options.key IS '配置键（唯一标识，如 site_name、register_enabled）';
COMMENT ON COLUMN sys_options.value IS '配置值';
COMMENT ON COLUMN sys_options.description IS '配置说明';
COMMENT ON COLUMN sys_options.category IS '配置分类（如 general、security、email、payment）';
COMMENT ON COLUMN sys_options.is_public IS '是否公开（前端可直接获取，如站点名称、注册开关）';
COMMENT ON COLUMN sys_options.created_at IS '创建时间';
COMMENT ON COLUMN sys_options.updated_at IS '更新时间';
COMMENT ON TABLE ord_orders IS '订单';
COMMENT ON COLUMN ord_orders.id IS '主键ID';
COMMENT ON COLUMN ord_orders.order_no IS '订单号（唯一，格式 ORD + 时间戳 + 随机数）';
COMMENT ON COLUMN ord_orders.tenant_id IS '租户ID';
COMMENT ON COLUMN ord_orders.user_id IS '下单用户ID';
COMMENT ON COLUMN ord_orders.order_type IS '订单类型：new_plan（新购）/ renew（续费）/ upgrade（升级）/ downgrade（降级）/ recharge（充值）';
COMMENT ON COLUMN ord_orders.plan_id IS '套餐ID（充值订单时为 NULL）';
COMMENT ON COLUMN ord_orders.amount IS '原始金额';
COMMENT ON COLUMN ord_orders.discount_amount IS '优惠金额';
COMMENT ON COLUMN ord_orders.final_amount IS '最终金额';
COMMENT ON COLUMN ord_orders.currency IS '货币';
COMMENT ON COLUMN ord_orders.payment_channel IS '支付渠道';
COMMENT ON COLUMN ord_orders.payment_method IS '支付方式描述';
COMMENT ON COLUMN ord_orders.payment_no IS '第三方支付流水号';
COMMENT ON COLUMN ord_orders.status IS '订单状态';
COMMENT ON COLUMN ord_orders.paid_at IS '支付时间';
COMMENT ON COLUMN ord_orders.fulfilled_at IS '履约完成时间';
COMMENT ON COLUMN ord_orders.expired_at IS '过期时间（未支付 30 分钟后自动过期）';
COMMENT ON COLUMN ord_orders.cancelled_at IS '取消时间';
COMMENT ON COLUMN ord_orders.related_order_id IS '关联订单ID（退款时指向原始订单）';
COMMENT ON COLUMN ord_orders.description IS '订单描述';
COMMENT ON COLUMN ord_orders.created_at IS '创建时间';
COMMENT ON COLUMN ord_orders.updated_at IS '更新时间';
COMMENT ON TABLE ord_payment_channels IS '支付渠道配置';
COMMENT ON COLUMN ord_payment_channels.id IS '主键ID';
COMMENT ON COLUMN ord_payment_channels.channel IS '渠道标识（alipay/wechat/stripe/mock）';
COMMENT ON COLUMN ord_payment_channels.name IS '显示名称';
COMMENT ON COLUMN ord_payment_channels.config IS '渠道配置（JSONB，含 API 密钥等敏感信息）';
COMMENT ON COLUMN ord_payment_channels.is_enabled IS '是否启用';
COMMENT ON COLUMN ord_payment_channels.sort_order IS '排序权重';
COMMENT ON COLUMN ord_payment_channels.created_at IS '创建时间';
COMMENT ON COLUMN ord_payment_channels.updated_at IS '更新时间';
COMMENT ON COLUMN ord_payment_channels.payment_type IS '子支付方式（alipay/wxpay 等，空表示该渠道支持所有方式）';
COMMENT ON COLUMN ord_payment_channels.callback_url IS '支付回调地址覆盖（为空则使用系统默认）';
COMMENT ON COLUMN ord_payment_channels.return_url IS '支付完成后前端跳转地址覆盖';
COMMENT ON TABLE ord_refunds IS '退款记录';
COMMENT ON COLUMN ord_refunds.id IS '主键ID';
COMMENT ON COLUMN ord_refunds.order_id IS '关联订单ID';
COMMENT ON COLUMN ord_refunds.tenant_id IS '租户ID';
COMMENT ON COLUMN ord_refunds.amount IS '退款金额';
COMMENT ON COLUMN ord_refunds.reason IS '退款原因';
COMMENT ON COLUMN ord_refunds.status IS '退款状态';
COMMENT ON COLUMN ord_refunds.payment_channel IS '原支付渠道';
COMMENT ON COLUMN ord_refunds.payment_refund_id IS '第三方退款流水号';
COMMENT ON COLUMN ord_refunds.approved_by IS '审批人（管理员ID）';
COMMENT ON COLUMN ord_refunds.approved_at IS '审批时间';
COMMENT ON COLUMN ord_refunds.created_at IS '创建时间';
COMMENT ON COLUMN ord_refunds.updated_at IS '更新时间';
COMMENT ON TABLE pln_feature_flags IS '功能开关配置';
COMMENT ON COLUMN pln_feature_flags.id IS '主键ID';
COMMENT ON COLUMN pln_feature_flags.feature_key IS '功能标识（如 api_docs, export_csv）';
COMMENT ON COLUMN pln_feature_flags.description IS '功能描述';
COMMENT ON COLUMN pln_feature_flags.default_enabled IS '默认是否启用';
COMMENT ON COLUMN pln_feature_flags.enabled IS '当前是否启用（计算后的最终值）';
COMMENT ON COLUMN pln_feature_flags.source IS '来源：plan（套餐）/ tenant（租户覆盖）/ manual（手动）';
COMMENT ON COLUMN pln_feature_flags.source_id IS '来源ID（plan_id 或 tenant_id）';
COMMENT ON COLUMN pln_feature_flags.tenant_id IS '关联租户ID（租户级覆盖时使用）';
COMMENT ON COLUMN pln_feature_flags.plan_id IS '关联套餐ID（套餐级配置时使用）';
COMMENT ON COLUMN pln_feature_flags.created_at IS '创建时间';
COMMENT ON COLUMN pln_feature_flags.updated_at IS '更新时间';
COMMENT ON TABLE pln_plans IS '套餐定义';
COMMENT ON COLUMN pln_plans.id IS '主键ID';
COMMENT ON COLUMN pln_plans.name IS '套餐显示名称';
COMMENT ON COLUMN pln_plans.identifier IS '套餐唯一标识（free/basic/pro/enterprise）';
COMMENT ON COLUMN pln_plans.description IS '套餐描述（面向用户的营销文案）';
COMMENT ON COLUMN pln_plans.monthly_price IS '月度价格（CNY）';
COMMENT ON COLUMN pln_plans.yearly_price IS '年度价格（CNY，通常为月价×10）';
COMMENT ON COLUMN pln_plans.status IS '状态：active（上架）/ archived（下架）';
COMMENT ON COLUMN pln_plans.monthly_quota_tokens IS '每月 Token 配额（0=不限）';
COMMENT ON COLUMN pln_plans.allowed_models IS '允许使用的模型列表（NULL=全部，空数组=无）';
COMMENT ON COLUMN pln_plans.is_recommended IS '是否推荐';
COMMENT ON COLUMN pln_plans.sort_order IS '排序权重（数字越小越靠前）';
COMMENT ON COLUMN pln_plans.created_at IS '创建时间';
COMMENT ON COLUMN pln_plans.updated_at IS '更新时间';
COMMENT ON TABLE pln_tenant_plans IS '租户套餐订阅';
COMMENT ON COLUMN pln_tenant_plans.id IS '主键ID';
COMMENT ON COLUMN pln_tenant_plans.tenant_id IS '租户ID';
COMMENT ON COLUMN pln_tenant_plans.plan_id IS '套餐ID';
COMMENT ON COLUMN pln_tenant_plans.status IS '状态：pending（待生效）/ active（生效中）/ expired（已过期）/ cancelled（已取消）';
COMMENT ON COLUMN pln_tenant_plans.start_at IS '生效起始时间';
COMMENT ON COLUMN pln_tenant_plans.end_at IS '到期时间';
COMMENT ON COLUMN pln_tenant_plans.auto_renew IS '是否自动续费';
COMMENT ON COLUMN pln_tenant_plans.monthly_quota_tokens IS '月度 Token 配额快照';
COMMENT ON COLUMN pln_tenant_plans.used_tokens IS '本月已使用 Token';
COMMENT ON COLUMN pln_tenant_plans.last_reset_at IS '上次配额重置时间';
COMMENT ON COLUMN pln_tenant_plans.cancelled_at IS '取消时间';
COMMENT ON COLUMN pln_tenant_plans.created_at IS '创建时间';
COMMENT ON COLUMN pln_tenant_plans.updated_at IS '更新时间';
COMMENT ON TABLE tnt_projects IS '项目（租户下的逻辑分组，用于组织 API Key 和统计用量）';
COMMENT ON COLUMN tnt_projects.id IS '主键ID';
COMMENT ON COLUMN tnt_projects.tenant_id IS '所属租户ID';
COMMENT ON COLUMN tnt_projects.name IS '项目名称';
COMMENT ON COLUMN tnt_projects.description IS '项目描述';
COMMENT ON COLUMN tnt_projects.status IS '状态：active（活跃）/ archived（归档）/ budget_exhausted（预算耗尽）';
COMMENT ON COLUMN tnt_projects.budget IS '项目预算上限（NUMERIC(20,10) 金额，NULL 表示不限制）';
COMMENT ON COLUMN tnt_projects.created_by IS '创建者用户ID';
COMMENT ON COLUMN tnt_projects.created_at IS '创建时间';
COMMENT ON COLUMN tnt_projects.updated_at IS '更新时间';
COMMENT ON TABLE sys_admin_data_scopes IS '管理员数据范围配置';
COMMENT ON COLUMN sys_admin_data_scopes.id IS '主键ID';
COMMENT ON COLUMN sys_admin_data_scopes.admin_user_id IS '关联的管理员用户ID';
COMMENT ON COLUMN sys_admin_data_scopes.scope_type IS '范围类型：all（全部）/ tenant_group（租户组）/ tenant（指定租户）';
COMMENT ON COLUMN sys_admin_data_scopes.scope_value IS '范围值（tenant_group时为组名，tenant时为租户ID列表，逗号分隔）';
COMMENT ON COLUMN sys_admin_data_scopes.created_at IS '创建时间';
COMMENT ON COLUMN sys_admin_data_scopes.updated_at IS '更新时间';
COMMENT ON TABLE sys_admin_role_perms IS '管理员角色权限关联';
COMMENT ON COLUMN sys_admin_role_perms.id IS '主键ID';
COMMENT ON COLUMN sys_admin_role_perms.admin_user_id IS '关联的管理员用户ID';
COMMENT ON COLUMN sys_admin_role_perms.permission_point IS '权限点标识（如 tenant:create、channel:edit）';
COMMENT ON COLUMN sys_admin_role_perms.created_at IS '创建时间';
COMMENT ON COLUMN sys_admin_role_perms.updated_at IS '更新时间';
COMMENT ON TABLE sys_admin_users IS '管理后台用户';
COMMENT ON COLUMN sys_admin_users.id IS '主键ID';
COMMENT ON COLUMN sys_admin_users.username IS '登录用户名';
COMMENT ON COLUMN sys_admin_users.password_hash IS '密码哈希（bcrypt）';
COMMENT ON COLUMN sys_admin_users.email IS '邮箱地址（可选，可为NULL）';
COMMENT ON COLUMN sys_admin_users.display_name IS '显示名称';
COMMENT ON COLUMN sys_admin_users.role IS '角色：super_admin（全权限）/ admin（可配置权限）';
COMMENT ON COLUMN sys_admin_users.status IS '状态：active（启用）/ disabled（禁用）';
COMMENT ON COLUMN sys_admin_users.last_login_at IS '最后登录时间';
COMMENT ON COLUMN sys_admin_users.last_login_ip IS '最后登录IP';
COMMENT ON COLUMN sys_admin_users.created_at IS '创建时间';
COMMENT ON COLUMN sys_admin_users.updated_at IS '更新时间';
COMMENT ON COLUMN sys_admin_users.totp_secret IS 'TOTP 密钥（AES-256 加密存储）';
COMMENT ON COLUMN sys_admin_users.totp_enabled IS '是否启用双因素认证';
COMMENT ON COLUMN sys_admin_users.backup_codes IS '备用恢复码（bcrypt 哈希存储）';
COMMENT ON TABLE sys_email_verify_codes IS '邮箱验证码';
COMMENT ON COLUMN sys_email_verify_codes.id IS '主键ID';
COMMENT ON COLUMN sys_email_verify_codes.email IS '目标邮箱地址';
COMMENT ON COLUMN sys_email_verify_codes.code IS '验证码（6位数字）';
COMMENT ON COLUMN sys_email_verify_codes.purpose IS '用途：register（注册）/ reset_password（重置密码）/ change_email（更换邮箱）';
COMMENT ON COLUMN sys_email_verify_codes.expires_at IS '过期时间（10分钟有效）';
COMMENT ON COLUMN sys_email_verify_codes.used_at IS '使用时间（NULL表示未使用）';
COMMENT ON COLUMN sys_email_verify_codes.created_at IS '创建时间';
COMMENT ON COLUMN sys_email_verify_codes.updated_at IS '更新时间';
COMMENT ON TABLE sys_idempotency_records IS '幂等记录（防止重复提交）';
COMMENT ON COLUMN sys_idempotency_records.id IS '主键ID';
COMMENT ON COLUMN sys_idempotency_records.idempotency_key IS '幂等键（来自请求头 Idempotency-Key）';
COMMENT ON COLUMN sys_idempotency_records.request_hash IS '请求体哈希（SHA-256，用于校验请求一致性）';
COMMENT ON COLUMN sys_idempotency_records.response_body IS '首次处理的响应体（幂等返回时复用）';
COMMENT ON COLUMN sys_idempotency_records.status IS '状态：processing（处理中）/ completed（已完成）/ failed（失败）';
COMMENT ON COLUMN sys_idempotency_records.expires_at IS '过期时间（过期后记录可清理）';
COMMENT ON COLUMN sys_idempotency_records.created_at IS '创建时间';
COMMENT ON COLUMN sys_idempotency_records.updated_at IS '更新时间';
COMMENT ON TABLE sys_sessions IS '用户会话（JWT Refresh Token 存储）';
COMMENT ON COLUMN sys_sessions.id IS '主键ID';
COMMENT ON COLUMN sys_sessions.user_type IS '用户类型：admin（管理后台）/ tenant（租户控制台）';
COMMENT ON COLUMN sys_sessions.user_id IS '用户ID';
COMMENT ON COLUMN sys_sessions.tenant_id IS '租户ID（admin类型时为NULL）';
COMMENT ON COLUMN sys_sessions.refresh_token_hash IS 'Refresh Token 哈希值';
COMMENT ON COLUMN sys_sessions.device_info IS '设备信息（JSONB：浏览器、操作系统等）';
COMMENT ON COLUMN sys_sessions.ip_address IS '登录IP地址';
COMMENT ON COLUMN sys_sessions.expires_at IS 'Token 过期时间';
COMMENT ON COLUMN sys_sessions.created_at IS '创建时间';
COMMENT ON COLUMN sys_sessions.updated_at IS '更新时间';
COMMENT ON TABLE spt_attachments IS '工单附件';
COMMENT ON COLUMN spt_attachments.id IS '主键ID';
COMMENT ON COLUMN spt_attachments.ticket_id IS '工单ID';
COMMENT ON COLUMN spt_attachments.reply_id IS '回复ID（NULL表示工单创建时的附件）';
COMMENT ON COLUMN spt_attachments.file_name IS '文件名';
COMMENT ON COLUMN spt_attachments.file_url IS '文件访问地址';
COMMENT ON COLUMN spt_attachments.file_size IS '文件大小（字节）';
COMMENT ON COLUMN spt_attachments.content_type IS '文件MIME类型';
COMMENT ON COLUMN spt_attachments.created_at IS '上传时间';
COMMENT ON TABLE spt_replies IS '工单回复';
COMMENT ON COLUMN spt_replies.id IS '主键ID';
COMMENT ON COLUMN spt_replies.ticket_id IS '工单ID';
COMMENT ON COLUMN spt_replies.user_id IS '回复者用户ID';
COMMENT ON COLUMN spt_replies.user_type IS '回复者类型：admin（管理员）/ tenant（租户用户）';
COMMENT ON COLUMN spt_replies.content IS '回复内容';
COMMENT ON COLUMN spt_replies.created_at IS '回复时间';
COMMENT ON TABLE spt_tickets IS '工单';
COMMENT ON COLUMN spt_tickets.id IS '主键ID';
COMMENT ON COLUMN spt_tickets.tenant_id IS '所属租户ID';
COMMENT ON COLUMN spt_tickets.user_id IS '创建者用户ID';
COMMENT ON COLUMN spt_tickets.category IS '分类：billing（计费）/ technical（技术）/ feature_request（功能建议）/ other（其他）';
COMMENT ON COLUMN spt_tickets.title IS '工单标题';
COMMENT ON COLUMN spt_tickets.description IS '工单描述';
COMMENT ON COLUMN spt_tickets.urgency IS '紧急程度：low（低）/ normal（普通）/ high（高）/ urgent（紧急）';
COMMENT ON COLUMN spt_tickets.status IS '状态：pending（待处理）/ processing（处理中）/ replied（已回复）/ closed（已关闭）/ reopened（已重开）';
COMMENT ON COLUMN spt_tickets.assigned_admin_id IS '处理人管理员ID';
COMMENT ON COLUMN spt_tickets.created_at IS '创建时间';
COMMENT ON COLUMN spt_tickets.updated_at IS '更新时间';
COMMENT ON TABLE tnt_invitations IS '租户邀请链接';
COMMENT ON COLUMN tnt_invitations.id IS '主键ID';
COMMENT ON COLUMN tnt_invitations.tenant_id IS '所属租户ID';
COMMENT ON COLUMN tnt_invitations.code IS '邀请码（唯一标识）';
COMMENT ON COLUMN tnt_invitations.invited_email IS '被邀请人邮箱（可选，指定后仅该邮箱可使用）';
COMMENT ON COLUMN tnt_invitations.role IS '邀请后分配的角色：owner / admin / member';
COMMENT ON COLUMN tnt_invitations.expires_at IS '过期时间：7天 / 30天 / 永久（NULL）';
COMMENT ON COLUMN tnt_invitations.used_by_user_id IS '使用该邀请注册的用户ID（NULL表示未使用）';
COMMENT ON COLUMN tnt_invitations.used_at IS '使用时间';
COMMENT ON COLUMN tnt_invitations.created_by IS '创建者用户ID';
COMMENT ON COLUMN tnt_invitations.created_at IS '创建时间';
COMMENT ON COLUMN tnt_invitations.updated_at IS '更新时间';
COMMENT ON TABLE tnt_member_imports IS '成员批量导入记录';
COMMENT ON COLUMN tnt_member_imports.id IS '主键ID';
COMMENT ON COLUMN tnt_member_imports.tenant_id IS '所属租户ID';
COMMENT ON COLUMN tnt_member_imports.filename IS '上传文件名';
COMMENT ON COLUMN tnt_member_imports.total_count IS '总行数';
COMMENT ON COLUMN tnt_member_imports.success_count IS '成功数';
COMMENT ON COLUMN tnt_member_imports.fail_count IS '失败数';
COMMENT ON COLUMN tnt_member_imports.skip_count IS '跳过数（重复）';
COMMENT ON COLUMN tnt_member_imports.status IS '状态：pending/processing/completed/failed';
COMMENT ON COLUMN tnt_member_imports.error_message IS '整体错误信息';
COMMENT ON COLUMN tnt_member_imports.result_json IS '逐行结果 [{row,username,status,error}]';
COMMENT ON COLUMN tnt_member_imports.created_by IS '创建者用户ID';
COMMENT ON COLUMN tnt_member_imports.created_at IS '创建时间';
COMMENT ON COLUMN tnt_member_imports.updated_at IS '更新时间';
COMMENT ON TABLE tnt_member_model_scopes IS '成员模型分配映射';
COMMENT ON COLUMN tnt_member_model_scopes.id IS '主键ID';
COMMENT ON COLUMN tnt_member_model_scopes.tenant_id IS '所属租户ID';
COMMENT ON COLUMN tnt_member_model_scopes.user_id IS '成员用户ID';
COMMENT ON COLUMN tnt_member_model_scopes.model_id IS '模型ID';
COMMENT ON COLUMN tnt_member_model_scopes.created_at IS '创建时间';
COMMENT ON TABLE tnt_oauth_identities IS '租户用户 OAuth 身份绑定';
COMMENT ON COLUMN tnt_oauth_identities.id IS '主键ID';
COMMENT ON COLUMN tnt_oauth_identities.tenant_id IS '所属租户ID';
COMMENT ON COLUMN tnt_oauth_identities.user_id IS '关联的用户ID';
COMMENT ON COLUMN tnt_oauth_identities.provider IS 'OAuth 供应商：github / google';
COMMENT ON COLUMN tnt_oauth_identities.provider_user_id IS '供应商用户ID';
COMMENT ON COLUMN tnt_oauth_identities.provider_username IS '供应商用户名';
COMMENT ON COLUMN tnt_oauth_identities.email IS '供应商返回的邮箱';
COMMENT ON COLUMN tnt_oauth_identities.avatar_url IS '供应商返回的头像URL';
COMMENT ON COLUMN tnt_oauth_identities.access_token IS '加密存储的 access_token';
COMMENT ON COLUMN tnt_oauth_identities.refresh_token IS '加密存储的 refresh_token';
COMMENT ON COLUMN tnt_oauth_identities.token_expires_at IS 'Token 过期时间';
COMMENT ON COLUMN tnt_oauth_identities.raw_data IS '供应商原始返回数据';
COMMENT ON TABLE tnt_tenants IS '租户/组织';
COMMENT ON COLUMN tnt_tenants.id IS '主键ID';
COMMENT ON COLUMN tnt_tenants.name IS '租户显示名称（如公司名）';
COMMENT ON COLUMN tnt_tenants.code IS '租户代码（唯一标识，用于 RAM 账号格式 username@tenant_code）';
COMMENT ON COLUMN tnt_tenants.logo_url IS '租户 Logo URL';
COMMENT ON COLUMN tnt_tenants.owner_user_id IS '所有者用户ID（关联 tnt_users.id）';
COMMENT ON COLUMN tnt_tenants.status IS '状态：trial（试用）/ active（活跃）/ past_due（逾期）/ frozen（冻结）/ terminated（已终止）/ free（免费版）/ suspended（暂停）/ closed（关闭）';
COMMENT ON COLUMN tnt_tenants.max_members IS '最大成员数上限';
COMMENT ON COLUMN tnt_tenants.settings IS '租户配置（JSONB）：通知偏好、安全策略、IP 白名单等';
COMMENT ON COLUMN tnt_tenants.created_at IS '创建时间';
COMMENT ON COLUMN tnt_tenants.updated_at IS '更新时间';
COMMENT ON COLUMN tnt_tenants.trial_ends_at IS '试用期结束时间';
COMMENT ON COLUMN tnt_tenants.grace_period_ends_at IS '宽限期结束时间（套餐到期后 7 天）';
COMMENT ON COLUMN tnt_tenants.frozen_at IS '冻结时间';
COMMENT ON COLUMN tnt_tenants.closing_requested_at IS '主动申请注销时间（7 天冷静期）';
COMMENT ON COLUMN tnt_tenants.data_removal_at IS '数据清除时间（冻结 30 天后）';
COMMENT ON COLUMN tnt_tenants.max_concurrency IS '租户总并发上限（0表示不限制）';
COMMENT ON COLUMN tnt_tenants.default_channel_scope IS '默认渠道范围（NULL或[]表示全部可用，否则为channel_id数组）';
COMMENT ON TABLE tnt_users IS '租户用户（RAM 账号，格式 username@tenant_code）';
COMMENT ON COLUMN tnt_users.id IS '主键ID';
COMMENT ON COLUMN tnt_users.tenant_id IS '所属租户ID';
COMMENT ON COLUMN tnt_users.username IS '用户名（租户内唯一）';
COMMENT ON COLUMN tnt_users.email IS '邮箱地址（租户内唯一）';
COMMENT ON COLUMN tnt_users.password_hash IS '密码哈希（bcrypt）';
COMMENT ON COLUMN tnt_users.display_name IS '显示名称';
COMMENT ON COLUMN tnt_users.role IS '角色：owner（所有者）/ admin（管理员）/ member（成员）';
COMMENT ON COLUMN tnt_users.status IS '状态：active（正常）/ disabled（禁用）/ locked（锁定）';
COMMENT ON COLUMN tnt_users.last_login_at IS '最后登录时间';
COMMENT ON COLUMN tnt_users.last_login_ip IS '最后登录IP';
COMMENT ON COLUMN tnt_users.failed_attempts IS '连续登录失败次数（成功登录后归零）';
COMMENT ON COLUMN tnt_users.locked_until IS '锁定截止时间（连续5次失败后锁定30分钟）';
COMMENT ON COLUMN tnt_users.created_at IS '创建时间';
COMMENT ON COLUMN tnt_users.updated_at IS '更新时间';
COMMENT ON COLUMN tnt_users.totp_secret IS 'TOTP 密钥（AES-256 加密存储）';
COMMENT ON COLUMN tnt_users.totp_enabled IS '是否启用双因素认证';
COMMENT ON COLUMN tnt_users.backup_codes IS '备用恢复码（bcrypt 哈希存储）';
COMMENT ON TABLE tsk_task_logs IS '任务执行日志';
COMMENT ON COLUMN tsk_task_logs.id IS '主键ID';
COMMENT ON COLUMN tsk_task_logs.task_id IS '关联任务ID';
COMMENT ON COLUMN tsk_task_logs.level IS '日志级别：info（信息）/ warn（警告）/ error（错误）';
COMMENT ON COLUMN tsk_task_logs.message IS '日志内容';
COMMENT ON COLUMN tsk_task_logs.created_at IS '创建时间';
COMMENT ON COLUMN tsk_task_logs.updated_at IS '更新时间';
COMMENT ON TABLE tsk_tasks IS '异步任务（用于邮件发送、数据导出、定时对账等后台任务）';
COMMENT ON COLUMN tsk_tasks.id IS '主键ID';
COMMENT ON COLUMN tsk_tasks.name IS '任务名称（如 "发送邮件"、"日对账"、"数据导出"）';
COMMENT ON COLUMN tsk_tasks.handler IS 'Handler 函数路径（用于任务路由）';
COMMENT ON COLUMN tsk_tasks.status IS '状态：pending（待执行）/ running（执行中）/ succeeded（成功）/ failed（失败）/ cancelled（已取消）';
COMMENT ON COLUMN tsk_tasks.payload IS '任务输入参数（JSONB）';
COMMENT ON COLUMN tsk_tasks.result IS '任务执行结果（JSONB）';
COMMENT ON COLUMN tsk_tasks.max_retries IS '最大重试次数';
COMMENT ON COLUMN tsk_tasks.retry_count IS '已重试次数';
COMMENT ON COLUMN tsk_tasks.started_at IS '开始执行时间';
COMMENT ON COLUMN tsk_tasks.finished_at IS '执行完成时间';
COMMENT ON COLUMN tsk_tasks.scheduled_at IS '计划执行时间（用于定时任务）';
COMMENT ON COLUMN tsk_tasks.error_message IS '失败时的错误信息';
COMMENT ON COLUMN tsk_tasks.created_at IS '创建时间';
COMMENT ON COLUMN tsk_tasks.updated_at IS '更新时间';
COMMENT ON TABLE opn_webhook_configs IS 'Webhook 配置';
COMMENT ON COLUMN opn_webhook_configs.id IS '主键ID';
COMMENT ON COLUMN opn_webhook_configs.tenant_id IS '所属租户ID';
COMMENT ON COLUMN opn_webhook_configs.name IS '配置名称';
COMMENT ON COLUMN opn_webhook_configs.url IS '回调地址（必须 HTTPS）';
COMMENT ON COLUMN opn_webhook_configs.secret_key IS 'HMAC-SHA256 签名密钥';
COMMENT ON COLUMN opn_webhook_configs.events IS '订阅的事件类型列表';
COMMENT ON COLUMN opn_webhook_configs.is_active IS '是否启用';
COMMENT ON COLUMN opn_webhook_configs.retry_policy IS '重试策略（JSON）';
COMMENT ON COLUMN opn_webhook_configs.consecutive_failures IS '连续失败次数';
COMMENT ON COLUMN opn_webhook_configs.max_consecutive_failures IS '最大连续失败次数（超过后自动禁用）';
COMMENT ON COLUMN opn_webhook_configs.last_delivery_at IS '最后投递时间';
COMMENT ON COLUMN opn_webhook_configs.created_at IS '创建时间';
COMMENT ON COLUMN opn_webhook_configs.updated_at IS '更新时间';
COMMENT ON TABLE opn_webhook_delivery_logs IS 'Webhook 投递日志';
COMMENT ON COLUMN opn_webhook_delivery_logs.id IS '主键ID';
COMMENT ON COLUMN opn_webhook_delivery_logs.tenant_id IS '所属租户ID';
COMMENT ON COLUMN opn_webhook_delivery_logs.webhook_config_id IS 'Webhook 配置ID';
COMMENT ON COLUMN opn_webhook_delivery_logs.event_id IS '关联的事件ID';
COMMENT ON COLUMN opn_webhook_delivery_logs.attempt IS '第几次尝试';
COMMENT ON COLUMN opn_webhook_delivery_logs.request_url IS '请求 URL';
COMMENT ON COLUMN opn_webhook_delivery_logs.request_headers IS '请求头（JSON）';
COMMENT ON COLUMN opn_webhook_delivery_logs.response_status IS 'HTTP 响应状态码';
COMMENT ON COLUMN opn_webhook_delivery_logs.response_body IS '响应体（截断到 2000 字符）';
COMMENT ON COLUMN opn_webhook_delivery_logs.response_time_ms IS '响应时间（毫秒）';
COMMENT ON COLUMN opn_webhook_delivery_logs.error_message IS '错误信息';
COMMENT ON COLUMN opn_webhook_delivery_logs.created_at IS '投递时间';
COMMENT ON TABLE opn_webhook_events IS 'Webhook 事件';
COMMENT ON COLUMN opn_webhook_events.id IS '主键ID';
COMMENT ON COLUMN opn_webhook_events.tenant_id IS '所属租户ID';
COMMENT ON COLUMN opn_webhook_events.webhook_config_id IS '关联的 Webhook 配置ID';
COMMENT ON COLUMN opn_webhook_events.event_id IS '事件唯一标识';
COMMENT ON COLUMN opn_webhook_events.event_type IS '事件类型';
COMMENT ON COLUMN opn_webhook_events.payload IS '事件载荷（JSON）';
COMMENT ON COLUMN opn_webhook_events.status IS '状态：pending / delivered / failed';
COMMENT ON COLUMN opn_webhook_events.attempts IS '已尝试次数';
COMMENT ON COLUMN opn_webhook_events.next_retry_at IS '下次重试时间';
COMMENT ON COLUMN opn_webhook_events.created_at IS '创建时间';
COMMENT ON COLUMN opn_webhook_events.updated_at IS '更新时间';
COMMENT ON TABLE bil_usage_logs IS 'API 调用日志（按月分区，高吞吐追加写表）';
COMMENT ON COLUMN bil_usage_logs.id IS '主键ID';
COMMENT ON COLUMN bil_usage_logs.tenant_id IS '租户ID';
COMMENT ON COLUMN bil_usage_logs.user_id IS '用户ID';
COMMENT ON COLUMN bil_usage_logs.api_key_id IS '使用的 API Key ID';
COMMENT ON COLUMN bil_usage_logs.channel_id IS '使用的渠道ID';
COMMENT ON COLUMN bil_usage_logs.model_name IS '调用的模型名';
COMMENT ON COLUMN bil_usage_logs.request_id IS '请求唯一ID';
COMMENT ON COLUMN bil_usage_logs.relay_mode IS '代理模式：chat_completions / embeddings / images_generations 等';
COMMENT ON COLUMN bil_usage_logs.input_tokens IS '输入 token 数';
COMMENT ON COLUMN bil_usage_logs.output_tokens IS '输出 token 数';
COMMENT ON COLUMN bil_usage_logs.total_cost IS '本次调用费用';
COMMENT ON COLUMN bil_usage_logs.currency IS '货币（USD）';
COMMENT ON COLUMN bil_usage_logs.latency_ms IS '请求延迟（毫秒）';
COMMENT ON COLUMN bil_usage_logs.status IS '状态：success（成功）/ error（错误）/ timeout（超时）/ cancelled（取消）';
COMMENT ON COLUMN bil_usage_logs.error_message IS '错误信息（成功时为 NULL）';
COMMENT ON COLUMN bil_usage_logs.client_ip IS '客户端 IP';
COMMENT ON COLUMN bil_usage_logs.created_at IS '创建时间';
COMMENT ON COLUMN bil_usage_logs.updated_at IS '更新时间';
COMMENT ON COLUMN bil_usage_logs.cache_creation_tokens IS '缓存创建 token 数 (Claude)';
COMMENT ON COLUMN bil_usage_logs.cache_read_tokens IS '缓存读取 token 数 (Claude/OpenAI)';
COMMENT ON COLUMN bil_usage_logs.input_cost IS '输入 token 费用';
COMMENT ON COLUMN bil_usage_logs.output_cost IS '输出 token 费用';
COMMENT ON COLUMN bil_usage_logs.cache_creation_cost IS '缓存创建费用';
COMMENT ON COLUMN bil_usage_logs.cache_read_cost IS '缓存读取费用';
COMMENT ON COLUMN bil_usage_logs.actual_cost IS '实际扣除费用（含折扣后）';
COMMENT ON COLUMN bil_usage_logs.requested_model IS '用户请求的模型名';
COMMENT ON COLUMN bil_usage_logs.upstream_model IS '上游实际模型名（模型映射后）';
COMMENT ON COLUMN bil_usage_logs.request_type IS '请求类型: 1=sync, 2=stream, 3=websocket';
COMMENT ON COLUMN bil_usage_logs.user_agent IS '客户端 User-Agent';
COMMENT ON COLUMN bil_usage_logs.first_token_ms IS '首 token 延迟（毫秒）';
COMMENT ON COLUMN bil_usage_logs.service_tier IS '服务等级 (default/flex等)';
COMMENT ON COLUMN bil_usage_logs.reasoning_effort IS '推理力度 (low/medium/high)';
COMMENT ON COLUMN bil_usage_logs.channel_name IS '渠道名称';
COMMENT ON COLUMN bil_usage_logs.channel_type IS '渠道类型 (ProviderType)';
COMMENT ON COLUMN bil_usage_logs.billing_mode IS '计费模式 (token/per_request/tiered)';
COMMENT ON COLUMN bil_usage_logs.billing_source IS '定价来源 (base/tenant/custom)';
COMMENT ON COLUMN bil_usage_logs.rate_multiplier IS '费率倍率/折扣';
COMMENT ON COLUMN bil_usage_logs.pre_deduct_amount IS '预扣金额';
COMMENT ON COLUMN bil_usage_logs.refund_amount IS '退款金额';
COMMENT ON COLUMN bil_usage_logs.supplement_amount IS '补扣金额';
COMMENT ON COLUMN bil_usage_logs.image_count IS '生成图片数量';
COMMENT ON COLUMN bil_usage_logs.image_size IS '图片尺寸';
COMMENT ON COLUMN bil_usage_logs.stream_end_reason IS '流结束原因 (done/timeout/client_gone/error/panic)';
COMMENT ON COLUMN bil_usage_logs.retry_index IS '重试次数（0=首次成功）';
COMMENT ON COLUMN bil_usage_logs.billing_summary IS '计费快照文本（人类可读的计费过程描述）';
COMMENT ON COLUMN bil_usage_logs.cache_creation_5m_tokens IS 'Claude 5分钟缓存创建 token 数';
COMMENT ON COLUMN bil_usage_logs.cache_creation_1h_tokens IS 'Claude 1小时缓存创建 token 数';
COMMENT ON COLUMN bil_usage_logs.audio_input_tokens IS '音频输入 token 数';
COMMENT ON COLUMN bil_usage_logs.audio_output_tokens IS '音频输出 token 数';
COMMENT ON COLUMN bil_usage_logs.image_output_tokens IS '图像输出 token 数（DALL-E 等）';
COMMENT ON COLUMN bil_usage_logs.reasoning_tokens IS '推理 token 数（O1/o3 等）';
COMMENT ON COLUMN bil_usage_logs.account_cost IS '上游账户成本（用于利润分析）';
COMMENT ON COLUMN bil_usage_logs.inbound_endpoint IS '客户端请求路径（如 /v1/chat/completions）';
COMMENT ON COLUMN bil_usage_logs.upstream_endpoint IS '上游实际请求路径';
COMMENT ON COLUMN bil_usage_logs.billing_snapshot IS '完整计费计算过程快照（JSONB）';
COMMENT ON TABLE ops_system_metrics IS '系统指标时序数据（按月分区）';
COMMENT ON COLUMN ops_system_metrics.id IS '主键ID';
COMMENT ON COLUMN ops_system_metrics.metric_type IS '指标类型：cpu/memory/disk/network/runtime/db_pool/redis_pool';
COMMENT ON COLUMN ops_system_metrics.metric_data IS '指标数据（JSONB，结构因类型而异）';
COMMENT ON COLUMN ops_system_metrics.collected_at IS '采集时间';

COMMENT ON COLUMN aud_request_logs.project_id IS '关联项目ID（通过API Key关联，NULL表示个人密钥无项目）';
COMMENT ON COLUMN aud_request_logs.first_token_ms IS '首个 Token 出现的用时（毫秒），仅流式请求有值';
COMMENT ON COLUMN aud_request_logs.request_headers IS '请求头信息（仅审计级别为 all 时记录，管理后台调试用）';
COMMENT ON COLUMN aud_request_logs.response_headers IS '响应头信息（仅审计级别为 all 时记录，管理后台调试用）';
COMMENT ON COLUMN aud_request_logs.forwarding_trace IS '请求转发路径追踪（仅管理员可见）';
COMMENT ON COLUMN bil_usage_logs.project_id IS '关联项目ID（通过API Key关联，NULL表示个人密钥无项目）';
COMMENT ON COLUMN tnt_users.quota_type IS '额度限制类型：none（不限）/ total（总额）/ periodic（周期性）';
COMMENT ON COLUMN tnt_users.quota_limit IS '额度上限（USD），quota_type 为 none 时忽略';
COMMENT ON COLUMN tnt_users.quota_used IS '已使用额度（USD）';
COMMENT ON COLUMN tnt_users.quota_period IS '周期类型：day / week / month（仅 periodic 时有效）';
COMMENT ON COLUMN tnt_users.quota_reset_at IS '上次额度重置时间（懒重置用）';
COMMENT ON COLUMN tnt_invitations.max_uses IS '最大使用次数，0表示不限';
COMMENT ON COLUMN tnt_invitations.use_count IS '已使用次数';
COMMENT ON COLUMN ntf_messages.target_roles IS '目标角色（NULL=全部角色，逗号分隔如 owner,admin 表示仅限这些角色）';
COMMENT ON COLUMN chn_channel_keys.key_type IS 'Key 类型：apikey（传统静态密钥）/ oauth（OAuth 令牌）';
COMMENT ON COLUMN chn_channel_keys.token_expires_at IS 'OAuth access_token 过期时间（仅 key_type=oauth 时有值）';

COMMENT ON TABLE ord_redemption_usages IS '兑换码使用记录';
COMMENT ON COLUMN ord_redemption_usages.id IS '主键ID';
COMMENT ON COLUMN ord_redemption_usages.redemption_id IS '关联兑换码ID';
COMMENT ON COLUMN ord_redemption_usages.tenant_id IS '使用兑换码的租户ID';
COMMENT ON COLUMN ord_redemption_usages.user_id IS '执行兑换操作的用户ID';
COMMENT ON COLUMN ord_redemption_usages.type IS '兑换类型：quota / plan / duration';
COMMENT ON COLUMN ord_redemption_usages.value IS '兑换面值（quota类型为金额，plan/duration为0）';
COMMENT ON COLUMN ord_redemption_usages.transaction_id IS '关联的交易流水ID（仅quota类型有值）';
COMMENT ON COLUMN ord_redemption_usages.created_at IS '创建时间';
COMMENT ON COLUMN ord_redemption_usages.updated_at IS '更新时间';
COMMENT ON TABLE sys_error_logs IS '系统错误日志';
COMMENT ON COLUMN sys_error_logs.id IS '主键';
COMMENT ON COLUMN sys_error_logs.request_id IS '请求ID，用于链路追踪';
COMMENT ON COLUMN sys_error_logs.error_code IS '错误码（HTTP状态码或GoFrame错误码）';
COMMENT ON COLUMN sys_error_logs.error_message IS '错误消息';
COMMENT ON COLUMN sys_error_logs.stack_trace IS '错误堆栈';
COMMENT ON COLUMN sys_error_logs.http_method IS 'HTTP请求方法';
COMMENT ON COLUMN sys_error_logs.request_path IS '请求路径';
COMMENT ON COLUMN sys_error_logs.request_body IS '请求体摘要（截断）';
COMMENT ON COLUMN sys_error_logs.source IS '错误来源：api/panic/cron/background';
COMMENT ON COLUMN sys_error_logs.resolved IS '是否已处理';
COMMENT ON COLUMN sys_error_logs.resolved_by IS '处理人ID';
COMMENT ON COLUMN sys_error_logs.resolved_at IS '处理时间';
COMMENT ON COLUMN sys_error_logs.created_at IS '创建时间';
COMMENT ON TABLE sys_cron_job_executions IS '定时任务执行记录';
COMMENT ON COLUMN sys_cron_job_executions.id IS '主键ID';
COMMENT ON COLUMN sys_cron_job_executions.job_name IS '任务名称（代码中定义的唯一标识）';
COMMENT ON COLUMN sys_cron_job_executions.status IS '执行状态：succeeded/failed';
COMMENT ON COLUMN sys_cron_job_executions.started_at IS '开始执行时间';
COMMENT ON COLUMN sys_cron_job_executions.finished_at IS '执行完成时间';
COMMENT ON COLUMN sys_cron_job_executions.duration_ms IS '执行耗时（毫秒）';
COMMENT ON COLUMN sys_cron_job_executions.error_message IS '错误消息（仅失败时有值）';
COMMENT ON COLUMN sys_cron_job_executions.triggered_by IS '触发方式：auto（自动调度）/ manual（手动触发）';
COMMENT ON COLUMN sys_cron_job_executions.created_at IS '创建时间';
COMMENT ON COLUMN sys_cron_job_executions.updated_at IS '更新时间';

-- ============================================================
-- Seed Data
-- ============================================================

INSERT INTO ntf_templates (code, channel, subject, body_template, variables, status)
VALUES (
    'balance_warning',
    'in_app',
    '余额预警通知',
    '<p>您的账户可用余额为 <strong>${available}</strong> USD，已低于预警线 <strong>${threshold}</strong> USD。</p><p>请及时充值以避免服务中断。</p>',
    '["available", "threshold"]'::jsonb,
    'active'
);


-- ============================================================
-- Plugin System
-- ============================================================

CREATE TABLE sys_plugins (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(64)  NOT NULL,
    label       VARCHAR(128) NOT NULL,
    version     VARCHAR(32)  NOT NULL,
    status      VARCHAR(16)  NOT NULL DEFAULT 'registered',
    category    VARCHAR(32)  NOT NULL DEFAULT 'extension',
    config      JSONB        DEFAULT '{}'::jsonb,
    error_msg   TEXT         DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uk_sys_plugins_name UNIQUE (name)
);

COMMENT ON TABLE sys_plugins IS '系统插件注册表';
COMMENT ON COLUMN sys_plugins.name IS '插件唯一标识，如 email-report';
COMMENT ON COLUMN sys_plugins.label IS '显示名称';
COMMENT ON COLUMN sys_plugins.version IS '当前安装版本';
COMMENT ON COLUMN sys_plugins.status IS '状态：registered=已注册, installed=已安装, enabled=已启用, disabled=已禁用, error=异常';
COMMENT ON COLUMN sys_plugins.category IS '分类：relay=代理扩展, middleware=中间件, billing=计费, notification=通知, extension=通用扩展';
COMMENT ON COLUMN sys_plugins.config IS '插件全局配置（JSON）';
COMMENT ON COLUMN sys_plugins.error_msg IS '异常信息';

CREATE TABLE tnt_tenant_plugins (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT       NOT NULL,
    plugin_name VARCHAR(64)  NOT NULL,
    enabled     BOOLEAN      NOT NULL DEFAULT FALSE,
    config      JSONB        DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uk_tnt_tenant_plugins UNIQUE (tenant_id, plugin_name)
);

COMMENT ON TABLE tnt_tenant_plugins IS '租户级插件启用/配置';
COMMENT ON COLUMN tnt_tenant_plugins.plugin_name IS '插件标识';
COMMENT ON COLUMN tnt_tenant_plugins.enabled IS '是否启用';
COMMENT ON COLUMN tnt_tenant_plugins.config IS '租户级配置覆盖（JSON），优先级高于全局配置';

-- +goose Down
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
