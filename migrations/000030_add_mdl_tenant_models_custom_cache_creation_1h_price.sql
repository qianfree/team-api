-- +goose Up
-- 租户模型自定义定价增加长时缓存（长 TTL 桶）写入价覆盖字段。
--
-- 背景：平台级定价早已支持 cache_creation_1h_price（长 TTL 桶，Claude 1h /
-- GPT 12h 归一双桶计价），但租户级覆盖只有读/写基准价两列——租户重定创建
-- 基准价时平台显式 1h 价被丢弃，长桶价重置为按新基准 ×1.6 推导
-- （pricing.go 租户覆盖块）；官方长 TTL 写入价为输入价 2~2.5×，推导值
-- 会低估，租户无法精确覆盖长时缓存写入成本。
--
-- 语义：NULL = 未覆盖，沿用现有链路（平台显式 1h 价 → 平台基准 ×1.6 推导）；
-- 正值 = 长桶价锚点整体替换（tiered 模式下作为锚点参与逐档回退）。

ALTER TABLE mdl_tenant_models
    ADD COLUMN IF NOT EXISTS custom_cache_creation_1h_price NUMERIC(20,10);

COMMENT ON COLUMN mdl_tenant_models.custom_cache_creation_1h_price IS '自定义长时缓存创建价格（$/1M token，长 TTL 桶：Claude 1h / GPT 12h 归一），NULL 表示未覆盖（平台显式 1h 价或基准 ×1.6 推导）';

-- +goose Down
ALTER TABLE mdl_tenant_models
    DROP COLUMN IF EXISTS custom_cache_creation_1h_price;
