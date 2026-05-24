-- +goose Up
-- 简化套餐模型访问：用 allowed_models TEXT[] 替代 model_group_id，移除 priority_access、advanced_log、support_level

-- 1. 添加 allowed_models 列（TEXT 数组，空数组 = 全部模型）
ALTER TABLE pln_plans ADD COLUMN allowed_models TEXT[] DEFAULT '{}';
COMMENT ON COLUMN pln_plans.allowed_models IS '允许使用的模型列表，空数组=全部模型';

-- 2. 迁移数据：将 model_group_id 引用的模型组内容写入 allowed_models
UPDATE pln_plans p
SET allowed_models = COALESCE(
    (SELECT ARRAY_AGG(mgi.model_name)
     FROM mdl_model_group_items mgi
     WHERE mgi.group_id = p.model_group_id),
    '{}'
);

-- 3. 删除旧列
ALTER TABLE pln_plans DROP COLUMN model_group_id;
ALTER TABLE pln_plans DROP COLUMN priority_access;
ALTER TABLE pln_plans DROP COLUMN advanced_log;
ALTER TABLE pln_plans DROP COLUMN support_level;

-- +goose Down
-- 回滚：恢复删除的列，将 allowed_models 转回 model_group_id

-- 恢复列
ALTER TABLE pln_plans ADD COLUMN model_group_id BIGINT DEFAULT 0;
COMMENT ON COLUMN pln_plans.model_group_id IS '模型组ID，0=全部模型';

ALTER TABLE pln_plans ADD COLUMN priority_access BOOLEAN DEFAULT false NOT NULL;
COMMENT ON COLUMN pln_plans.priority_access IS '是否包含优先通道访问';

ALTER TABLE pln_plans ADD COLUMN advanced_log BOOLEAN DEFAULT false NOT NULL;
COMMENT ON COLUMN pln_plans.advanced_log IS '是否包含高级日志';

ALTER TABLE pln_plans ADD COLUMN support_level VARCHAR(20) DEFAULT 'standard' NOT NULL;
COMMENT ON COLUMN pln_plans.support_level IS '支持级别：standard/premium/dedicated';

-- 删除新列
ALTER TABLE pln_plans DROP COLUMN allowed_models;
