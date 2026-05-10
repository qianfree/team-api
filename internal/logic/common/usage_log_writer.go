package common

import (
	"sync"
	"sync/atomic"
	"time"

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
			if _, err := g.DB().Model(w.table).Ctx(ctx).Data(record).Insert(); err != nil {
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

// flushBatch 批量写入数据库
func (w *UsageLogWriter) flushBatch(records []any) {
	if len(records) == 0 {
		return
	}
	batch := make([]any, len(records))
	copy(batch, records)

	ctx := gctx.New()
	_, err := g.DB().Model(w.table).Ctx(ctx).Data(batch).Batch(len(batch)).Insert()
	if err != nil {
		w.failed.Add(int64(len(batch)))
		g.Log().Errorf(ctx, "usage_log_writer: batch insert %d records failed: %v", len(batch), err)
		return
	}
	w.completed.Add(int64(len(batch)))
}
