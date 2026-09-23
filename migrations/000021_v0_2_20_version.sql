-- +goose Up
-- 定价存储 JSON 化 + 特殊计费 + 视频任务软删除 + 租户标签备注/钱包累计消费 + 模型厂商（由原 000021~000027 合并）。
-- 设计文档：docs/模型定价存储JSON化与按秒计费设计.md、docs/租户标签备注与累计消费-执行规划.md
--
-- mdl_pricing 一步收敛为「每模型一行 + pricing JSONB 唯一真相」：
--   * 旧价格列/多行档位回填进 JSON 后删除，model_id 加唯一约束；
--   * 新增 official_pricing 官方参考定价（管理端折扣计算的基准参照，非计费依据）；
--   * billing_mode 值域新增 special（pricing JSONB 顶层 scheme 键声明计费方案，如 custom:minimax-material）；
--   * 配套 bil_usage_logs.duration_seconds 落按秒计费任务时长，不塞 token 字段凑合。
-- 另含四笔独立变更：tsk_model_tasks 软删除、tnt_tenants 标签/备注、bil_wallets 累计消费、mdl_models 厂商字段。
-- ⚠️ 部署顺序强约束：必须先部署识别 special 的新代码，再执行本迁移——
-- 旧代码读到 special 会按未配价 fail-closed 拒绝该模型的同步路径请求，且旧代码依赖的标量价格列在本迁移中被删除。

-- ── 一、tsk_model_tasks：OpenAI Videos DELETE 端点软删除 ──
-- 官方语义为「永久删除已完成/失败视频及其资产」，网关侧保留计费与审计记录，
-- 采用软删除（deleted=true 后客户端查询/下载均按 404 处理）。
-- +goose StatementBegin
ALTER TABLE tsk_model_tasks
	ADD COLUMN deleted BOOLEAN NOT NULL DEFAULT FALSE,
	ADD COLUMN deleted_at TIMESTAMPTZ NULL;
-- +goose StatementEnd

COMMENT ON COLUMN tsk_model_tasks.deleted IS '软删除标记：true 表示已被客户端删除，查询/下载按不存在处理（计费与审计记录保留）';
COMMENT ON COLUMN tsk_model_tasks.deleted_at IS '软删除时间';

-- ── 二、mdl_pricing：收敛为每模型一行，pricing JSONB 唯一真相 ──
-- 保留列：model_id（加 UNIQUE）、billing_mode（可索引筛选）、pricing、official_pricing、
-- price_note / discount_label / price_change_note。

-- 1. 锚点行去重（历史脏数据：个别模型存在多条 min_tokens=0 行）。
--    保留每组 id 最大的一条——与旧读取语义一致（GetModelPriceAt 全行扫描循环覆盖，后行胜出）；
--    去重先于回填，避免被删锚点行作为 min_tokens=0 档位混进聚合出的 tiers 数组、污染首档价格。
-- +goose StatementBegin
DELETE FROM mdl_pricing p
	USING mdl_pricing p2
	WHERE p.model_id = p2.model_id
	  AND p.min_tokens = 0 AND p2.min_tokens = 0
	  AND p.id < p2.id;
-- +goose StatementEnd

-- 2. 新增 pricing（唯一计费真相）与 official_pricing（官方参考定价）两列
-- +goose StatementBegin
ALTER TABLE mdl_pricing
	ADD COLUMN IF NOT EXISTS pricing JSONB,
	ADD COLUMN IF NOT EXISTS official_pricing JSONB;
-- +goose StatementEnd

COMMENT ON COLUMN mdl_pricing.pricing IS '计费详情（JSONB，按 billing_mode 单模式存储）：token={input_price,output_price,cache_read_price,cache_creation_price}；tiered={tiers:[{min_tokens,max_tokens,input_price,output_price}],cache_read_price,cache_creation_price}；per_request={price}；per_second={unit:"second",prices:{分辨率规格:每秒单价,"*":兜底价}}；顶层可含 time_segments（时段定价）与 scheme（特殊计费方案）';

COMMENT ON COLUMN mdl_pricing.official_pricing IS '官方参考定价 JSONB（结构与 pricing 一致 + billing_mode 键，本位币；仅作定价折扣计算基准与几折展示，非计费依据）';

-- 3. 存量回填：仅锚点行（min_tokens=0）写 pricing 列
--    token / per_request：锚点行标量列构造 JSON
--    tiered：全部档位行聚合成 tiers 数组（锚点行承载 cache 价）
--    time_segments：并入 pricing 顶层
-- +goose StatementBegin
WITH tiers AS (
	SELECT model_id,
	       jsonb_agg(jsonb_build_object(
	         'min_tokens', min_tokens, 'max_tokens', max_tokens,
	         'input_price', input_price, 'output_price', output_price
	       ) ORDER BY min_tokens) AS tier_arr
	FROM mdl_pricing GROUP BY model_id
)
UPDATE mdl_pricing p SET pricing =
	CASE p.billing_mode
		WHEN 'per_request' THEN jsonb_build_object('price', COALESCE(p.per_request_price, 0))
		WHEN 'tiered' THEN jsonb_build_object(
			'tiers', t.tier_arr,
			'cache_read_price', p.cache_read_price,
			'cache_creation_price', p.cache_creation_price)
		ELSE jsonb_build_object(
			'input_price', p.input_price, 'output_price', p.output_price,
			'cache_read_price', p.cache_read_price,
			'cache_creation_price', p.cache_creation_price)
	END
	|| CASE WHEN p.time_segments IS NOT NULL
	     THEN jsonb_build_object('time_segments', p.time_segments) ELSE '{}'::jsonb END
FROM tiers t
WHERE t.model_id = p.model_id AND p.min_tokens = 0 AND p.pricing IS NULL;
-- +goose StatementEnd

-- 4. 删除阶梯档位行（档位已聚合进锚点行 pricing->'tiers'）
-- +goose StatementBegin
DELETE FROM mdl_pricing WHERE min_tokens <> 0;
-- +goose StatementEnd

-- 5. 删除旧价格列与时段列（时段已并入 pricing 顶层）
-- +goose StatementBegin
ALTER TABLE mdl_pricing
	DROP COLUMN IF EXISTS min_tokens,
	DROP COLUMN IF EXISTS max_tokens,
	DROP COLUMN IF EXISTS input_price,
	DROP COLUMN IF EXISTS output_price,
	DROP COLUMN IF EXISTS per_request_price,
	DROP COLUMN IF EXISTS cache_read_price,
	DROP COLUMN IF EXISTS cache_creation_price,
	DROP COLUMN IF EXISTS time_segments;
-- +goose StatementEnd

-- 6. 每模型一行唯一约束
-- +goose StatementBegin
ALTER TABLE mdl_pricing ADD CONSTRAINT uk_mdl_pricing_model UNIQUE (model_id);
-- +goose StatementEnd

-- 7. billing_mode 值域新增 special：pricing 顶层声明了 scheme 键的存量方案模型（如 MiniMax-H3 的
--    custom:minimax-material 素材计费）从 per_second 改标 special，使用量日志/账单展示「特殊计费」
--    而非「按秒计费」。计费金额不受影响（计费分发只看 pricing JSONB 的 scheme 键，billing_mode 是
--    展示参考字段）。WHERE 带 billing_mode='per_second' 条件，可重复执行（幂等）。
UPDATE mdl_pricing
	SET billing_mode = 'special'
	WHERE billing_mode = 'per_second'
	  AND pricing->>'scheme' IS NOT NULL
	  AND pricing->>'scheme' <> '';

COMMENT ON COLUMN mdl_pricing.billing_mode IS '计费模式：token=按量；per_request=按次；tiered=阶梯；per_second=按秒（视频）；special=特殊计费（pricing JSONB 顶层 scheme 声明计费方案，如 custom:minimax-material）';

-- ── 三、bil_usage_logs：按秒计费任务时长 ──
-- +goose StatementBegin
ALTER TABLE bil_usage_logs ADD COLUMN IF NOT EXISTS duration_seconds INT;
-- +goose StatementEnd

COMMENT ON COLUMN bil_usage_logs.duration_seconds IS '按秒计费任务的视频时长（秒），来自任务提交时的 spec.duration；非时长类任务为 NULL';

-- ── 四、tnt_tenants 标签备注 + bil_wallets 累计消费（方案 A：Redis 权威 + DB 物化副本）──
-- +goose StatementBegin
ALTER TABLE tnt_tenants
	ADD COLUMN tags JSONB NOT NULL DEFAULT '[]'::jsonb,
	ADD COLUMN remark VARCHAR(1000) NOT NULL DEFAULT '';
-- +goose StatementEnd

COMMENT ON COLUMN tnt_tenants.tags IS '租户标签（JSONB 字符串数组，管理后台运营标注，不暴露给租户控制台）';
COMMENT ON COLUMN tnt_tenants.remark IS '管理员备注（运营备注，不暴露给租户控制台）';

-- +goose StatementBegin
ALTER TABLE bil_wallets
	ADD COLUMN total_consumed NUMERIC(20,10) NOT NULL DEFAULT 0;
-- +goose StatementEnd

COMMENT ON COLUMN bil_wallets.total_consumed IS '累计消费总额（本位币；Redis 钱包 hash total_consumed 的物化副本，随结算事件递增/补偿递减）';

-- 存量回填：从流水账重放历史消费（consume 流水 amount 为负值），幂等可重跑
UPDATE bil_wallets w
SET total_consumed = COALESCE((
	SELECT SUM(-t.amount) FROM bil_transactions t
	WHERE t.tenant_id = w.tenant_id AND t.type = 'consume'
), 0);

-- ── 五、mdl_models：研发厂商字段 ──
-- 注意与 chn_channels.type（渠道供应商/托管方）是两个维度：研发厂商指模型的开发公司，
-- 例如 Bedrock 上的 Claude 厂商仍是 anthropic，硅基流动上的 Qwen 厂商仍是 alibaba。
-- +goose StatementBegin
ALTER TABLE mdl_models
	ADD COLUMN vendor VARCHAR(30) NOT NULL DEFAULT '';
-- +goose StatementEnd

COMMENT ON COLUMN mdl_models.vendor IS '研发厂商枚举：openai/anthropic/google/xai/mistral/cohere/meta/alibaba/bytedance/deepseek/zhipu/moonshot/minimax/baidu/tencent/xunfei/kuaishou/midjourney/suno，空串=未分类';

-- 存量回填：按 model_id 前缀推断厂商（仅回填 vendor 为空的行，幂等可重跑；推断不准可在管理后台修正）
UPDATE mdl_models SET vendor = 'openai'    WHERE vendor = '' AND (model_id LIKE 'gpt-%' OR model_id LIKE 'chatgpt-%' OR model_id LIKE 'o1%' OR model_id LIKE 'o3%' OR model_id LIKE 'o4%' OR model_id LIKE 'codex-%' OR model_id LIKE 'sora-%' OR model_id LIKE 'dall-e-%' OR model_id LIKE 'gpt-image%' OR model_id LIKE 'whisper-%' OR model_id LIKE 'tts-%' OR model_id LIKE 'text-embedding-%');
UPDATE mdl_models SET vendor = 'anthropic' WHERE vendor = '' AND model_id LIKE 'claude-%';
UPDATE mdl_models SET vendor = 'google'    WHERE vendor = '' AND (model_id LIKE 'gemini-%' OR model_id LIKE 'veo-%' OR model_id LIKE 'imagen-%');
UPDATE mdl_models SET vendor = 'xai'       WHERE vendor = '' AND model_id LIKE 'grok-%';
UPDATE mdl_models SET vendor = 'mistral'   WHERE vendor = '' AND (model_id LIKE 'mistral-%' OR model_id LIKE 'codestral-%' OR model_id LIKE 'pixtral-%' OR model_id LIKE 'magistral-%');
UPDATE mdl_models SET vendor = 'cohere'    WHERE vendor = '' AND (model_id LIKE 'command-%' OR model_id LIKE 'embed-%' OR model_id LIKE 'rerank-%');
UPDATE mdl_models SET vendor = 'meta'      WHERE vendor = '' AND model_id LIKE 'llama-%';
UPDATE mdl_models SET vendor = 'alibaba'   WHERE vendor = '' AND (model_id LIKE 'qwen%' OR model_id LIKE 'qwq%' OR model_id LIKE 'qvq%');
UPDATE mdl_models SET vendor = 'bytedance' WHERE vendor = '' AND (model_id LIKE 'doubao-%' OR model_id LIKE 'seedream-%' OR model_id LIKE 'seedance-%' OR model_id LIKE 'jimeng-%');
UPDATE mdl_models SET vendor = 'deepseek'  WHERE vendor = '' AND model_id LIKE 'deepseek-%';
UPDATE mdl_models SET vendor = 'zhipu'     WHERE vendor = '' AND model_id LIKE 'glm-%';
UPDATE mdl_models SET vendor = 'moonshot'  WHERE vendor = '' AND (model_id LIKE 'kimi-%' OR model_id LIKE 'moonshot-%');
UPDATE mdl_models SET vendor = 'minimax'   WHERE vendor = '' AND (model_id LIKE 'minimax-%' OR model_id LIKE 'abab-%');
UPDATE mdl_models SET vendor = 'baidu'     WHERE vendor = '' AND (model_id LIKE 'ernie-%' OR model_id LIKE 'wenxin-%');
UPDATE mdl_models SET vendor = 'tencent'   WHERE vendor = '' AND model_id LIKE 'hunyuan-%';
UPDATE mdl_models SET vendor = 'xunfei'    WHERE vendor = '' AND model_id LIKE 'spark-%';
UPDATE mdl_models SET vendor = 'kuaishou'  WHERE vendor = '' AND model_id LIKE 'kling-%';
UPDATE mdl_models SET vendor = 'midjourney' WHERE vendor = '' AND (model_id LIKE 'mj_%' OR model_id LIKE 'midjourney-%');
UPDATE mdl_models SET vendor = 'suno'      WHERE vendor = '' AND model_id LIKE 'suno-%';

-- +goose Down
-- 逆序回滚：厂商 → 租户/钱包 → 用量时长 → 定价旧结构重建 → 视频软删除列。

-- ── 五、mdl_models：移除厂商字段 ──
-- +goose StatementBegin
ALTER TABLE mdl_models
	DROP COLUMN IF EXISTS vendor;
-- +goose StatementEnd

-- ── 四、tnt_tenants / bil_wallets ──
-- +goose StatementBegin
ALTER TABLE tnt_tenants
	DROP COLUMN IF EXISTS tags,
	DROP COLUMN IF EXISTS remark;
ALTER TABLE bil_wallets
	DROP COLUMN IF EXISTS total_consumed;
-- +goose StatementEnd

-- ── 三、bil_usage_logs ──
-- +goose StatementBegin
ALTER TABLE bil_usage_logs DROP COLUMN IF EXISTS duration_seconds;
-- +goose StatementEnd

-- ── 二、mdl_pricing：从 pricing JSONB 无损反向重建旧结构 ──
-- JSON 自迁移起为唯一被写的真相，覆盖其后的全部改价。
-- special 反向还原：scheme 模型收回 per_second（旧代码完全兼容形态）；
-- 迁移后通过方案编辑器新保存的模型 scheme 非空，同样被覆盖回 per_second。
UPDATE mdl_pricing
	SET billing_mode = 'per_second'
	WHERE billing_mode = 'special'
	  AND pricing->>'scheme' IS NOT NULL
	  AND pricing->>'scheme' <> '';

COMMENT ON COLUMN mdl_pricing.billing_mode IS '计费模式：token=按量；per_request=按次；tiered=阶梯；per_second=按秒（视频）';

-- +goose StatementBegin
ALTER TABLE mdl_pricing DROP CONSTRAINT IF EXISTS uk_mdl_pricing_model;
ALTER TABLE mdl_pricing
	ADD COLUMN IF NOT EXISTS min_tokens BIGINT DEFAULT 0 NOT NULL,
	ADD COLUMN IF NOT EXISTS max_tokens BIGINT,
	ADD COLUMN IF NOT EXISTS input_price NUMERIC(20,10) DEFAULT 0 NOT NULL,
	ADD COLUMN IF NOT EXISTS output_price NUMERIC(20,10) DEFAULT 0 NOT NULL,
	ADD COLUMN IF NOT EXISTS per_request_price NUMERIC(20,10),
	ADD COLUMN IF NOT EXISTS cache_read_price NUMERIC(20,10) DEFAULT 0 NOT NULL,
	ADD COLUMN IF NOT EXISTS cache_creation_price NUMERIC(20,10) DEFAULT 0 NOT NULL,
	ADD COLUMN IF NOT EXISTS time_segments JSONB;
-- +goose StatementEnd

-- 锚点行标量列从 JSON 回写（tiered 取首档价，与旧行为一致）
-- +goose StatementBegin
UPDATE mdl_pricing SET
	input_price = COALESCE((pricing->>'input_price')::numeric, (pricing->'tiers'->0->>'input_price')::numeric, 0),
	output_price = COALESCE((pricing->>'output_price')::numeric, (pricing->'tiers'->0->>'output_price')::numeric, 0),
	per_request_price = (pricing->>'price')::numeric,
	cache_read_price = COALESCE((pricing->>'cache_read_price')::numeric, 0),
	cache_creation_price = COALESCE((pricing->>'cache_creation_price')::numeric, 0),
	time_segments = pricing->'time_segments'
WHERE pricing IS NOT NULL;
-- +goose StatementEnd

-- tiered 档位行从 JSON tiers 数组展开重建（跳过首档——首档即锚点行 min_tokens=0）
-- +goose StatementBegin
INSERT INTO mdl_pricing (model_id, billing_mode, min_tokens, max_tokens, input_price, output_price, created_at, updated_at)
SELECT p.model_id,
       p.billing_mode,
       (t.tier->>'min_tokens')::bigint,
       NULLIF(t.tier->>'max_tokens', '')::bigint,
       COALESCE((t.tier->>'input_price')::numeric, 0),
       COALESCE((t.tier->>'output_price')::numeric, 0),
       now(),
       now()
FROM mdl_pricing p
	CROSS JOIN LATERAL jsonb_array_elements(COALESCE(p.pricing->'tiers', '[]'::jsonb))
	WITH ORDINALITY AS t(tier, ord)
WHERE p.billing_mode = 'tiered'
  AND p.pricing IS NOT NULL
  AND (t.tier->>'min_tokens')::bigint <> 0
  AND t.ord > 1;
-- +goose StatementEnd

-- JSON 列最后删除（上方旧结构重建均需读取 pricing）
-- +goose StatementBegin
ALTER TABLE mdl_pricing
	DROP COLUMN IF EXISTS pricing,
	DROP COLUMN IF EXISTS official_pricing;
-- +goose StatementEnd

-- ── 一、tsk_model_tasks ──
-- +goose StatementBegin
ALTER TABLE tsk_model_tasks
	DROP COLUMN IF EXISTS deleted,
	DROP COLUMN IF EXISTS deleted_at;
-- +goose StatementEnd
