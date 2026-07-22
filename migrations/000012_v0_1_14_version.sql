-- +goose Up

-- A3 修复：结算幂等 —— bil_records.request_id 由普通 btree 索引升级为唯一索引。
-- 原 idx_bil_records_request 无唯一性保护，同一 request_id 重复结算会写入多条计费记录
-- 并重复扣款。唯一索引作为 DB 层最终兜底：重复结算的第二次 INSERT 触发 23505 唯一冲突，
-- 结算事务整体回滚（钱包扣款一并撤销），应用层据此识别为幂等空操作。
--
-- 前置约束：若历史数据因该 bug 已产生重复 request_id，本迁移会失败（这是有意为之——
-- 金融记录不做静默删除，应由运维先人工核对重复计费记录再重跑）。bil_prededuct_tracks
-- 早已对 request_id 施加 UNIQUE，证明「一次请求一条记录」本就是系统不变式。
DROP INDEX IF EXISTS idx_bil_records_request;
CREATE UNIQUE INDEX IF NOT EXISTS uk_bil_records_request ON bil_records USING btree (request_id);

-- +goose Down

-- 还原为非唯一 btree 索引
DROP INDEX IF EXISTS uk_bil_records_request;
CREATE INDEX IF NOT EXISTS idx_bil_records_request ON bil_records USING btree (request_id);
