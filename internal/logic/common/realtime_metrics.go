package common

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// 实时 RPM/TPM 统计（参考 new-api 的"最近 60 秒"口径，但不查日志表）：
// 写入侧在每条用量记录落地时向 Redis 的 10 秒桶原子累加请求数与 token 数（全局 + 租户两个维度），
// 读取侧对最近 60 秒做滑动窗口聚合（6 个整桶 + 最旧桶按窗口覆盖比例加权）。
// 相比 new-api 每次实时 COUNT 日志表，写入 O(1)、读取 O(桶数)，与 bil_usage_logs 的规模解耦。
// 指标为 best-effort：Redis 不可用时计数丢弃、查询返回 0，不影响请求主链路。
const (
	rtMetricsKeyPrefix     = "rt_metrics:" // rt_metrics:{scope}:{bucketStartUnix}
	rtMetricsBucketSeconds = 10            // 桶粒度（秒）
	rtMetricsWindowSeconds = 60            // 统计窗口（秒），与 new-api 的 RPM/TPM 口径一致
	rtMetricsKeyTTL        = 180           // 桶 TTL（秒），覆盖读取窗口即可
)

// rtMetricsWindowBuckets 窗口内整桶数
const rtMetricsWindowBuckets = rtMetricsWindowSeconds / rtMetricsBucketSeconds

// rtMetricsIncrLua 一次 RTT 原子累加所有维度的桶计数并续期。
// KEYS = 各维度的当前桶 key；ARGV[1] = token 数，ARGV[2] = TTL 秒。
const rtMetricsIncrLua = `
for i = 1, #KEYS do
	redis.call("HINCRBY", KEYS[i], "req", 1)
	redis.call("HINCRBY", KEYS[i], "tok", tonumber(ARGV[1]))
	redis.call("EXPIRE", KEYS[i], tonumber(ARGV[2]))
end
return 1
`

// rtMetricsReadLua 一次 RTT 读取一组桶的 req/tok 计数。
// KEYS = 桶 key（从旧到新）；返回扁平数组 [req1, tok1, req2, tok2, ...]，缺失桶为 0。
const rtMetricsReadLua = `
local res = {}
for i = 1, #KEYS do
	res[2*i-1] = tonumber(redis.call("HGET", KEYS[i], "req") or "0")
	res[2*i] = tonumber(redis.call("HGET", KEYS[i], "tok") or "0")
end
return res
`

// rtMetricsBucketKey 桶 key：scope 为 "g"（全局）或 "t:{tenantID}"（租户）
func rtMetricsBucketKey(scope string, bucketStart int64) string {
	return fmt.Sprintf("%s%s:%d", rtMetricsKeyPrefix, scope, bucketStart)
}

func rtMetricsTenantScope(tenantID int64) string {
	return fmt.Sprintf("t:%d", tenantID)
}

// RecordRealtimeMetrics 累加实时 RPM/TPM 计数（全局 + 租户维度）。
// 在用量记录入口（DataProvider.RecordUsage）调用，每条用量记录计 1 次请求、tokens 个 token。
// 内部异步执行且脱离请求 ctx 的取消，失败仅记 debug 日志，不影响主链路。
func RecordRealtimeMetrics(ctx context.Context, tenantID int64, tokens int64) {
	if tokens < 0 {
		tokens = 0
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				g.Log().Warningf(context.Background(), "realtime metrics incr panic: tenant=%d panic=%v", tenantID, r)
			}
		}()
		bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()

		now := time.Now().Unix()
		bucketStart := now - now%rtMetricsBucketSeconds

		keys := []any{rtMetricsBucketKey("g", bucketStart)}
		if tenantID > 0 {
			keys = append(keys, rtMetricsBucketKey(rtMetricsTenantScope(tenantID), bucketStart))
		}

		args := make([]any, 0, len(keys)+4)
		args = append(args, rtMetricsIncrLua, len(keys))
		args = append(args, keys...)
		args = append(args, tokens, rtMetricsKeyTTL)
		if _, err := g.Redis().Do(bgCtx, "EVAL", args...); err != nil {
			// 指标是 best-effort，Redis 抖动只记 debug，不刷告警
			g.Log().Debugf(bgCtx, "realtime metrics incr failed: tenant=%d err=%v", tenantID, err)
		}
	}()
}

// GetRealtimeMetrics 返回最近 60 秒滑动窗口的 RPM（请求数）与 TPM（token 数）。
// tenantID <= 0 时返回全局维度，否则返回指定租户维度。
// Redis 不可用时返回 0, 0（看板容忍指标短暂缺失）。
func GetRealtimeMetrics(ctx context.Context, tenantID int64) (rpm int64, tpm int64) {
	scope := "g"
	if tenantID > 0 {
		scope = rtMetricsTenantScope(tenantID)
	}

	now := time.Now().Unix()
	bucketStart := now - now%rtMetricsBucketSeconds
	elapsed := now - bucketStart

	// 桶 key 从旧到新：窗口涉及 windowBuckets+1 个桶（最旧桶只有部分时间落在窗口内）
	keys := make([]any, 0, rtMetricsWindowBuckets+1)
	for i := rtMetricsWindowBuckets; i >= 0; i-- {
		keys = append(keys, rtMetricsBucketKey(scope, bucketStart-int64(i)*rtMetricsBucketSeconds))
	}

	args := make([]any, 0, len(keys)+2)
	args = append(args, rtMetricsReadLua, len(keys))
	args = append(args, keys...)
	result, err := g.Redis().Do(ctx, "EVAL", args...)
	if err != nil {
		g.Log().Debugf(ctx, "realtime metrics read failed: scope=%s err=%v", scope, err)
		return 0, 0
	}

	vals := result.Vars()
	reqBuckets := make([]int64, 0, rtMetricsWindowBuckets+1)
	tokBuckets := make([]int64, 0, rtMetricsWindowBuckets+1)
	for i := 0; i+1 < len(vals); i += 2 {
		reqBuckets = append(reqBuckets, vals[i].Int64())
		tokBuckets = append(tokBuckets, vals[i+1].Int64())
	}

	return slidingWindowSum(reqBuckets, elapsed), slidingWindowSum(tokBuckets, elapsed)
}

// slidingWindowSum 对从旧到新排列的桶计数做 60 秒滑动窗口聚合。
// 最旧桶只有 (bucketSeconds - elapsed) 秒落在窗口内，按覆盖比例加权，其余桶全额计入；
// elapsed 为当前时间在当前桶内经过的秒数（0 ~ bucketSeconds-1）。
func slidingWindowSum(buckets []int64, elapsed int64) int64 {
	if len(buckets) == 0 {
		return 0
	}
	oldestWeight := float64(rtMetricsBucketSeconds-elapsed) / float64(rtMetricsBucketSeconds)
	if oldestWeight < 0 {
		oldestWeight = 0
	}
	if oldestWeight > 1 {
		oldestWeight = 1
	}
	sum := float64(buckets[0]) * oldestWeight
	for _, v := range buckets[1:] {
		sum += float64(v)
	}
	return int64(math.Round(sum))
}
