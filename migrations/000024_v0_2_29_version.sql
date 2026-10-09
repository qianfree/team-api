-- +goose Up
-- ============================================================
-- 兑换码每租户限兑一次
--
-- ord_redemptions 支持 max_uses > 1 的多用途码后，需保证同一
-- 租户对同一兑换码只能兑换一次（权益按租户归属，按租户去重），
-- 防止单一租户反复兑换耗尽整码额度。
-- ============================================================

-- 防御性检查：API 层 max_uses>1 曾可用，存量理论上有重复；
-- 存在重复时直接失败，提示人工清理后再执行迁移
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM ord_redemption_usages
        GROUP BY redemption_id, tenant_id
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'ord_redemption_usages 存在重复 (redemption_id, tenant_id) 记录，需人工清理后再执行迁移';
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE ord_redemption_usages
    ADD CONSTRAINT uk_ord_redemption_usages_redemption_tenant UNIQUE (redemption_id, tenant_id);

-- 唯一约束索引的左前缀已覆盖 redemption_id 单列查询，删除冗余索引
DROP INDEX IF EXISTS idx_ord_redemption_usages_redemption;

-- +goose Down
ALTER TABLE ord_redemption_usages
    DROP CONSTRAINT IF EXISTS uk_ord_redemption_usages_redemption_tenant;

CREATE INDEX IF NOT EXISTS idx_ord_redemption_usages_redemption
    ON ord_redemption_usages USING btree (redemption_id);
