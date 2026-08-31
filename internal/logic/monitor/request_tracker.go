package monitor

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

const (
	maxActiveRequestsInResponse = 200

	// 采样周期与保留窗口：后台采样器固定 5s 采一次，环形缓冲保留 120 个点 = 10 分钟。
	samplerInterval = 5 * time.Second
	historyCapacity = 120
	historyWindow   = time.Duration(historyCapacity) * samplerInterval

	// 悬挂条目清扫阈值：普通 relay 请求受 HTTP 超时约束，异步任务由轮询器 30min 超时兜底。
	// 超过该时长仍留在 map 中的一定是漏调 Unregister（如任务被另一实例的轮询器终结）。
	staleRequestTTL = 30 * time.Minute
	staleTaskTTL    = 60 * time.Minute
)

// TrackedRequest represents a single active relay request.
type TrackedRequest struct {
	RequestID   string    `json:"request_id"`
	TenantID    int64     `json:"tenant_id"`
	UserID      int64     `json:"user_id"`
	ProjectID   int64     `json:"project_id"`
	ModelName   string    `json:"model_name"`
	ChannelID   int64     `json:"channel_id"`
	ChannelName string    `json:"channel_name"`
	IsStream    bool      `json:"is_stream"`
	StartTime   time.Time `json:"start_time"`
	Path        string    `json:"path"`
	IsAsyncTask bool      `json:"is_async_task"` // 异步任务（视频/音乐/图像生成）
	TaskID      string    `json:"task_id,omitempty"`
}

// ConcurrencySnapshot is a point-in-time concurrency view.
type ConcurrencySnapshot struct {
	Timestamp          time.Time `json:"timestamp"`
	TotalActive        int       `json:"total_active"`
	StreamingActive    int       `json:"streaming_active"`
	NonStreamingActive int       `json:"non_streaming_active"`
}

// BandwidthSnapshot is a point-in-time bandwidth view.
type BandwidthSnapshot struct {
	Timestamp      time.Time `json:"timestamp"`
	BytesInPerSec  float64   `json:"bytes_in_per_sec"`
	BytesOutPerSec float64   `json:"bytes_out_per_sec"`
}

// RealtimeData is the combined response for /monitor/realtime.
type RealtimeData struct {
	Concurrency      ConcurrencySnapshot   `json:"concurrency"`
	Bandwidth        BandwidthSnapshot     `json:"bandwidth"`
	Runtime          RuntimeMetrics        `json:"runtime"`
	GoRuntime        GoRuntimeInfo         `json:"go_runtime"`
	History          []ConcurrencySnapshot `json:"history"`
	BandwidthHistory []BandwidthSnapshot   `json:"bandwidth_history"`
	ActiveRequests   []TrackedRequest      `json:"active_requests"`
	ByModel          map[string]int        `json:"by_model"`
	ByChannel        map[string]int        `json:"by_channel"`
	ByTenant         map[int64]int         `json:"by_tenant"`
	SyncImagePool    SyncImagePoolSnapshot `json:"sync_image_pool"`
	Rpm              int64                 `json:"rpm"` // 最近60秒滑动窗口请求数（全平台，由 Realtime 方法填充）
	Tpm              int64                 `json:"tpm"` // 最近60秒滑动窗口token数（全平台，由 Realtime 方法填充）
}

// snapshotRing is a fixed-size circular buffer for arbitrary snapshot types.
type snapshotRing[T any] struct {
	items []T
	cap   int
	head  int
	size  int
	mu    sync.RWMutex
}

func newSnapshotRing[T any](capacity int) *snapshotRing[T] {
	return &snapshotRing[T]{
		items: make([]T, capacity),
		cap:   capacity,
	}
}

func (r *snapshotRing[T]) Push(v T) {
	r.mu.Lock()
	r.items[r.head] = v
	r.head = (r.head + 1) % r.cap
	if r.size < r.cap {
		r.size++
	}
	r.mu.Unlock()
}

func (r *snapshotRing[T]) All() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.size == 0 {
		return nil
	}
	result := make([]T, r.size)
	for i := 0; i < r.size; i++ {
		idx := (r.head - r.size + i + r.cap) % r.cap
		result[i] = r.items[idx]
	}
	return result
}

// Last 返回最新一个样本（无样本时 ok=false）。
func (r *snapshotRing[T]) Last() (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var zero T
	if r.size == 0 {
		return zero, false
	}
	return r.items[(r.head-1+r.cap)%r.cap], true
}

// filterRecent 裁掉早于 cutoff 的样本。环形缓冲按时间升序返回，命中首个在窗口内的点即可整体切片。
// 采样器正常运行时不会有过期点，此处是采样器停摆（如进程挂起）后的兜底，
// 避免把陈旧曲线当成当前流量画出来。
func filterRecent[T any](items []T, cutoff time.Time, at func(T) time.Time) []T {
	for i := range items {
		if !at(items[i]).Before(cutoff) {
			return items[i:]
		}
	}
	return nil
}

// requestTracker is the global singleton for tracking active requests.
type requestTracker struct {
	mu             sync.RWMutex
	activeRequests map[string]*TrackedRequest

	totalActive     atomic.Int64
	streamingActive atomic.Int64

	bytesIn      atomic.Int64
	bytesOut     atomic.Int64
	bandwidthMu  sync.Mutex
	lastBytesIn  int64
	lastBytesOut int64
	lastBwTime   time.Time

	concHistory *snapshotRing[ConcurrencySnapshot]
	bwHistory   *snapshotRing[BandwidthSnapshot]
}

var (
	tracker     *requestTracker
	trackerStop chan struct{}
)

// InitRequestTracker initializes the global request tracker and starts its sampler.
//
// 采样必须由后台定时器驱动、与管理页轮询解耦：早期实现只在 /monitor/realtime 被调用时
// 才往环形缓冲写点，页面关闭期间既不采样也不过期，再次打开时读到的仍是上一次会话遗留的
// 旧曲线（前端 x 轴只显示 mm:ss，看起来就像"刚刚还有并发/带宽"）。
func InitRequestTracker() {
	StopRequestTracker()

	t := &requestTracker{
		activeRequests: make(map[string]*TrackedRequest),
		concHistory:    newSnapshotRing[ConcurrencySnapshot](historyCapacity),
		bwHistory:      newSnapshotRing[BandwidthSnapshot](historyCapacity),
	}
	// 带宽基线置为启动时刻，避免首个样本把「进程启动至今累计字节」算成瞬时速率
	t.lastBwTime = time.Now()

	tracker = t
	trackerStop = make(chan struct{})
	go t.runSampler(trackerStop)
}

// StopRequestTracker stops the background sampler (idempotent).
func StopRequestTracker() {
	if trackerStop != nil {
		close(trackerStop)
		trackerStop = nil
	}
}

func (t *requestTracker) runSampler(stop chan struct{}) {
	ticker := time.NewTicker(samplerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			t.sample(now)
		}
	}
}

// sample 采一次并发 + 带宽快照。带宽差值只在这里计算（固定 5s 间隔），
// 不再由 HTTP 轮询触发——否则多个管理页同时打开会互相偷走增量，
// 两次轮询间隔极短时还会算出「字节数 / 几毫秒」的假尖峰。
func (t *requestTracker) sample(now time.Time) {
	total, streaming := t.sweepStale(now)
	t.concHistory.Push(ConcurrencySnapshot{
		Timestamp:          now,
		TotalActive:        total,
		StreamingActive:    streaming,
		NonStreamingActive: total - streaming,
	})
	t.bwHistory.Push(t.snapshotBandwidth(now))
}

// sweepStale 清扫超时仍未反注册的悬挂条目，并以 map 为唯一真相重算计数。
// 漏减场景确实存在：异步任务由轮询器终结，多实例部署下终结它的实例未必是当初受理提交的实例，
// 受理实例内存里的条目就再也没人删——表现为并发数永远回不到 0。
func (t *requestTracker) sweepStale(now time.Time) (total, streaming int) {
	var swept int

	t.mu.Lock()
	for key, req := range t.activeRequests {
		ttl := staleRequestTTL
		if req.IsAsyncTask {
			ttl = staleTaskTTL
		}
		if now.Sub(req.StartTime) > ttl {
			delete(t.activeRequests, key)
			swept++
			continue
		}
		total++
		if req.IsStream {
			streaming++
		}
	}
	t.mu.Unlock()

	// 计数以 map 为准回写，顺带修正任何漏减/多减造成的漂移
	t.totalActive.Store(int64(total))
	t.streamingActive.Store(int64(streaming))

	if swept > 0 {
		g.Log().Warningf(gctx.New(), "monitor: 清扫 %d 条悬挂的活跃请求记录（注册后未反注册）", swept)
	}
	return total, streaming
}

// RegisterRequest adds a new request to the tracker.
func RegisterRequest(req *TrackedRequest) {
	if tracker == nil {
		return
	}
	tracker.mu.Lock()
	tracker.activeRequests[req.RequestID] = req
	tracker.totalActive.Add(1)
	if req.IsStream {
		tracker.streamingActive.Add(1)
	}
	tracker.mu.Unlock()
}

// UnregisterRequest removes a completed request.
func UnregisterRequest(requestID string) {
	if tracker == nil {
		return
	}
	tracker.mu.Lock()
	if req, ok := tracker.activeRequests[requestID]; ok {
		delete(tracker.activeRequests, requestID)
		tracker.totalActive.Add(-1)
		if req.IsStream {
			tracker.streamingActive.Add(-1)
		}
	}
	tracker.mu.Unlock()
}

// GetTrackedRequest returns the tracked request by ID (for in-place updates like channel info).
func GetTrackedRequest(requestID string) *TrackedRequest {
	if tracker == nil {
		return nil
	}
	tracker.mu.RLock()
	defer tracker.mu.RUnlock()
	return tracker.activeRequests[requestID]
}

// SwitchToTaskID replaces the request ID key with a task ID for long-lived async tasks.
// The tracked request keeps its position in the active list but is now keyed by taskID.
func SwitchToTaskID(requestID, taskID string) {
	if tracker == nil {
		return
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if req, ok := tracker.activeRequests[requestID]; ok {
		delete(tracker.activeRequests, requestID)
		req.TaskID = taskID
		tracker.activeRequests[taskID] = req
	}
}

// UnregisterRequestByTaskID removes a completed async task by its task ID.
func UnregisterRequestByTaskID(taskID string) {
	if tracker == nil {
		return
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if req, ok := tracker.activeRequests[taskID]; ok {
		delete(tracker.activeRequests, taskID)
		tracker.totalActive.Add(-1)
		if req.IsStream {
			tracker.streamingActive.Add(-1)
		}
	}
}

// RecordBytesIn atomically accumulates inbound bytes.
func RecordBytesIn(n int) {
	if tracker == nil {
		return
	}
	tracker.bytesIn.Add(int64(n))
}

// RecordBytesOut atomically accumulates outbound bytes.
func RecordBytesOut(n int) {
	if tracker == nil {
		return
	}
	tracker.bytesOut.Add(int64(n))
}

// GetRealtimeData builds the combined realtime snapshot.
// 纯读：历史点与带宽速率均由后台采样器产出，此处不再推进任何采样状态，
// 因此多个管理页同时打开互不干扰，页面不打开时曲线也照常滚动过期。
func GetRealtimeData() *RealtimeData {
	if tracker == nil {
		return &RealtimeData{}
	}

	now := time.Now()
	cutoff := now.Add(-historyWindow)

	// 当前并发：取实时计数（比最近一个样本更新）
	total := int(tracker.totalActive.Load())
	streaming := int(tracker.streamingActive.Load())
	concSnap := ConcurrencySnapshot{
		Timestamp:          now,
		TotalActive:        total,
		StreamingActive:    streaming,
		NonStreamingActive: total - streaming,
	}

	// 当前带宽：取最近一个样本；样本过旧（采样器停摆）则视为 0，不展示陈旧速率
	bwSnap := BandwidthSnapshot{Timestamp: now}
	if last, ok := tracker.bwHistory.Last(); ok && !last.Timestamp.Before(now.Add(-2*samplerInterval)) {
		bwSnap = last
	}

	// Build breakdowns
	tracker.mu.RLock()
	byModel := make(map[string]int, len(tracker.activeRequests))
	byChannel := make(map[string]int, len(tracker.activeRequests))
	byTenant := make(map[int64]int, len(tracker.activeRequests))

	activeReqs := make([]TrackedRequest, 0, min(len(tracker.activeRequests), maxActiveRequestsInResponse))
	for _, req := range tracker.activeRequests {
		byModel[req.ModelName]++
		if req.ChannelName != "" {
			byChannel[req.ChannelName]++
		}
		byTenant[req.TenantID]++
		activeReqs = append(activeReqs, *req)
	}
	tracker.mu.RUnlock()

	// Runtime from existing collector
	var rt RuntimeMetrics
	if snap := GetLatestMetrics(); snap != nil {
		rt = snap.Runtime
	}

	return &RealtimeData{
		Concurrency: concSnap,
		Bandwidth:   bwSnap,
		Runtime:     rt,
		GoRuntime:   GetGoRuntimeInfo(),
		History: filterRecent(tracker.concHistory.All(), cutoff,
			func(s ConcurrencySnapshot) time.Time { return s.Timestamp }),
		BandwidthHistory: filterRecent(tracker.bwHistory.All(), cutoff,
			func(s BandwidthSnapshot) time.Time { return s.Timestamp }),
		ActiveRequests: activeReqs,
		ByModel:        byModel,
		ByChannel:      byChannel,
		ByTenant:       byTenant,
		SyncImagePool:  GetSyncImagePoolSnapshot(),
	}
}

func (t *requestTracker) snapshotBandwidth(now time.Time) BandwidthSnapshot {
	t.bandwidthMu.Lock()
	defer t.bandwidthMu.Unlock()

	currentIn := t.bytesIn.Load()
	currentOut := t.bytesOut.Load()

	snap := BandwidthSnapshot{Timestamp: now}

	if !t.lastBwTime.IsZero() && !now.After(t.lastBwTime) {
		return snap
	}
	if !t.lastBwTime.IsZero() {
		elapsed := now.Sub(t.lastBwTime).Seconds()
		snap.BytesInPerSec = float64(currentIn-t.lastBytesIn) / elapsed
		snap.BytesOutPerSec = float64(currentOut-t.lastBytesOut) / elapsed
	}

	t.lastBytesIn = currentIn
	t.lastBytesOut = currentOut
	t.lastBwTime = now

	return snap
}
