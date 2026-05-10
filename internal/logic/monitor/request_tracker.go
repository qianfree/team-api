package monitor

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxActiveRequestsInResponse = 200
	historyCapacity             = 100 // ~5 min at 3s polling
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

// requestTracker is the global singleton for tracking active requests.
type requestTracker struct {
	mu             sync.RWMutex
	activeRequests map[string]*TrackedRequest

	totalActive     atomic.Int64
	streamingActive atomic.Int64

	bytesIn      atomic.Int64
	bytesOut     atomic.Int64
	lastBytesIn  int64
	lastBytesOut int64
	lastBwTime   time.Time

	concHistory *snapshotRing[ConcurrencySnapshot]
	bwHistory   *snapshotRing[BandwidthSnapshot]
}

var tracker *requestTracker

// InitRequestTracker initializes the global request tracker.
func InitRequestTracker() {
	tracker = &requestTracker{
		activeRequests: make(map[string]*TrackedRequest),
		concHistory:    newSnapshotRing[ConcurrencySnapshot](historyCapacity),
		bwHistory:      newSnapshotRing[BandwidthSnapshot](historyCapacity),
	}
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
func GetRealtimeData() *RealtimeData {
	if tracker == nil {
		return &RealtimeData{}
	}

	now := time.Now()

	// Concurrency snapshot
	total := int(tracker.totalActive.Load())
	streaming := int(tracker.streamingActive.Load())
	concSnap := ConcurrencySnapshot{
		Timestamp:          now,
		TotalActive:        total,
		StreamingActive:    streaming,
		NonStreamingActive: total - streaming,
	}
	tracker.concHistory.Push(concSnap)

	// Bandwidth snapshot (delta between calls)
	bwSnap := tracker.snapshotBandwidth(now)
	tracker.bwHistory.Push(bwSnap)

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
		Concurrency:      concSnap,
		Bandwidth:        bwSnap,
		Runtime:          rt,
		GoRuntime:        GetGoRuntimeInfo(),
		History:          tracker.concHistory.All(),
		BandwidthHistory: tracker.bwHistory.All(),
		ActiveRequests:   activeReqs,
		ByModel:          byModel,
		ByChannel:        byChannel,
		ByTenant:         byTenant,
	}
}

func (t *requestTracker) snapshotBandwidth(now time.Time) BandwidthSnapshot {
	currentIn := t.bytesIn.Load()
	currentOut := t.bytesOut.Load()

	snap := BandwidthSnapshot{Timestamp: now}

	if !t.lastBwTime.IsZero() {
		elapsed := now.Sub(t.lastBwTime).Seconds()
		if elapsed > 0 {
			snap.BytesInPerSec = float64(currentIn-t.lastBytesIn) / elapsed
			snap.BytesOutPerSec = float64(currentOut-t.lastBytesOut) / elapsed
		}
	}

	t.lastBytesIn = currentIn
	t.lastBytesOut = currentOut
	t.lastBwTime = now

	return snap
}
