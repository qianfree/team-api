-- +goose Up
-- ============================================================================
-- JS 任务插件用量键统一为宿主固定词汇
--
-- 背景：JS 任务插件契约移除 manifest.usageSchema（插件自定义用量键），改为宿主
-- 固定词汇：output_seconds / resolution / output_count / input_image_count /
-- input_video_seconds / input_audio_seconds / input_tokens / output_tokens /
-- generate_audio。计费公式经 u("键") 引用用量，存量公式里的旧键需同步改名，
-- 否则 u() 取 0 静默错价。
--
-- 旧键 → 新键（内置插件 alibaba/doubao/hailuo/golden 与常见上传插件的旧口径）：
--   u("seconds")      → u("output_seconds")
--   u("input_images") → u("input_image_count")
--   u("image_count")  → u("output_count")
--   u("clips")        → u("output_count")
--   u("tokens")       → u("output_tokens")
-- 无对应物的旧键（如 doubao 的 video_input_ratio、golden 的 tier）不替换：
-- 引用它们的公式保存时烟测会拒绝，需运营按新词汇改写（例：
-- u("input_video_seconds") > 0 ? 系数A : 系数B）。
--
-- 作用范围：mdl_pricing.pricing（formula/formula_base 键）与
-- mdl_tenant_models.custom_pricing（租户覆盖 formula 键）。按 JSONB 文本替换，
-- 只匹配带引号的完整 u("键") 字面量（JSON 内引号转义为 \"），幂等可重放。
-- ============================================================================

UPDATE mdl_pricing
SET pricing = replace(replace(replace(replace(replace(pricing::text,
        'u(\"seconds\")',      'u(\"output_seconds\")'),
        'u(\"input_images\")', 'u(\"input_image_count\")'),
        'u(\"image_count\")',  'u(\"output_count\")'),
        'u(\"clips\")',        'u(\"output_count\")'),
        'u(\"tokens\")',       'u(\"output_tokens\")')::jsonb
WHERE pricing::text LIKE '%u(\\"%';

UPDATE mdl_tenant_models
SET custom_pricing = replace(replace(replace(replace(replace(custom_pricing::text,
        'u(\"seconds\")',      'u(\"output_seconds\")'),
        'u(\"input_images\")', 'u(\"input_image_count\")'),
        'u(\"image_count\")',  'u(\"output_count\")'),
        'u(\"clips\")',        'u(\"output_count\")'),
        'u(\"tokens\")',       'u(\"output_tokens\")')::jsonb
WHERE custom_pricing IS NOT NULL AND custom_pricing::text LIKE '%u(\\"%';

-- +goose Down
-- 反向替换（output_count 来源不唯一，统一回退为 image_count；clips 口径无法区分不再还原）
UPDATE mdl_pricing
SET pricing = replace(replace(replace(replace(pricing::text,
        'u(\"output_seconds\")',    'u(\"seconds\")'),
        'u(\"input_image_count\")', 'u(\"input_images\")'),
        'u(\"output_count\")',      'u(\"image_count\")'),
        'u(\"output_tokens\")',     'u(\"tokens\")')::jsonb
WHERE pricing::text LIKE '%u(\\"%';

UPDATE mdl_tenant_models
SET custom_pricing = replace(replace(replace(replace(custom_pricing::text,
        'u(\"output_seconds\")',    'u(\"seconds\")'),
        'u(\"input_image_count\")', 'u(\"input_images\")'),
        'u(\"output_count\")',      'u(\"image_count\")'),
        'u(\"output_tokens\")',     'u(\"tokens\")')::jsonb
WHERE custom_pricing IS NOT NULL AND custom_pricing::text LIKE '%u(\\"%';
