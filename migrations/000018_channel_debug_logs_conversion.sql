-- +goose Up
-- 渠道调试日志：新增协议转换信息列。
-- 记录客户端协议 →（relaykit 转换链）→ 上游协议及桥接方式（responses_api 桥接 / responses 直连 / 直传），
-- 供排查协议转换类问题（字段丢失/格式不符）时与四段报文对照。

ALTER TABLE chn_debug_logs ADD COLUMN IF NOT EXISTS conversion JSONB;

COMMENT ON COLUMN chn_debug_logs.conversion IS '协议转换信息 JSON：client_format 客户端协议 / upstream_format 上游协议 / chain 请求侧转换链 / bridge 桥接方式（responses_api|responses_direct|pass_through，空=常规转换）';

-- +goose Down
ALTER TABLE chn_debug_logs DROP COLUMN IF EXISTS conversion;
