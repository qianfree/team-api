-- +goose Up
-- ============================================================
-- js_task_artifacts 补产物声明原样列 + 「不转存」状态
--
--   · source_path：声明期的 taskData 取值路径（不可变）。此前复用 source_url 存路径，
--     url 型下载前会被改写为真实 URL——下载/上传失败的行被 sweep 重建时把 URL 当路径
--     取值，重试必然失败；统一内容链接的回源代理也需要稳定的路径
--   · source_auth：声明的下载鉴权形态（channel-bearer），此前未持久化，sweep 重建丢失
--   · status 新增 direct：插件关闭「产物转存到平台对象存储」时，产物只登记声明、
--     不进转存队列，统一内容链接按声明实时回源代理
-- ============================================================

-- +goose StatementBegin
ALTER TABLE js_task_artifacts ADD COLUMN IF NOT EXISTS source_path TEXT NOT NULL DEFAULT '';
ALTER TABLE js_task_artifacts ADD COLUMN IF NOT EXISTS source_auth VARCHAR(32) NOT NULL DEFAULT '';

-- 存量回填：尚未被改写为 URL 的 source_url 即声明路径（可重复执行：仅回填空 source_path）
UPDATE js_task_artifacts
SET source_path = source_url,
    updated_at  = now()
WHERE source_path = ''
  AND source_url IS NOT NULL
  AND source_url <> ''
  AND source_url NOT LIKE 'http://%'
  AND source_url NOT LIKE 'https://%'
  AND source_url NOT LIKE '/%';
-- +goose StatementEnd

COMMENT ON COLUMN js_task_artifacts.source_path IS '产物声明的 taskData 取值路径（声明期写入，不随转存改写；回源代理与 sweep 重建的依据）';
COMMENT ON COLUMN js_task_artifacts.source_auth IS '产物声明的下载鉴权形态：空=裸下载 / channel-bearer=注入渠道密钥';
COMMENT ON COLUMN js_task_artifacts.source_url IS '实际下载 URL（url 型转存时写入；存量行声明期曾存取值路径）';
COMMENT ON COLUMN js_task_artifacts.status IS 'pending=待转存 running=进行中 stored=完成 failed=失败（retry_count<5 由 sweep 重试） direct=不转存（插件关闭转存，回源代理）';

-- +goose Down
-- +goose StatementBegin
ALTER TABLE js_task_artifacts DROP COLUMN IF EXISTS source_path;
ALTER TABLE js_task_artifacts DROP COLUMN IF EXISTS source_auth;
-- +goose StatementEnd
COMMENT ON COLUMN js_task_artifacts.source_url IS '上游取值路径或下载 URL（声明期存 taskData 取值路径，转存时更新为真实 URL）';
COMMENT ON COLUMN js_task_artifacts.status IS 'pending=待转存 running=进行中 stored=完成 failed=失败（retry_count<5 由 sweep 重试）';
