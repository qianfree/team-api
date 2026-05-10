package common

import (
	"context"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

// BatchWriter provides buffered batch writing for high-throughput log tables.
// It accumulates records in memory and flushes them to the database in batches.
//
// Three flush strategies:
//   - Synchronous: Flush immediately (for single writes)
//   - Async batch: Flush when buffer reaches capacity (default 64)
//   - Best-effort: Flush when buffer reaches max capacity (default 256), drop on failure
//
// Usage:
//
//	writer := common.NewBatchWriter("bil_usage_logs", 64, 256)
//	writer.Write(ctx, record)
//	writer.Flush(ctx) // manual flush
//	writer.Close()     // auto flush on shutdown
type BatchWriter struct {
	table      string
	buffer     []any
	bufferMu   sync.Mutex
	flushSize  int
	maxSize    int
	flushTimer *time.Timer
	closed     bool
	closeMu    sync.Mutex
}

// NewBatchWriter creates a new BatchWriter.
//   - table: target database table name
//   - flushSize: flush when buffer reaches this size (0 = no auto-flush)
//   - maxSize: maximum buffer size, drop oldest on overflow
func NewBatchWriter(table string, flushSize, maxSize int) *BatchWriter {
	if maxSize == 0 {
		maxSize = 256
	}
	if flushSize == 0 {
		flushSize = maxSize
	}

	bw := &BatchWriter{
		table:     table,
		buffer:    make([]any, 0, maxSize),
		flushSize: flushSize,
		maxSize:   maxSize,
	}

	// Auto-flush every 5 seconds if there are pending records
	bw.flushTimer = time.AfterFunc(5*time.Second, func() {
		bw.autoFlush()
	})

	return bw
}

// Write adds a record to the buffer.
// If flushSize is reached, triggers an asynchronous flush.
func (bw *BatchWriter) Write(ctx context.Context, record any) {
	bw.bufferMu.Lock()
	defer bw.bufferMu.Unlock()

	if bw.closed {
		return
	}

	if len(bw.buffer) >= bw.maxSize {
		// Drop oldest record (best-effort)
		bw.buffer = bw.buffer[1:]
	}

	bw.buffer = append(bw.buffer, record)

	// Trigger async flush if threshold reached
	if len(bw.buffer) >= bw.flushSize {
		go bw.flush(gctx.New())
	}
}

// Flush synchronously flushes all buffered records to the database.
func (bw *BatchWriter) Flush(ctx context.Context) error {
	bw.bufferMu.Lock()
	if len(bw.buffer) == 0 {
		bw.bufferMu.Unlock()
		return nil
	}
	records := make([]any, len(bw.buffer))
	copy(records, bw.buffer)
	bw.buffer = bw.buffer[:0]
	bw.bufferMu.Unlock()

	if len(records) == 0 {
		return nil
	}

	_, err := g.DB().Model(bw.table).Ctx(ctx).Data(records).Batch(len(records)).Insert()
	if err != nil {
		g.Log().Errorf(ctx, "batch write to %s failed (%d records): %v", bw.table, len(records), err)
		return err
	}
	return nil
}

// flush is the internal flush without locking the buffer.
func (bw *BatchWriter) flush(ctx context.Context) {
	_ = bw.Flush(ctx)
}

// autoFlush is called periodically by the timer.
func (bw *BatchWriter) autoFlush() {
	bw.closeMu.Lock()
	defer bw.closeMu.Unlock()

	if bw.closed {
		return
	}

	_ = bw.Flush(gctx.New())

	// Reset timer
	bw.flushTimer.Reset(5 * time.Second)
}

// Len returns the current buffer length.
func (bw *BatchWriter) Len() int {
	bw.bufferMu.Lock()
	defer bw.bufferMu.Unlock()
	return len(bw.buffer)
}

// Close flushes all remaining records and stops the auto-flush timer.
func (bw *BatchWriter) Close() {
	bw.closeMu.Lock()
	defer bw.closeMu.Unlock()

	if bw.closed {
		return
	}
	bw.closed = true

	if bw.flushTimer != nil {
		bw.flushTimer.Stop()
	}

	// Final flush
	_ = bw.Flush(gctx.New())
}

// SyncWrite writes a single record synchronously (bypasses buffer).
// Use this for critical records that must be persisted immediately.
func SyncWrite(ctx context.Context, table string, record any) error {
	_, err := g.DB().Model(table).Ctx(ctx).Data(record).Insert()
	return err
}
