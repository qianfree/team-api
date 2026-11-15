package jsplugin

import (
	"sync"
	"sync/atomic"
	"time"
)

// 内存运行指标（自进程启动累计，重启归零——钩子调用频次高，落库成本不值得；
// 任务量与产物转存量等业务指标走 DB 统计，由指标 API 合流）。操作审计
//（上传/激活/启停）由 OperationLog 中间件覆盖，不在此重复。

// HookMetrics 单钩子累计指标。
type HookMetrics struct {
	Calls     uint64
	Errors    uint64
	TotalNs   uint64 // 累计耗时（纳秒）——平均耗时 = TotalNs / Calls
	LastErr   string
	LastErrAt time.Time
}

// PluginMetricsSnapshot 单插件指标快照（含派生值，供 API 直接序列化）。
type PluginMetricsSnapshot struct {
	Key           string                      `json:"key"`
	InService     bool                        `json:"in_service"`
	LiveInstances int                         `json:"live_instances"`
	TotalCalls    uint64                      `json:"total_calls"`
	TotalErrors   uint64                      `json:"total_errors"`
	AvgMs         float64                     `json:"avg_ms"` // 全钩子平均耗时
	Hooks         map[string]*HookMetricsView `json:"hooks"`
}

// HookMetricsView 钩子指标视图（含错误率与平均耗时派生值）。
type HookMetricsView struct {
	Calls     uint64  `json:"calls"`
	Errors    uint64  `json:"errors"`
	ErrorRate float64 `json:"error_rate"`
	AvgMs     float64 `json:"avg_ms"`
	LastErr   string  `json:"last_err,omitempty"`
	LastErrAt string  `json:"last_err_at,omitempty"`
}

type pluginMetrics struct {
	hooks sync.Map // hook name → *hookMetricsAtomic
}

type hookMetricsAtomic struct {
	calls     atomic.Uint64
	errors    atomic.Uint64
	totalNs   atomic.Uint64
	lastErr   atomic.Pointer[string]
	lastErrAt atomic.Int64 // unix nano
}

var (
	metricsMu  sync.RWMutex
	metricsReg = map[string]*pluginMetrics{}
	startedAt  = time.Now()
)

// MetricsSince 指标累计起点（进程启动时间）。
func MetricsSince() time.Time { return startedAt }

// recordHookMetrics 采集一次钩子调用（RecordHookCall 内部调用）。
func recordHookMetrics(pluginKey, hook string, dur time.Duration, callErr error) {
	metricsMu.RLock()
	pm, ok := metricsReg[pluginKey]
	metricsMu.RUnlock()
	if !ok {
		metricsMu.Lock()
		pm, ok = metricsReg[pluginKey]
		if !ok {
			pm = &pluginMetrics{}
			metricsReg[pluginKey] = pm
		}
		metricsMu.Unlock()
	}
	hmAny, _ := pm.hooks.LoadOrStore(hook, &hookMetricsAtomic{})
	hm := hmAny.(*hookMetricsAtomic)
	hm.calls.Add(1)
	hm.totalNs.Add(uint64(dur))
	if callErr != nil {
		hm.errors.Add(1)
		msg := callErr.Error()
		if len(msg) > 256 {
			msg = msg[:256]
		}
		hm.lastErr.Store(&msg)
		hm.lastErrAt.Store(time.Now().UnixNano())
	}
}

// MetricsSnapshot 全部插件指标快照（含 generation 派生状态与实例数）。
func MetricsSnapshot() []PluginMetricsSnapshot {
	gen := CurrentGeneration()
	metricsMu.RLock()
	defer metricsMu.RUnlock()

	out := make([]PluginMetricsSnapshot, 0, len(metricsReg))
	for key, pm := range metricsReg {
		snap := PluginMetricsSnapshot{
			Key:   key,
			Hooks: map[string]*HookMetricsView{},
		}
		if plugin, ok := gen.Plugin(key); ok {
			snap.InService = true
			snap.LiveInstances = plugin.Pool.LiveInstances()
		}
		pm.hooks.Range(func(name, v any) bool {
			hm := v.(*hookMetricsAtomic)
			view := &HookMetricsView{Calls: hm.calls.Load(), Errors: hm.errors.Load()}
			if view.Calls > 0 {
				view.AvgMs = float64(hm.totalNs.Load()) / float64(view.Calls) / 1e6
				view.ErrorRate = float64(view.Errors) / float64(view.Calls)
			}
			if msg := hm.lastErr.Load(); msg != nil {
				view.LastErr = *msg
				if at := hm.lastErrAt.Load(); at > 0 {
					view.LastErrAt = time.Unix(0, at).Format("2006-01-02 15:04:05")
				}
			}
			snap.Hooks[name.(string)] = view
			snap.TotalCalls += view.Calls
			snap.TotalErrors += view.Errors
			return true
		})
		if snap.TotalCalls > 0 {
			// 全钩子平均：以总耗时/总次数重算（非各钩子平均的简单平均）
			var totalNs uint64
			pm.hooks.Range(func(_, v any) bool {
				totalNs += v.(*hookMetricsAtomic).totalNs.Load()
				return true
			})
			snap.AvgMs = float64(totalNs) / float64(snap.TotalCalls) / 1e6
		}
		out = append(out, snap)
	}
	return out
}
