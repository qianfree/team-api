-- +goose Up
-- 模型分组增加"默认分组"标记，标记为默认的分组在新租户注册时自动关联

ALTER TABLE mdl_model_groups ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT FALSE;
COMMENT ON COLUMN mdl_model_groups.is_default IS '是否为新租户默认模型组，注册时自动关联';

-- +goose Down

ALTER TABLE mdl_model_groups DROP COLUMN IF EXISTS is_default;
