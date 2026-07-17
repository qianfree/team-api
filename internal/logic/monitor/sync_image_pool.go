package monitor

// SyncImagePoolSnapshot 是同步图片「异步化」worker 池的一次性状态快照。
// 由 internal/logic/task 在启动 worker 池时通过 RegisterSyncImagePoolProvider 注入取数函数，
// monitor 反向持有该函数即可，避免 monitor → task 的导入环（task 已 import monitor）。
type SyncImagePoolSnapshot struct {
	WorkerTotal     int           `json:"worker_total"`     // 池大小（固定）
	WorkerBusy      int           `json:"worker_busy"`      // 忙碌 worker 数（瞬时）
	QueueDepth      int           `json:"queue_depth"`      // 当前排队深度
	QueueCap        int           `json:"queue_cap"`        // 队列容量
	Enqueued        int64         `json:"enqueued"`         // 累计入队成功
	Rejected        int64         `json:"rejected"`         // 累计拒绝（队列满 → 429 退款）
	Succeeded       int64         `json:"succeeded"`        // 累计 worker 处理成功
	Failed          int64         `json:"failed"`           // 累计 worker 处理失败
	ChannelInflight map[int64]int `json:"channel_inflight"` // 每渠道在途数（层② per-channel 容量）
	Enabled         bool          `json:"enabled"`          // 池是否已启动（未注册 provider 时 false）
}

// syncImagePoolProvider 由 task 包在启动 worker 池时注册。
var syncImagePoolProvider func() SyncImagePoolSnapshot

// RegisterSyncImagePoolProvider 注册同步图片 worker 池状态取数函数。
func RegisterSyncImagePoolProvider(f func() SyncImagePoolSnapshot) {
	syncImagePoolProvider = f
}

// GetSyncImagePoolSnapshot 返回当前 worker 池状态；未注册（池未启动）时返回 Enabled=false。
func GetSyncImagePoolSnapshot() SyncImagePoolSnapshot {
	if syncImagePoolProvider == nil {
		return SyncImagePoolSnapshot{Enabled: false}
	}
	s := syncImagePoolProvider()
	s.Enabled = true
	return s
}
