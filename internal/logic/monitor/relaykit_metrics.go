package monitor

import (
	"encoding/json"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/gtime"
	do "github.com/qianfree/team-api/internal/model/do"
)

// relaykit 转换器监控（改造期间可观测性）。
//
// 设计说明：
//   - 转换是高频事件（每个请求一次），逐次落库代价过高，故采用「内存原子计数 +
//     每分钟聚合落库」：热路径 TrackConverterCall 只做 atomic 自增（无锁），
//     flushRelaykitMetrics 在 collector 每分钟 tick 上把累计快照写入 ops_system_metrics。
//   - 计数为累计值（自启动起），相邻行做差即得窗口速率；实现最简、最不易错。
//   - 落库复用 collector.go 的包级 metricsWriter + 真实 ops_system_metrics schema
//     （metric_type + JSONB metric_data + collected_at）。

// RelayConverterMetrics 是单个转换器的累计指标快照（读取时计算派生字段）。
type RelayConverterMetrics struct {
	ConverterID   string  `json:"converter_id"`            // 如 "openai_to_claude"
	From          string  `json:"from"`                    // 源协议格式
	To            string  `json:"to"`                      // 目标协议格式
	Success       int64   `json:"success"`                 // 累计成功次数
	Failed        int64   `json:"failed"`                  // 累计失败次数
	TotalMs       int64   `json:"total_ms"`                // 累计耗时（毫秒），用于算均值
	LastError     string  `json:"last_error,omitempty"`    // 最近一次错误信息（截断）
	ErrorRate     float64 `json:"error_rate"`              // 派生：失败率 = failed/(success+failed)
	AvgDurationMs float64 `json:"avg_duration_ms"`         // 派生：平均耗时（毫秒）
}

// converterCounter 是单个转换器的并发安全计数器。
// from/to 在创建时写入后只读（无需加锁）；热路径字段用 atomic；lastErr 失败稀疏，用自带互斥锁。
type converterCounter struct {
	from     string
	to       string
	success  atomic.Int64
	failed   atomic.Int64
	totalMs  atomic.Int64
	lastErrMu sync.Mutex
	lastErr   string
}

func (c *converterCounter) setLastError(err error) {
	if err == nil {
		return
	}
	msg := err.Error()
	// 截断，避免错误信息过大撑爆 JSONB payload。
	if len(msg) > 500 {
		msg = msg[:500]
	}
	c.lastErrMu.Lock()
	c.lastErr = msg
	c.lastErrMu.Unlock()
}

func (c *converterCounter) lastErrorString() string {
	c.lastErrMu.Lock()
	defer c.lastErrMu.Unlock()
	return c.lastErr
}

// relaykitTracker 是全局单例，按 converterID 聚合各转换器的计数器。
type relaykitTracker struct {
	mu       sync.RWMutex
	counters map[string]*converterCounter
}

var relaykitT *relaykitTracker

// InitRelaykitTracker 初始化全局 relaykit 计数器。在 cmd 启动流程中与 InitRequestTracker 一起调用。
func InitRelaykitTracker() {
	relaykitT = &relaykitTracker{counters: make(map[string]*converterCounter)}
}

// getOrCreate 以双检锁获取或创建计数器（首次见到某 converterID 时创建并记录 from/to）。
func (t *relaykitTracker) getOrCreate(converterID, from, to string) *converterCounter {
	t.mu.RLock()
	c, ok := t.counters[converterID]
	t.mu.RUnlock()
	if ok {
		return c
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if c, ok := t.counters[converterID]; ok {
		return c
	}
	c = &converterCounter{from: from, to: to}
	t.counters[converterID] = c
	return c
}

// TrackConverterCall 记录一次转换器调用（热路径入口，阶段 4 由 handler 调用）。
// converterID 形如 "openai_to_claude"；duration 为本次转换耗时；err 非空视为失败。
func TrackConverterCall(converterID, from, to string, duration time.Duration, err error) {
	if relaykitT == nil || converterID == "" {
		return
	}
	c := relaykitT.getOrCreate(converterID, from, to)
	if err != nil {
		c.failed.Add(1)
		c.setLastError(err)
	} else {
		c.success.Add(1)
	}
	if duration > 0 {
		c.totalMs.Add(duration.Milliseconds())
	}
}

// GetRelaykitConverterMetrics 返回所有转换器的累计指标快照（按 converterID 排序，结果稳定）。
// 用于 dashboard 展示与定时落库。无活动时返回 nil。
func GetRelaykitConverterMetrics() []RelayConverterMetrics {
	if relaykitT == nil {
		return nil
	}
	relaykitT.mu.RLock()
	ids := make([]string, 0, len(relaykitT.counters))
	for id := range relaykitT.counters {
		ids = append(ids, id)
	}
	relaykitT.mu.RUnlock()
	if len(ids) == 0 {
		return nil
	}
	sort.Strings(ids)

	out := make([]RelayConverterMetrics, 0, len(ids))
	for _, id := range ids {
		relaykitT.mu.RLock()
		c := relaykitT.counters[id]
		relaykitT.mu.RUnlock()
		if c == nil {
			continue
		}
		success := c.success.Load()
		failed := c.failed.Load()
		total := success + failed
		var errorRate, avgMs float64
		if total > 0 {
			errorRate = float64(failed) / float64(total)
			avgMs = float64(c.totalMs.Load()) / float64(total)
		}
		out = append(out, RelayConverterMetrics{
			ConverterID:   id,
			From:          c.from,
			To:            c.to,
			Success:       success,
			Failed:        failed,
			TotalMs:       c.totalMs.Load(),
			LastError:     c.lastErrorString(),
			ErrorRate:     errorRate,
			AvgDurationMs: avgMs,
		})
	}
	return out
}

// flushRelaykitMetrics 把累计指标快照写入 ops_system_metrics（metric_type=relaykit_converter）。
// 由 collector.go 的 CollectSystemMetrics 每分钟 tick 调用；无 relaykit 活动时跳过，避免空噪声。
func flushRelaykitMetrics(ts time.Time) {
	metrics := GetRelaykitConverterMetrics()
	if len(metrics) == 0 {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"converters":          metrics,
		"window_collected_at": ts,
	})
	if err != nil {
		g.Log().Warningf(gctx.New(), "marshal relaykit metrics failed: %v", err)
		return
	}
	metricsWriter.Write(gctx.New(), do.OpsSystemMetrics{
		MetricType:  "relaykit_converter",
		MetricData:  string(payload),
		CollectedAt: gtime.NewFromTime(ts),
	})
}
