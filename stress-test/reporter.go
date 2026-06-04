package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics 压测指标收集器
type Metrics struct {
	mu sync.Mutex

	// 计数器（原子操作）
	TotalRequests   atomic.Int64 // 总请求数
	SuccessRequests atomic.Int64 // 成功数
	FailedRequests  atomic.Int64 // 失败数
	TimeoutRequests atomic.Int64 // 超时数

	// 流式相关
	FirstTokenTimes []time.Duration // 首个 token 延迟（TTFB）
	TotalTokens     atomic.Int64    // 总生成 token 数

	// 视频任务相关
	TasksSubmitted atomic.Int64 // 提交成功数
	TasksCompleted atomic.Int64 // 完成数
	TasksFailed    atomic.Int64 // 任务失败数

	// 延迟分布
	latencies []time.Duration

	// HTTP 状态码分布
	statusCodes map[int]int64

	// 错误分类
	errors map[string]int64

	// 时间窗口统计（用于实时 QPS 计算）
	requestTimeline []time.Time

	// 开始时间
	startTime time.Time
}

func NewMetrics() *Metrics {
	return &Metrics{
		statusCodes:     make(map[int]int64),
		errors:          make(map[string]int64),
		FirstTokenTimes: make([]time.Duration, 0),
		latencies:       make([]time.Duration, 0),
	}
}

// RecordRequest 记录一次请求
func (m *Metrics) RecordRequest(latency time.Duration, statusCode int, err error, firstToken time.Duration, tokens int64) {
	m.TotalRequests.Add(1)

	m.mu.Lock()
	m.latencies = append(m.latencies, latency)
	m.requestTimeline = append(m.requestTimeline, time.Now())
	if statusCode > 0 {
		m.statusCodes[statusCode]++
	}
	m.mu.Unlock()

	if firstToken > 0 {
		m.mu.Lock()
		m.FirstTokenTimes = append(m.FirstTokenTimes, firstToken)
		m.mu.Unlock()
	}

	if tokens > 0 {
		m.TotalTokens.Add(tokens)
	}

	if err != nil {
		m.FailedRequests.Add(1)
		m.mu.Lock()
		errMsg := err.Error()
		if len(errMsg) > 80 {
			errMsg = errMsg[:80]
		}
		m.errors[errMsg]++
		m.mu.Unlock()
	} else {
		m.SuccessRequests.Add(1)
	}
}

// RecordTimeout 记录超时
func (m *Metrics) RecordTimeout() {
	m.TotalRequests.Add(1)
	m.TimeoutRequests.Add(1)
	m.FailedRequests.Add(1)
}

// CurrentQPS 计算当前 QPS（滑动窗口 10 秒）
func (m *Metrics) CurrentQPS() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-10 * time.Second)
	count := 0
	for _, t := range m.requestTimeline {
		if t.After(windowStart) {
			count++
		}
	}
	return float64(count) / 10.0
}

// GetLatencyPercentiles 计算延迟百分位
func (m *Metrics) GetLatencyPercentiles() map[string]time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.latencies) == 0 {
		return nil
	}

	sorted := make([]time.Duration, len(m.latencies))
	copy(sorted, m.latencies)
	slices.Sort(sorted)

	result := map[string]time.Duration{
		"min": sorted[0],
		"p50": sorted[len(sorted)*50/100],
		"p90": sorted[len(sorted)*90/100],
		"p95": sorted[len(sorted)*95/100],
		"p99": sorted[len(sorted)*99/100],
		"max": sorted[len(sorted)-1],
		"avg": avgDuration(sorted),
	}

	return result
}

// GetTTFTPecentiles 计算首 token 延迟百分位
func (m *Metrics) GetTTFTPecentiles() map[string]time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.FirstTokenTimes) == 0 {
		return nil
	}

	sorted := make([]time.Duration, len(m.FirstTokenTimes))
	copy(sorted, m.FirstTokenTimes)
	slices.Sort(sorted)

	return map[string]time.Duration{
		"min": sorted[0],
		"p50": sorted[len(sorted)*50/100],
		"p90": sorted[len(sorted)*90/100],
		"p95": sorted[len(sorted)*95/100],
		"max": sorted[len(sorted)-1],
		"avg": avgDuration(sorted),
	}
}

// PrintReport 打印最终报告
func (m *Metrics) PrintReport(cfg *Config, elapsed time.Duration) {
	total := m.TotalRequests.Load()
	success := m.SuccessRequests.Load()
	failed := m.FailedRequests.Load()
	timedOut := m.TimeoutRequests.Load()

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    压力测试报告                             ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  运行时长:     %-44s ║\n", elapsed.Round(time.Millisecond))
	fmt.Printf("║  总请求数:     %-44d ║\n", total)
	fmt.Printf("║  成功:         %-44d ║\n", success)
	fmt.Printf("║  失败:         %-44d ║\n", failed)
	fmt.Printf("║  超时:         %-44d ║\n", timedOut)

	if total > 0 {
		fmt.Printf("║  成功率:       %-44.2f%% ║\n", float64(success)/float64(total)*100)
		fmt.Printf("║  平均 QPS:     %-44.2f ║\n", float64(total)/elapsed.Seconds())
	}

	// 延迟分布
	pct := m.GetLatencyPercentiles()
	if pct != nil {
		fmt.Println("╠══════════════════════════════════════════════════════════════╣")
		fmt.Println("║  响应延迟:                                                  ║")
		fmt.Printf("║    Min:    %-48s ║\n", pct["min"])
		fmt.Printf("║    Avg:    %-48s ║\n", pct["avg"])
		fmt.Printf("║    P50:    %-48s ║\n", pct["p50"])
		fmt.Printf("║    P90:    %-48s ║\n", pct["p90"])
		fmt.Printf("║    P95:    %-48s ║\n", pct["p95"])
		fmt.Printf("║    P99:    %-48s ║\n", pct["p99"])
		fmt.Printf("║    Max:    %-48s ║\n", pct["max"])
	}

	// 首 token 延迟
	ttfb := m.GetTTFTPecentiles()
	if ttfb != nil {
		fmt.Println("╠══════════════════════════════════════════════════════════════╣")
		fmt.Println("║  首 Token 延迟 (TTFB):                                     ║")
		fmt.Printf("║    P50:    %-48s ║\n", ttfb["p50"])
		fmt.Printf("║    P90:    %-48s ║\n", ttfb["p90"])
		fmt.Printf("║    P95:    %-48s ║\n", ttfb["p95"])
		fmt.Printf("║    Max:    %-48s ║\n", ttfb["max"])
	}

	// Token 吞吐
	totalTokens := m.TotalTokens.Load()
	if totalTokens > 0 {
		fmt.Println("╠══════════════════════════════════════════════════════════════╣")
		fmt.Printf("║  Token 吞吐:  %-44d ║\n", totalTokens)
		fmt.Printf("║  Tokens/s:    %-44.2f ║\n", float64(totalTokens)/elapsed.Seconds())
	}

	// 视频任务统计
	tasksSub := m.TasksSubmitted.Load()
	if tasksSub > 0 {
		fmt.Println("╠══════════════════════════════════════════════════════════════╣")
		fmt.Println("║  异步任务:                                                  ║")
		fmt.Printf("║    提交成功: %-48d ║\n", tasksSub)
		fmt.Printf("║    完成:     %-48d ║\n", m.TasksCompleted.Load())
		fmt.Printf("║    失败:     %-48d ║\n", m.TasksFailed.Load())
	}

	// HTTP 状态码
	m.mu.Lock()
	if len(m.statusCodes) > 0 {
		fmt.Println("╠══════════════════════════════════════════════════════════════╣")
		fmt.Println("║  状态码分布:                                                ║")
		for code, count := range m.statusCodes {
			fmt.Printf("║    %d: %-52d ║\n", code, count)
		}
	}

	// Top 错误
	if len(m.errors) > 0 {
		fmt.Println("╠══════════════════════════════════════════════════════════════╣")
		fmt.Println("║  Top 错误:                                                  ║")
		type errEntry struct {
			msg   string
			count int64
		}
		var entries []errEntry
		for msg, count := range m.errors {
			entries = append(entries, errEntry{msg, count})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].count > entries[j].count }) // 按数量倒序，不需要 slices.Sort
		for i, e := range entries {
			if i >= 5 {
				break
			}
			line := fmt.Sprintf("%s (×%d)", truncate(e.msg, 44), e.count)
			fmt.Printf("║    %-56s ║\n", line)
		}
	}
	m.mu.Unlock()

	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
}

// SaveJSON 保存 JSON 报告
func (m *Metrics) SaveJSON(cfg *Config, elapsed time.Duration) error {
	os.MkdirAll(cfg.OutputDir, 0755)

	total := m.TotalRequests.Load()
	report := map[string]any{
		"timestamp":    time.Now().Format(time.RFC3339),
		"scene":        cfg.Provider,
		"model":        cfg.Model,
		"concurrency":  cfg.Concurrency,
		"duration_sec": elapsed.Seconds(),
		"summary": map[string]any{
			"total_requests":  total,
			"success":         m.SuccessRequests.Load(),
			"failed":          m.FailedRequests.Load(),
			"timeout":         m.TimeoutRequests.Load(),
			"success_rate":    0.0,
			"avg_qps":         0.0,
			"total_tokens":    m.TotalTokens.Load(),
			"tokens_per_sec":  0.0,
			"tasks_submitted": m.TasksSubmitted.Load(),
			"tasks_completed": m.TasksCompleted.Load(),
		},
	}
	if total > 0 {
		report["summary"].(map[string]any)["success_rate"] = float64(m.SuccessRequests.Load()) / float64(total) * 100
		report["summary"].(map[string]any)["avg_qps"] = float64(total) / elapsed.Seconds()
	}
	if m.TotalTokens.Load() > 0 {
		report["summary"].(map[string]any)["tokens_per_sec"] = float64(m.TotalTokens.Load()) / elapsed.Seconds()
	}

	pct := m.GetLatencyPercentiles()
	if pct != nil {
		report["latency"] = map[string]string{
			"min": pct["min"].String(), "avg": pct["avg"].String(),
			"p50": pct["p50"].String(), "p90": pct["p90"].String(),
			"p95": pct["p95"].String(), "p99": pct["p99"].String(),
			"max": pct["max"].String(),
		}
	}

	ttfb := m.GetTTFTPecentiles()
	if ttfb != nil {
		report["ttfb"] = map[string]string{
			"p50": ttfb["p50"].String(), "p90": ttfb["p90"].String(),
			"p95": ttfb["p95"].String(), "max": ttfb["max"].String(),
		}
	}

	filename := filepath.Join(cfg.OutputDir,
		fmt.Sprintf("report_%s_%s_%d.json",
			cfg.Provider,
			time.Now().Format("20060102_150405"),
			cfg.Concurrency,
		))

	data, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return err
	}

	fmt.Printf("\n📁 报告已保存: %s\n", filename)
	return nil
}

func avgDuration(durations []time.Duration) time.Duration {
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return sum / time.Duration(len(durations))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
