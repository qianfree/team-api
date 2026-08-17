package common

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

// OverflowPolicy 队列满时的处理策略
type OverflowPolicy int

const (
	OverflowDrop         OverflowPolicy = iota // 丢弃新记录
	OverflowBlock                              // 阻塞等待
	OverflowSyncFallback                       // 降级为同步写入
)

// UsageLogWriterConfig 写入器配置
type UsageLogWriterConfig struct {
	Table         string
	QueueSize     int
	BatchSize     int
	FlushInterval time.Duration
	Workers       int
	Overflow      OverflowPolicy
	// DB 可选：返回本次写入使用的数据库实例。为 nil 时使用默认库 g.DB()。
	// 审计日志等需落到独立审计库（database.audit 分组）时传入 GetAuditDB。
	DB func() gdb.DB
}

// WriterStats 写入器统计
type WriterStats struct {
	Submitted int64 `json:"submitted"`
	Completed int64 `json:"completed"`
	Dropped   int64 `json:"dropped"`
	Failed    int64 `json:"failed"`
	QueueLen  int   `json:"queue_len"`
}

// UsageLogWriter channel + 固定 worker 的异步批量写入器
type UsageLogWriter struct {
	table         string
	queue         chan any
	batchSize     int
	flushInterval time.Duration
	workers       int
	overflow      OverflowPolicy
	db            func() gdb.DB

	submitted atomic.Int64
	completed atomic.Int64
	dropped   atomic.Int64
	failed    atomic.Int64

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// DefaultUsageLogWriter 全局实例
var DefaultUsageLogWriter *UsageLogWriter

// InitUsageLogWriter 初始化全局写入器
func InitUsageLogWriter() {
	DefaultUsageLogWriter = NewUsageLogWriter(UsageLogWriterConfig{
		Table:         "bil_usage_logs",
		QueueSize:     8192,
		BatchSize:     64,
		FlushInterval: 3 * time.Second,
		Workers:       4,
		Overflow:      OverflowDrop,
	})
	DefaultUsageLogWriter.Start()
}

// CloseUsageLogWriter 关闭全局写入器
func CloseUsageLogWriter() {
	if DefaultUsageLogWriter != nil {
		DefaultUsageLogWriter.Close()
	}
}

// NewUsageLogWriter 创建写入器实例
func NewUsageLogWriter(cfg UsageLogWriterConfig) *UsageLogWriter {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 8192
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 64
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 3 * time.Second
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	return &UsageLogWriter{
		table:         cfg.Table,
		queue:         make(chan any, cfg.QueueSize),
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
		workers:       cfg.Workers,
		overflow:      cfg.Overflow,
		db:            cfg.DB,
		stopCh:        make(chan struct{}),
	}
}

// Start 启动所有 worker
func (w *UsageLogWriter) Start() {
	for i := range w.workers {
		w.wg.Add(1)
		go w.runWorker(i)
	}
}

// Submit 提交一条记录到队列
func (w *UsageLogWriter) Submit(record any) {
	w.submitted.Add(1)
	switch w.overflow {
	case OverflowBlock:
		w.queue <- record
	case OverflowSyncFallback:
		select {
		case w.queue <- record:
		default:
			ctx := gctx.New()
			if _, err := w.dbInstance().Model(w.table).Ctx(ctx).Data(record).Insert(); err != nil {
				w.failed.Add(1)
				g.Log().Errorf(ctx, "usage_log_writer: sync fallback insert failed: %v", err)
			} else {
				w.completed.Add(1)
			}
		}
	default: // OverflowDrop
		select {
		case w.queue <- record:
		default:
			w.dropped.Add(1)
		}
	}
}

// Stats 返回当前统计
func (w *UsageLogWriter) Stats() WriterStats {
	return WriterStats{
		Submitted: w.submitted.Load(),
		Completed: w.completed.Load(),
		Dropped:   w.dropped.Load(),
		Failed:    w.failed.Load(),
		QueueLen:  len(w.queue),
	}
}

// Close 优雅关闭：通知 worker 停止，排空队列
func (w *UsageLogWriter) Close() {
	close(w.stopCh)
	w.wg.Wait()
}

// runWorker 单个 worker 循环
func (w *UsageLogWriter) runWorker(id int) {
	defer w.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			g.Log().Errorf(gctx.New(), "usage_log_writer: worker %d panic: %v, restarting", id, r)
			w.wg.Add(1)
			go w.runWorker(id)
		}
	}()

	buf := make([]any, 0, w.batchSize)
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(buf) == 0 {
			return
		}
		w.flushBatch(buf)
		buf = buf[:0]
	}

	for {
		select {
		case record, ok := <-w.queue:
			if !ok {
				flush()
				return
			}
			buf = append(buf, record)
			if len(buf) >= w.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-w.stopCh:
			// 排空 channel 中剩余记录
			for {
				select {
				case record, ok := <-w.queue:
					if !ok {
						flush()
						return
					}
					buf = append(buf, record)
					if len(buf) >= w.batchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

// dbInstance 返回本次写入使用的数据库实例：配置了自定义 DB 函数（如独立审计库）时优先使用，
// 否则回退到默认库。
func (w *UsageLogWriter) dbInstance() gdb.DB {
	if w.db != nil {
		return w.db()
	}
	return g.DB()
}

// flushBatch 批量写入数据库
func (w *UsageLogWriter) flushBatch(records []any) {
	if len(records) == 0 {
		return
	}
	batch := make([]any, len(records))
	copy(batch, records)

	ctx := gctx.New()
	_, err := w.dbInstance().Model(w.table).Ctx(ctx).Data(batch).Batch(len(batch)).Insert()
	if err != nil {
		// 批量写入失败（如某条记录含非法 UTF-8 字节被 PG 整体拒绝）时降级为逐条写入，
		// 避免一条脏数据拖垮整批记录（usage_logs 为计费数据，audit_logs 不允许丢失）；
		// 单条仍失败才丢弃该条，并汇总打日志。
		g.Log().Errorf(ctx, "usage_log_writer: batch insert %d records failed: %v, fallback to per-record insert", len(batch), err)
		saved := 0
		var firstErr error
		for _, rec := range batch {
			if _, e := w.dbInstance().Model(w.table).Ctx(ctx).Data(rec).Insert(); e != nil {
				if firstErr == nil {
					firstErr = e
				}
				continue
			}
			saved++
		}
		w.completed.Add(int64(saved))
		dropped := len(batch) - saved
		w.failed.Add(int64(dropped))
		if firstErr != nil {
			g.Log().Errorf(ctx, "usage_log_writer: %d/%d records dropped in per-record fallback, first error: %v", dropped, len(batch), firstErr)
		}
		return
	}
	w.completed.Add(int64(len(batch)))
}
