package monitor

import (
	"encoding/json"
	"maps"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/gtime"
	do "github.com/qianfree/team-api/internal/model/do"
)

// 渠道调度引擎监控（阶段 4，基线方案 §14.3）。
//
// 与 relaykit_metrics 同模式：热路径内存计数（低基数标签），collector 每分钟 tick
// 把累计快照写入 ops_system_metrics（metric_type=channel_dispatch）。计数为累计值，
// 相邻行做差即得窗口速率。对应指标语义：
//   - selections：dispatch_selection_total{reason}（bind/hrw/overflow/probe/cred_rotate）
//   - retries：dispatch_retry_total{err_class → decision}
//   - overflow_by_tier：dispatch_overflow_total{to_tier}（验证 §8.3 溢出是否按预期工作）
//   - session_sources：dispatch_session_source_total{source}（验证 Claude Code 解析有效性）
//   - breaker_opens：熔断打开次数（渠道级+模型级合计）

type dispatchTracker struct {
	mu             sync.Mutex
	selections     map[string]int64 // reason → 次数
	retries        map[string]int64 // "class→decision" → 次数
	overflowByTier map[string]int64 // 溢出目标层级 → 次数
	sessionSources map[string]int64 // 会话键来源 → 次数
	breakerOpens   int64            // 熔断打开次数
	noCandidate    int64            // 无可用渠道次数
}

var dispatchT *dispatchTracker

// InitDispatchTracker 初始化调度指标计数器（cmd 启动流程调用，与 InitRelaykitTracker 一起）。
func InitDispatchTracker() {
	dispatchT = &dispatchTracker{
		selections:     make(map[string]int64),
		retries:        make(map[string]int64),
		overflowByTier: make(map[string]int64),
		sessionSources: make(map[string]int64),
	}
}

// TrackDispatchSelection 记录一次调度选择（handler 每次 Next 成功后调用）。
func TrackDispatchSelection(reason, tier, sessionSource string) {
	if dispatchT == nil {
		return
	}
	dispatchT.mu.Lock()
	defer dispatchT.mu.Unlock()
	dispatchT.selections[reason]++
	dispatchT.sessionSources[sessionSource]++
	if reason == "overflow" {
		dispatchT.overflowByTier[tier]++
	}
}

// TrackDispatchRetry 记录一次失败重试决策（handler 每次 Report 后调用）。
func TrackDispatchRetry(errClass, decision string) {
	if dispatchT == nil {
		return
	}
	dispatchT.mu.Lock()
	defer dispatchT.mu.Unlock()
	dispatchT.retries[errClass+"→"+decision]++
}

// TrackDispatchNoCandidate 记录一次无可用渠道。
func TrackDispatchNoCandidate() {
	if dispatchT == nil {
		return
	}
	dispatchT.mu.Lock()
	defer dispatchT.mu.Unlock()
	dispatchT.noCandidate++
}

// TrackDispatchBreakerOpen 记录一次熔断打开（dispatchadapter 在转移发生时调用）。
func TrackDispatchBreakerOpen() {
	if dispatchT == nil {
		return
	}
	dispatchT.mu.Lock()
	defer dispatchT.mu.Unlock()
	dispatchT.breakerOpens++
}

// DispatchMetricsSnapshot 调度指标累计快照。
type DispatchMetricsSnapshot struct {
	Selections     map[string]int64 `json:"selections"`
	Retries        map[string]int64 `json:"retries"`
	OverflowByTier map[string]int64 `json:"overflow_by_tier"`
	SessionSources map[string]int64 `json:"session_sources"`
	BreakerOpens   int64            `json:"breaker_opens"`
	NoCandidate    int64            `json:"no_candidate"`
}

// GetDispatchMetrics 返回累计指标快照（dashboard 展示与定时落库用）。无活动时返回 nil。
func GetDispatchMetrics() *DispatchMetricsSnapshot {
	if dispatchT == nil {
		return nil
	}
	dispatchT.mu.Lock()
	defer dispatchT.mu.Unlock()
	if len(dispatchT.selections) == 0 && dispatchT.noCandidate == 0 && dispatchT.breakerOpens == 0 {
		return nil
	}
	return &DispatchMetricsSnapshot{
		Selections:     copyCounter(dispatchT.selections),
		Retries:        copyCounter(dispatchT.retries),
		OverflowByTier: copyCounter(dispatchT.overflowByTier),
		SessionSources: copyCounter(dispatchT.sessionSources),
		BreakerOpens:   dispatchT.breakerOpens,
		NoCandidate:    dispatchT.noCandidate,
	}
}

func copyCounter(m map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(m))
	maps.Copy(out, m)
	return out
}

// flushDispatchMetrics 把累计指标快照写入 ops_system_metrics（metric_type=channel_dispatch）。
// 由 collector.go 的 CollectSystemMetrics 每分钟 tick 调用；无调度活动时跳过。
func flushDispatchMetrics(ts time.Time) {
	snapshot := GetDispatchMetrics()
	if snapshot == nil {
		return
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		g.Log().Warningf(gctx.New(), "marshal dispatch metrics failed: %v", err)
		return
	}
	metricsWriter.Write(gctx.New(), do.OpsSystemMetrics{
		MetricType:  "channel_dispatch",
		MetricData:  string(payload),
		CollectedAt: gtime.NewFromTime(ts),
	})
}
