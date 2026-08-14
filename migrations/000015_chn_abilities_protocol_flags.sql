-- +goose Up
-- 渠道×模型级协议能力标记（配合 Responses 端点支持）：
--   supports_responses：模型在该渠道支持 OpenAI Responses 协议（/v1/responses）。
--     responses 入站请求命中时原样直连上游 /v1/responses；调度时对 responses 入站软偏好此类渠道。
--   chat_via_responses：上游仅有 Responses 协议（responses-only，如 Codex 类中转），
--     chat 入站请求经 chat→Responses 桥接发送到 /v1/responses。
-- 两项均为渠道×模型粒度（非渠道级）：同一渠道的不同模型协议支持可能不同。

ALTER TABLE chn_abilities ADD COLUMN supports_responses BOOLEAN DEFAULT false NOT NULL;
COMMENT ON COLUMN chn_abilities.supports_responses IS '模型在该渠道支持 OpenAI Responses 协议（/v1/responses），responses 入站直连转发并获调度软偏好';

ALTER TABLE chn_abilities ADD COLUMN chat_via_responses BOOLEAN DEFAULT false NOT NULL;
COMMENT ON COLUMN chn_abilities.chat_via_responses IS '上游仅有 Responses 协议（responses-only 上游），chat 入站经桥接转换后发送 /v1/responses';

-- +goose Down
ALTER TABLE chn_abilities DROP COLUMN IF EXISTS supports_responses;
ALTER TABLE chn_abilities DROP COLUMN IF EXISTS chat_via_responses;
