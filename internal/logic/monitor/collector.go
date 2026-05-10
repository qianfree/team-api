package monitor

import (
	"context"
	"encoding/json"
	"github.com/qianfree/team-api/internal/dao"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	psnet "github.com/shirou/gopsutil/v3/net"

	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
)

// SystemMetricsSnapshot represents a single point-in-time collection of system metrics.
type SystemMetricsSnapshot struct {
	Timestamp time.Time      `json:"timestamp"`
	CPU       CPUMetrics     `json:"cpu"`
	Memory    MemoryMetrics  `json:"memory"`
	Disk      DiskMetrics    `json:"disk"`
	Network   NetworkMetrics `json:"network"`
	Runtime   RuntimeMetrics `json:"runtime"`
}

// CPUMetrics holds CPU utilization data.
type CPUMetrics struct {
	Percent     float64 `json:"percent"`
	CoreCount   int     `json:"core_count"`
	UserPercent float64 `json:"user_percent"`
	SysPercent  float64 `json:"sys_percent"`
}

// MemoryMetrics holds memory utilization data.
type MemoryMetrics struct {
	TotalMB     float64 `json:"total_mb"`
	UsedMB      float64 `json:"used_mb"`
	AvailableMB float64 `json:"available_mb"`
	UsedPercent float64 `json:"used_percent"`
}

// DiskMetrics holds disk utilization data.
type DiskMetrics struct {
	TotalGB     float64 `json:"total_gb"`
	UsedGB      float64 `json:"used_gb"`
	FreeGB      float64 `json:"free_gb"`
	UsedPercent float64 `json:"used_percent"`
}

// NetworkMetrics holds network throughput data.
type NetworkMetrics struct {
	BytesSentPerSec float64 `json:"bytes_sent_per_sec"`
	BytesRecvPerSec float64 `json:"bytes_recv_per_sec"`
}

// RuntimeMetrics holds Go runtime data.
type RuntimeMetrics struct {
	GoroutineCount int     `json:"goroutine_count"`
	HeapAllocMB    float64 `json:"heap_alloc_mb"`
	HeapSysMB      float64 `json:"heap_sys_mb"`
	StackInUseMB   float64 `json:"stack_in_use_mb"`
	GCPauseNs      float64 `json:"gc_pause_ns"`
	NumGC          uint32  `json:"num_gc"`
}

// ringBuffer is a fixed-size circular buffer for metrics snapshots.
type ringBuffer struct {
	snapshots []SystemMetricsSnapshot
	capacity  int
	head      int
	size      int
	mu        sync.RWMutex
}

func newRingBuffer(capacity int) *ringBuffer {
	return &ringBuffer{
		snapshots: make([]SystemMetricsSnapshot, capacity),
		capacity:  capacity,
	}
}

func (rb *ringBuffer) Push(s SystemMetricsSnapshot) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.snapshots[rb.head] = s
	rb.head = (rb.head + 1) % rb.capacity
	if rb.size < rb.capacity {
		rb.size++
	}
}

func (rb *ringBuffer) Latest() *SystemMetricsSnapshot {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	if rb.size == 0 {
		return nil
	}
	idx := (rb.head - 1 + rb.capacity) % rb.capacity
	s := rb.snapshots[idx]
	return &s
}

func (rb *ringBuffer) History(since time.Time) []SystemMetricsSnapshot {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	if rb.size == 0 {
		return nil
	}
	var result []SystemMetricsSnapshot
	for i := 0; i < rb.size; i++ {
		idx := (rb.head - 1 - i + rb.capacity) % rb.capacity
		s := rb.snapshots[idx]
		if s.Timestamp.Before(since) {
			break
		}
		result = append(result, s)
	}
	// Reverse to chronological order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

var (
	systemBuffer  *ringBuffer
	metricsWriter *common.BatchWriter
	lastNetStats  []psnet.IOCountersStat
	lastNetTime   time.Time
	startupTime   time.Time
)

// GoRuntimeInfo holds comprehensive Go runtime information.
type GoRuntimeInfo struct {
	GoVersion    string      `json:"go_version"`
	NumCPU       int         `json:"num_cpu"`
	NumGoroutine int         `json:"num_goroutine"`
	NumCgoCall   int64       `json:"num_cgo_call"`
	Uptime       string      `json:"uptime"`
	UptimeSec    int64       `json:"uptime_sec"`
	Memory       GoMemInfo   `json:"memory"`
	GC           GoGCInfo    `json:"gc"`
	Build        GoBuildInfo `json:"build"`
}

// GoMemInfo holds detailed Go memory allocator stats.
type GoMemInfo struct {
	AllocMB        float64 `json:"alloc_mb"`
	TotalAllocMB   float64 `json:"total_alloc_mb"`
	SysMB          float64 `json:"sys_mb"`
	HeapAllocMB    float64 `json:"heap_alloc_mb"`
	HeapSysMB      float64 `json:"heap_sys_mb"`
	HeapIdleMB     float64 `json:"heap_idle_mb"`
	HeapInuseMB    float64 `json:"heap_inuse_mb"`
	HeapReleasedMB float64 `json:"heap_released_mb"`
	HeapObjects    uint64  `json:"heap_objects"`
	StackInuseMB   float64 `json:"stack_inuse_mb"`
	StackSysMB     float64 `json:"stack_sys_mb"`
	MSpanInuseMB   float64 `json:"mspan_inuse_mb"`
	MSpanSysMB     float64 `json:"mspan_sys_mb"`
	MCacheInuseMB  float64 `json:"mcache_inuse_mb"`
	MCacheSysMB    float64 `json:"mcache_sys_mb"`
	GCSysMB        float64 `json:"gc_sys_mb"`
	OtherSysMB     float64 `json:"other_sys_mb"`
}

// GoGCInfo holds garbage collector statistics.
type GoGCInfo struct {
	NumGC         uint32    `json:"num_gc"`
	NumForcedGC   uint32    `json:"num_forced_gc"`
	GCCPUFraction float64   `json:"gc_cpu_fraction"`
	NextGCMB      float64   `json:"next_gc_mb"`
	PauseTotalMs  float64   `json:"pause_total_ms"`
	PauseLastMs   float64   `json:"pause_last_ms"`
	RecentPauseMs []float64 `json:"recent_pause_ms"`
	MemoryLimitMB float64   `json:"memory_limit_mb"`
}

// GoBuildInfo holds build metadata.
type GoBuildInfo struct {
	Path      string `json:"path"`
	Main      string `json:"main"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

// GetGoRuntimeInfo reads comprehensive runtime information directly from the Go runtime.
func GetGoRuntimeInfo() GoRuntimeInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	mb := float64(1024 * 1024)
	info := GoRuntimeInfo{
		GoVersion:    runtime.Version(),
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
		NumCgoCall:   runtime.NumCgoCall(),
		Uptime:       time.Since(startupTime).Round(time.Second).String(),
		UptimeSec:    int64(time.Since(startupTime).Seconds()),
		Memory: GoMemInfo{
			AllocMB:        float64(m.Alloc) / mb,
			TotalAllocMB:   float64(m.TotalAlloc) / mb,
			SysMB:          float64(m.Sys) / mb,
			HeapAllocMB:    float64(m.HeapAlloc) / mb,
			HeapSysMB:      float64(m.HeapSys) / mb,
			HeapIdleMB:     float64(m.HeapIdle) / mb,
			HeapInuseMB:    float64(m.HeapInuse) / mb,
			HeapReleasedMB: float64(m.HeapReleased) / mb,
			HeapObjects:    m.HeapObjects,
			StackInuseMB:   float64(m.StackInuse) / mb,
			StackSysMB:     float64(m.StackSys) / mb,
			MSpanInuseMB:   float64(m.MSpanInuse) / mb,
			MSpanSysMB:     float64(m.MSpanSys) / mb,
			MCacheInuseMB:  float64(m.MCacheInuse) / mb,
			MCacheSysMB:    float64(m.MCacheSys) / mb,
			GCSysMB:        float64(m.GCSys) / mb,
			OtherSysMB:     float64(m.OtherSys) / mb,
		},
		GC: GoGCInfo{
			NumGC:         m.NumGC,
			NumForcedGC:   m.NumForcedGC,
			GCCPUFraction: m.GCCPUFraction,
			NextGCMB:      float64(m.NextGC) / mb,
			PauseTotalMs:  float64(m.PauseTotalNs) / 1e6,
			MemoryLimitMB: float64(debug.SetMemoryLimit(-1)) / mb,
		},
	}

	if m.NumGC > 0 {
		lastIdx := (m.NumGC + 255) % 256
		info.GC.PauseLastMs = float64(m.PauseNs[lastIdx]) / 1e6

		n := m.NumGC
		if n > 10 {
			n = 10
		}
		recent := make([]float64, 0, n)
		for i := uint32(0); i < n; i++ {
			idx := (m.NumGC - i + 255) % 256
			recent = append(recent, float64(m.PauseNs[idx])/1e6)
		}
		info.GC.RecentPauseMs = recent
	}

	if bi, ok := debug.ReadBuildInfo(); ok {
		info.Build = GoBuildInfo{
			Path: bi.Path,
		}
		if bi.Main.Path != "" {
			info.Build.Main = bi.Main.Path
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if len(s.Value) > 8 {
					info.Build.Commit = s.Value[:8]
				} else {
					info.Build.Commit = s.Value
				}
			case "vcs.time":
				info.Build.BuildTime = s.Value
			}
		}
	}

	return info
}

// InitCollector initializes the metrics collector.
func InitCollector(ctx context.Context) {
	systemBuffer = newRingBuffer(360) // 1 hour at 10s intervals
	metricsWriter = common.NewBatchWriter("ops_system_metrics", 32, 128)
	startupTime = time.Now()
	g.Log().Info(ctx, "monitor collector initialized")
}

// IsWarmedUp returns true if the system has been running for at least 2 minutes.
func IsWarmedUp() bool {
	return time.Since(startupTime) >= 2*time.Minute
}

// CollectSystemMetrics collects a full set of system metrics.
func CollectSystemMetrics(ctx context.Context) error {
	snapshot := SystemMetricsSnapshot{Timestamp: time.Now()}

	// CPU
	if percents, err := cpu.Percent(0, false); err == nil && len(percents) > 0 {
		snapshot.CPU.Percent = percents[0]
	}
	if counts, err := cpu.Counts(true); err == nil {
		snapshot.CPU.CoreCount = counts
	}
	if times, err := cpu.Times(false); err == nil && len(times) > 0 {
		total := times[0].User + times[0].System + times[0].Idle + times[0].Nice +
			times[0].Iowait + times[0].Irq + times[0].Softirq + times[0].Steal +
			times[0].Guest + times[0].GuestNice
		if total > 0 {
			snapshot.CPU.UserPercent = (times[0].User / total) * 100
			snapshot.CPU.SysPercent = (times[0].System / total) * 100
		}
	}

	// Memory
	if vm, err := mem.VirtualMemory(); err == nil {
		snapshot.Memory.TotalMB = float64(vm.Total) / 1024 / 1024
		snapshot.Memory.UsedMB = float64(vm.Used) / 1024 / 1024
		snapshot.Memory.AvailableMB = float64(vm.Available) / 1024 / 1024
		snapshot.Memory.UsedPercent = vm.UsedPercent
	}

	// Disk
	if du, err := disk.Usage("/"); err == nil {
		snapshot.Disk.TotalGB = float64(du.Total) / 1024 / 1024 / 1024
		snapshot.Disk.UsedGB = float64(du.Used) / 1024 / 1024 / 1024
		snapshot.Disk.FreeGB = float64(du.Free) / 1024 / 1024 / 1024
		snapshot.Disk.UsedPercent = du.UsedPercent
	}

	// Network (calculate per-second rates)
	if ioStats, err := psnet.IOCounters(false); err == nil && len(ioStats) > 0 {
		now := time.Now()
		if lastNetTime.IsZero() {
			snapshot.Network.BytesSentPerSec = 0
			snapshot.Network.BytesRecvPerSec = 0
		} else {
			elapsed := now.Sub(lastNetTime).Seconds()
			if elapsed > 0 {
				snapshot.Network.BytesSentPerSec = float64(ioStats[0].BytesSent-lastNetStats[0].BytesSent) / elapsed
				snapshot.Network.BytesRecvPerSec = float64(ioStats[0].BytesRecv-lastNetStats[0].BytesRecv) / elapsed
			}
		}
		lastNetStats = ioStats
		lastNetTime = now
	}

	// Go runtime
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	snapshot.Runtime.GoroutineCount = runtime.NumGoroutine()
	snapshot.Runtime.HeapAllocMB = float64(memStats.HeapAlloc) / 1024 / 1024
	snapshot.Runtime.HeapSysMB = float64(memStats.HeapSys) / 1024 / 1024
	snapshot.Runtime.StackInUseMB = float64(memStats.StackInuse) / 1024 / 1024
	snapshot.Runtime.NumGC = memStats.NumGC
	if memStats.NumGC > 0 {
		lastGC := memStats.PauseNs[(memStats.NumGC+255)%256]
		snapshot.Runtime.GCPauseNs = float64(lastGC)
	}

	// Store in ring buffer
	systemBuffer.Push(snapshot)

	// Write to batch writer for DB persistence
	writeMetricsToDB(snapshot)

	return nil
}

func writeMetricsToDB(s SystemMetricsSnapshot) {
	// CPU metric
	if s.CPU.Percent > 0 {
		data, _ := json.Marshal(s.CPU)
		metricsWriter.Write(gctx.New(), do.OpsSystemMetrics{
			MetricType:  "cpu",
			MetricData:  string(data),
			CollectedAt: gtime.NewFromTime(s.Timestamp),
		})
	}

	// Memory metric
	if s.Memory.TotalMB > 0 {
		data, _ := json.Marshal(s.Memory)
		metricsWriter.Write(gctx.New(), do.OpsSystemMetrics{
			MetricType:  "memory",
			MetricData:  string(data),
			CollectedAt: gtime.NewFromTime(s.Timestamp),
		})
	}

	// Disk metric
	if s.Disk.TotalGB > 0 {
		data, _ := json.Marshal(s.Disk)
		metricsWriter.Write(gctx.New(), do.OpsSystemMetrics{
			MetricType:  "disk",
			MetricData:  string(data),
			CollectedAt: gtime.NewFromTime(s.Timestamp),
		})
	}

	// Runtime metric
	data, _ := json.Marshal(s.Runtime)
	metricsWriter.Write(gctx.New(), do.OpsSystemMetrics{
		MetricType:  "runtime",
		MetricData:  string(data),
		CollectedAt: gtime.NewFromTime(s.Timestamp),
	})
}

// GetLatestMetrics returns the most recent metrics snapshot.
func GetLatestMetrics() *SystemMetricsSnapshot {
	return systemBuffer.Latest()
}

// GetMetricsHistory returns all snapshots within the given duration.
func GetMetricsHistory(dur time.Duration) []SystemMetricsSnapshot {
	return systemBuffer.History(time.Now().Add(-dur))
}

// GetCPUPercent returns the current CPU usage percentage.
func GetCPUPercent() float64 {
	if s := systemBuffer.Latest(); s != nil {
		return s.CPU.Percent
	}
	return 0
}

// GetMemoryPercent returns the current memory usage percentage.
func GetMemoryPercent() float64 {
	if s := systemBuffer.Latest(); s != nil {
		return s.Memory.UsedPercent
	}
	return 0
}

// GetDiskPercent returns the current disk usage percentage.
func GetDiskPercent() float64 {
	if s := systemBuffer.Latest(); s != nil {
		return s.Disk.UsedPercent
	}
	return 0
}

// CleanupOldMetrics removes metrics older than 30 days.
func CleanupOldMetrics(ctx context.Context) error {
	cutoff := time.Now().AddDate(0, 0, -30)
	_, err := dao.OpsSystemMetrics.Ctx(ctx).
		Where("collected_at < ?", cutoff).
		Delete()
	if err != nil {
		g.Log().Errorf(ctx, "cleanup old metrics failed: %v", err)
		return err
	}
	return nil
}
