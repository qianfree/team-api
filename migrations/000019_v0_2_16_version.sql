-- +goose Up
-- 回填 bil_usage_logs.input_tokens 的「含缓存总输入」口径（恢复被丢失的 974633a8 配套回填）。
--
-- 背景：974633a8 将写侧 input_tokens 统一为「含缓存总输入」（Claude 口径入库时补加
-- cache_read + cache_creation），缓存命中率公式随之改为 cache_read / input_tokens。
-- 当时配套的历史数据回填 UPDATE 在 3676cd4d 中被误删、d01dbc79 合并迁移记录时彻底丢失，
-- 导致存量 Claude 渠道行（channel_type=2）仍是「不含缓存」旧口径：
-- 这些行 cache_read_tokens 可大于 input_tokens，个人工作台按月聚合后缓存命中率超过 100%。
--
-- 本迁移将 8/17 之前的存量 Claude 渠道行补加缓存 token，使全表口径一致：
--   input_tokens = 普通输入 + cache_read + cache_creation
-- OpenAI / Gemini 渠道行的 prompt 原生含缓存（cached 为其子集），不受影响；
-- 经协议转换已按含缓存口径入库的行（如 openai 适配器解析 Claude 风格 usage，channel_type≠2）同样不触碰。
--
-- 注意：本迁移假设 channel_type=2 的历史行均为「不含缓存」口径（与 974633a8 原迁移一致）。
-- 若曾在 974633a8 合入后、3676cd4d 之前（2026-08-17 10:01 ~ 11:25）对同一库执行过 goose up，
-- 原回填已生效，禁止重复执行本迁移（会造成缓存 token 双重累加）。
--
-- 回填后需重跑用量日聚合刷新 bil_usage_daily（ON CONFLICT 幂等，可重复执行）：
--   team-api aggregate-usage --from 2026-08-01 --to <今天>

-- +goose StatementBegin
UPDATE bil_usage_logs
SET input_tokens = input_tokens + COALESCE(cache_read_tokens, 0) + COALESCE(cache_creation_tokens, 0)
WHERE channel_type = 2
  AND (COALESCE(cache_read_tokens, 0) + COALESCE(cache_creation_tokens, 0)) > 0;
-- +goose StatementEnd

-- +goose Down
-- 逆转回填：Claude 渠道行恢复「不含缓存」口径（OpenAI/Gemini 行本就含缓存，不受影响）。
-- Down 后需再次执行 aggregate-usage 刷新 bil_usage_daily。

-- +goose StatementBegin
UPDATE bil_usage_logs
SET input_tokens = input_tokens - COALESCE(cache_read_tokens, 0) - COALESCE(cache_creation_tokens, 0)
WHERE channel_type = 2
  AND (COALESCE(cache_read_tokens, 0) + COALESCE(cache_creation_tokens, 0)) > 0;
-- +goose StatementEnd
