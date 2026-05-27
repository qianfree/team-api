-- +goose Up
-- 模型分组功能：通过分组批量管理租户可用的模型

-- 模型分组定义
CREATE TABLE mdl_model_groups (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    code        VARCHAR(50) NOT NULL,
    description TEXT,
    status      VARCHAR(20) DEFAULT 'active' NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at  TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_mdl_model_groups_code UNIQUE (code)
);

COMMENT ON TABLE mdl_model_groups IS '模型分组定义';
COMMENT ON COLUMN mdl_model_groups.id IS '主键ID';
COMMENT ON COLUMN mdl_model_groups.name IS '分组名称（如"全量模型"、"基础对话"）';
COMMENT ON COLUMN mdl_model_groups.code IS '分组唯一标识（如 full_access、basic_chat）';
COMMENT ON COLUMN mdl_model_groups.description IS '分组描述';
COMMENT ON COLUMN mdl_model_groups.status IS '状态：active（启用）/ disabled（禁用）';
COMMENT ON COLUMN mdl_model_groups.created_at IS '创建时间';
COMMENT ON COLUMN mdl_model_groups.updated_at IS '更新时间';

-- 分组-模型关联（多对多）
CREATE TABLE mdl_group_models (
    id         BIGSERIAL PRIMARY KEY,
    group_id   BIGINT NOT NULL,
    model_id   BIGINT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_mdl_group_models UNIQUE (group_id, model_id)
);

CREATE INDEX idx_mdl_group_models_model_id ON mdl_group_models (model_id);

COMMENT ON TABLE mdl_group_models IS '分组-模型关联';
COMMENT ON COLUMN mdl_group_models.id IS '主键ID';
COMMENT ON COLUMN mdl_group_models.group_id IS '分组ID（关联 mdl_model_groups.id）';
COMMENT ON COLUMN mdl_group_models.model_id IS '模型ID（关联 mdl_models.id）';
COMMENT ON COLUMN mdl_group_models.created_at IS '创建时间';
COMMENT ON COLUMN mdl_group_models.updated_at IS '更新时间';

-- 租户-分组关联（多对多）
CREATE TABLE mdl_tenant_groups (
    id         BIGSERIAL PRIMARY KEY,
    tenant_id  BIGINT NOT NULL,
    group_id   BIGINT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT uk_mdl_tenant_groups UNIQUE (tenant_id, group_id)
);

CREATE INDEX idx_mdl_tenant_groups_tenant_id ON mdl_tenant_groups (tenant_id);

COMMENT ON TABLE mdl_tenant_groups IS '租户-分组关联';
COMMENT ON COLUMN mdl_tenant_groups.id IS '主键ID';
COMMENT ON COLUMN mdl_tenant_groups.tenant_id IS '租户ID（关联 tnt_tenants.id）';
COMMENT ON COLUMN mdl_tenant_groups.group_id IS '分组ID（关联 mdl_model_groups.id）';
COMMENT ON COLUMN mdl_tenant_groups.created_at IS '创建时间';
COMMENT ON COLUMN mdl_tenant_groups.updated_at IS '更新时间';

-- +goose Down
DROP TABLE IF EXISTS mdl_tenant_groups;
DROP TABLE IF EXISTS mdl_group_models;
DROP TABLE IF EXISTS mdl_model_groups;
