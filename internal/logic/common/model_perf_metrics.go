package common

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// 模型当日性能热桶（参考 new-api perf_metrics 的「Redis 小时桶」设计）。
//
// 背景：bil_usage_daily 日聚合任务固定 01:00 运行且当天数据不聚合（usage_daily_aggregate，
// end 取今天 00:00 开区间），导致「模型性能」页只能看到前一天的数据。本模块在用量记录入口
// （DataProvider.RecordUsage）把每次请求的关键指标原子累加到 Redis 按小时的实时桶，供模型性能页
// 「当天」实时展示；次日数据由 usage_daily_aggregate 落入 bil_usage_daily 后即不再依赖本桶。
//
// 指标口径与 bil_usage_daily 对齐：
//   - 成功率 = 成功请求 / 全部请求
//   - 平均延迟 = Σlatency / Σrequest（lat 仅累加有延迟的样本，分母为全部请求，与日聚合
//     SUM(latency_ms) 对 NULL 跳过、但 SUM/COUNT 用全部请求数作分母的语义一致）
//   - 平均首 Token = Σttft / Σttft_n（首 Token 单独计数，避免非流式/无首Token请求计入分母拉低均值）
//   - 成本以整数 micro-USD 累加（对齐 Redis 钱包 micro 规范，杜绝 HINCRBYFLOAT 漂移）
//   - 缓存指标与日聚合对齐：cache 命中请求数（chit）按「该次请求 cache_read>0」计 1，
//     仅写入缓存（creation>0、read=0）不计命中
//
// 可靠性为 best-effort：Redis 不可用时计数丢弃、查询返回空，不影响请求主链路，页面退化为历史日表。
const (
	modelPerfPrefix      = "perf_model:"    // perf_model:{hex(model)}:{hourStartUnix}
	modelPerfModelsKeyFF = "perf_models:%s" // perf_models:2026-08-13（当日活跃模型集合）
	modelPerfBucketSec   = 3600             // 桶粒度：小时
	modelPerfTTLSeconds  = 48 * 3600        // 桶/集合 TTL（秒）：覆盖到次日 01:00 日聚合跑完
)

// ModelPerfRecord 一次请求的模型性能采样（由 relay 层 RecordUsage 传入）。
type ModelPerfRecord struct {
	Model        string
	Success      bool
	LatencyMs    float64
	FirstTokenMs int
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	// 缓存 token（命中判定由本模块从 CacheReadTokens>0 推导，调用方无需单独传标志）
	CacheCreationTokens int
	CacheReadTokens     int
}

// ModelPerfCounter 当日某模型的累计计数（Redis 小时桶聚合结果）。
// CostMicro 为累计成本，单位 micro-USD（整数），查询侧用 billing.FromMicro 转回 USD。
type ModelPerfCounter struct {
	Req       int64
	Ok        int64
	Lat       int64 // Σ延迟 ms
	Ttft      int64 // Σ首Token延迟 ms
	TtftN     int64 // 有首Token的样本数
	Tin       int64
	Tout      int64
	CostMicro int64
	// 缓存累计：creation/read token 与 命中缓存的请求数（与 bil_usage_daily 同口径）
	CacheCreation int64
	CacheRead     int64
	CacheHitReq   int64
}

func modelPerfModelsKey(date string) string {
	return fmt.Sprintf(modelPerfModelsKeyFF, date)
}

// modelPerfBucketKey 拼接小时桶 key：model 以 hex 编码，避免模型名含 `:` 等分隔符破坏 key 结构。
func modelPerfBucketKey(hexModel string, hourStart int64) string {
	return fmt.Sprintf("%s%s:%d", modelPerfPrefix, hexModel, hourStart)
}

func modelPerfHex(model string) string {
	return hex.EncodeToString([]byte(model))
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// microFromUSD 将展示口径的 USD float64 转成整数 micro-USD（仅累计展示用，非账本）。
func microFromUSD(usd float64) int64 {
	return int64(math.Round(usd * 1_000_000))
}

// modelPerfIncrLua 一次 RTT 把样本原子累加到「当日模型集合 + 小时桶」并续期。
// KEYS[1] = perf_models:{date}；KEYS[2] = perf_model:{hex}:{hour}
// ARGV[1] = 模型名；ARGV[2] = TTL 秒；ARGV[3] = ok(1/0)；ARGV[4] = lat ms；
// ARGV[5] = ttft ms；ARGV[6] = input tokens；ARGV[7] = output tokens；ARGV[8] = cost micro-USD；
// ARGV[9] = cache creation tokens；ARGV[10] = cache read tokens（>0 时 chit 同步 +1）。
const modelPerfIncrLua = `
redis.call("SADD", KEYS[1], ARGV[1])
redis.call("EXPIRE", KEYS[1], ARGV[2])
redis.call("HINCRBY", KEYS[2], "req", 1)
if tonumber(ARGV[3]) == 1 then redis.call("HINCRBY", KEYS[2], "ok", 1) end
if tonumber(ARGV[4]) > 0 then redis.call("HINCRBY", KEYS[2], "lat", tonumber(ARGV[4])) end
if tonumber(ARGV[5]) > 0 then
	redis.call("HINCRBY", KEYS[2], "ttft", tonumber(ARGV[5]))
	redis.call("HINCRBY", KEYS[2], "ttft_n", 1)
end
if tonumber(ARGV[6]) > 0 then redis.call("HINCRBY", KEYS[2], "tin", tonumber(ARGV[6])) end
if tonumber(ARGV[7]) > 0 then redis.call("HINCRBY", KEYS[2], "tout", tonumber(ARGV[7])) end
if tonumber(ARGV[8]) > 0 then redis.call("HINCRBY", KEYS[2], "cost", tonumber(ARGV[8])) end
if tonumber(ARGV[9]) > 0 then redis.call("HINCRBY", KEYS[2], "ccr", tonumber(ARGV[9])) end
if tonumber(ARGV[10]) > 0 then
	redis.call("HINCRBY", KEYS[2], "crd", tonumber(ARGV[10]))
	redis.call("HINCRBY", KEYS[2], "chit", 1)
end
redis.call("EXPIRE", KEYS[2], ARGV[2])
return 1
`

// modelPerfReadLua 累加某模型当日所有小时桶计数。KEYS = 当日各小时桶 key。
// 返回扁平数组 [req, ok, lat, ttft, ttft_n, tin, tout, cost, ccr, crd, chit]。
// 旧版本桶缺失 ccr/crd/chit 字段时 tonumber(nil) or 0 兜底为 0，平滑上线。
const modelPerfReadLua = `
local req,ok,lat,ttft,ttftn,tin,tout,cost,ccr,crd,chit = 0,0,0,0,0,0,0,0,0,0,0
for i = 1, #KEYS do
	local h = redis.call("HGETALL", KEYS[i])
	for j = 1, #h, 2 do
		local f = h[j]
		local v = tonumber(h[j+1]) or 0
		if f == "req" then req = req + v
		elseif f == "ok" then ok = ok + v
		elseif f == "lat" then lat = lat + v
		elseif f == "ttft" then ttft = ttft + v
		elseif f == "ttft_n" then ttftn = ttftn + v
		elseif f == "tin" then tin = tin + v
		elseif f == "tout" then tout = tout + v
		elseif f == "cost" then cost = cost + v
		elseif f == "ccr" then ccr = ccr + v
		elseif f == "crd" then crd = crd + v
		elseif f == "chit" then chit = chit + v end
	end
end
return {req, ok, lat, ttft, ttftn, tin, tout, cost, ccr, crd, chit}
`

// RecordModelPerfMetrics 将一次请求的模型性能采样累加到 Redis 当日小时桶。
// 在用量记录入口（DataProvider.RecordUsage）调用；内部异步执行并脱离请求 ctx 的取消，
// 失败仅记 debug 日志，不影响请求主链路（best-effort）。
func RecordModelPerfMetrics(ctx context.Context, r ModelPerfRecord) {
	if r.Model == "" {
		return
	}
	go func() {
		defer func() {
			if p := recover(); p != nil {
				g.Log().Warningf(context.Background(), "model perf metrics incr panic: model=%s panic=%v", r.Model, p)
			}
		}()
		bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()

		now := time.Now()
		hourStart := now.Unix() - now.Unix()%modelPerfBucketSec
		keys := []any{
			modelPerfModelsKey(now.Format("2006-01-02")),
			modelPerfBucketKey(modelPerfHex(r.Model), hourStart),
		}
		args := []any{
			modelPerfIncrLua, 2,
			keys[0], keys[1],
			r.Model, modelPerfTTLSeconds,
			boolToInt(r.Success), int64(math.Round(r.LatencyMs)),
			int64(r.FirstTokenMs), int64(r.InputTokens), int64(r.OutputTokens),
			microFromUSD(r.CostUSD),
			int64(r.CacheCreationTokens), int64(r.CacheReadTokens),
		}
		if _, err := g.Redis().Do(bgCtx, "EVAL", args...); err != nil {
			// 指标是 best-effort，Redis 抖动只记 debug，不刷告警
			g.Log().Debugf(bgCtx, "model perf metrics incr failed: model=%s err=%v", r.Model, err)
		}
	}()
}

// GetModelPerfMetricsToday 返回指定日期（YYYY-MM-DD，本地时区）Redis 当日各模型的性能累计。
// 当日无请求与 Redis 不可用时返回错误，由调用方决定降级（页面退化为仅历史日表）。
func GetModelPerfMetricsToday(ctx context.Context, date string) (map[string]ModelPerfCounter, error) {
	dayStart, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return nil, err
	}
	dayStartUnix := dayStart.Unix()
	now := time.Now()
	if now.Unix() < dayStartUnix {
		return nil, nil
	}
	hours := int((now.Unix()-dayStartUnix)/modelPerfBucketSec) + 1 // 已开始的小时数（含当前小时）
	if hours > 24 {
		hours = 24
	}

	modelsRes, err := g.Redis().Do(ctx, "SMEMBERS", modelPerfModelsKey(date))
	if err != nil {
		return nil, err
	}
	modelVars := modelsRes.Vars()
	if len(modelVars) == 0 {
		return map[string]ModelPerfCounter{}, nil
	}

	out := make(map[string]ModelPerfCounter, len(modelVars))
	for _, mv := range modelVars {
		model := mv.String()
		keys := make([]any, 0, hours)
		for h := 0; h < hours; h++ {
			keys = append(keys, modelPerfBucketKey(modelPerfHex(model), dayStartUnix+int64(h)*modelPerfBucketSec))
		}
		args := make([]any, 0, len(keys)+2)
		args = append(args, modelPerfReadLua, len(keys))
		args = append(args, keys...)
		res, err := g.Redis().Do(ctx, "EVAL", args...)
		if err != nil {
			return nil, err
		}
		vals := res.Vars()
		out[model] = ModelPerfCounter{
			Req:           vals[0].Int64(),
			Ok:            vals[1].Int64(),
			Lat:           vals[2].Int64(),
			Ttft:          vals[3].Int64(),
			TtftN:         vals[4].Int64(),
			Tin:           vals[5].Int64(),
			Tout:          vals[6].Int64(),
			CostMicro:     vals[7].Int64(),
			CacheCreation: vals[8].Int64(),
			CacheRead:     vals[9].Int64(),
			CacheHitReq:   vals[10].Int64(),
		}
	}
	return out, nil
}
