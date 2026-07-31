-- +goose Up
-- 渠道调度重构收尾（阶段 5）：
-- chn_channel_affinities 表废弃——会话绑定已全部迁移至 Redis（dispatch:v1:bind:*），
-- 本表自旧调度（relay:affinity:v2 之前的 DB 方案）起已无代码读写，保留仅供历史查询。
-- 按计划标记废弃不删除，后续版本确认无审计需求后再物理清理。
COMMENT ON TABLE chn_channel_affinities IS '【已废弃 2026-07】渠道亲和性缓存。会话绑定已迁移至 Redis（dispatch:v1:bind:*），本表无代码读写，仅保留历史数据';

-- +goose Down
COMMENT ON TABLE chn_channel_affinities IS '渠道亲和性缓存（用户+模型→渠道映射，TTL 1800s）';
