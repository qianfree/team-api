package task

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
)

// aggregateUsageSQL 将 bil_usage_logs 在 [startDate, endDate)（半开区间，endDate 不含）
// 内的记录按 租户/项目/模型/渠道/状态 维度聚合，幂等 upsert 到 bil_usage_daily。
// 参数为 YYYY-MM-DD 字符串。ON CONFLICT 使重跑安全（覆盖短暂宕机、历史回填重算）。
const aggregateUsageSQL = `
INSERT INTO bil_usage_daily
    (stat_date, tenant_id, project_id, model_name, channel_id, status,
     request_count, input_tokens, output_tokens, total_cost, account_cost,
     sum_latency_ms, sum_first_token_ms,
     cache_creation_tokens, cache_read_tokens, cache_hit_request_count,
     updated_at)
SELECT
    DATE(created_at)                            AS stat_date,
    tenant_id                                   AS tenant_id,
    COALESCE(project_id, 0)                     AS project_id,
    COALESCE(NULLIF(model_name, ''), 'unknown') AS model_name,
    COALESCE(channel_id, 0)                     AS channel_id,
    COALESCE(NULLIF(status, ''), 'unknown')     AS status,
    COUNT(*)                                    AS request_count,
    COALESCE(SUM(input_tokens), 0)              AS input_tokens,
    COALESCE(SUM(output_tokens), 0)             AS output_tokens,
    COALESCE(SUM(total_cost), 0)                AS total_cost,
    COALESCE(SUM(account_cost), 0)              AS account_cost,
    COALESCE(SUM(latency_ms), 0)                AS sum_latency_ms,
    COALESCE(SUM(first_token_ms), 0)            AS sum_first_token_ms,
    COALESCE(SUM(cache_creation_tokens), 0)     AS cache_creation_tokens,
    COALESCE(SUM(cache_read_tokens), 0)         AS cache_read_tokens,
    COALESCE(SUM(CASE WHEN cache_read_tokens > 0 THEN 1 ELSE 0 END), 0) AS cache_hit_request_count,
    now()
FROM bil_usage_logs
WHERE created_at >= $1 AND created_at < $2
GROUP BY
    DATE(created_at),
    tenant_id,
    COALESCE(project_id, 0),
    COALESCE(NULLIF(model_name, ''), 'unknown'),
    COALESCE(channel_id, 0),
    COALESCE(NULLIF(status, ''), 'unknown')
ON CONFLICT (stat_date, tenant_id, project_id, model_name, channel_id, status) DO UPDATE SET
    request_count      = EXCLUDED.request_count,
    input_tokens       = EXCLUDED.input_tokens,
    output_tokens      = EXCLUDED.output_tokens,
    total_cost         = EXCLUDED.total_cost,
    account_cost       = EXCLUDED.account_cost,
    sum_latency_ms     = EXCLUDED.sum_latency_ms,
    sum_first_token_ms = EXCLUDED.sum_first_token_ms,
    cache_creation_tokens   = EXCLUDED.cache_creation_tokens,
    cache_read_tokens       = EXCLUDED.cache_read_tokens,
    cache_hit_request_count = EXCLUDED.cache_hit_request_count,
    updated_at         = now()
`

// AggregateUsageRange 将 bil_usage_logs 在 [startDate, endDate)（半开区间）的用量
// 聚合并幂等写入 bil_usage_daily。startDate/endDate 为 YYYY-MM-DD。
// 日常增量聚合与历史回填共用此函数：ON CONFLICT 保证可重复执行。
// 注意：endDate 为开区间上界，日常增量传「今天」可避免聚合当天进行中的数据。
func AggregateUsageRange(ctx context.Context, startDate, endDate string) error {
	res, err := g.DB().Ctx(ctx).Exec(ctx, aggregateUsageSQL,
		startDate+" 00:00:00",
		endDate+" 00:00:00",
	)
	if err != nil {
		g.Log().Errorf(ctx, "[Cron] 聚合 bil_usage_daily [%s,%s) 失败: %v", startDate, endDate, err)
		return err
	}
	if affected, e := res.RowsAffected(); e == nil {
		g.Log().Infof(ctx, "[Cron] 聚合 bil_usage_daily [%s,%s) 完成，影响 %d 行", startDate, endDate, affected)
	}
	return nil
}
