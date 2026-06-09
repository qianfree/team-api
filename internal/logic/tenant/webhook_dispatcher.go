package tenant

import (
	"context"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/model/entity"
)

const (
	dispatcherChannelSize   = 1000
	dispatcherWorkerCount   = 5
	dispatcherSweepInterval = 2 * time.Minute

	// 清理策略
	cleanupInterval       = 6 * time.Hour       // 每 6 小时执行一次清理
	cleanupDeliveredAge   = 7 * 24 * time.Hour  // 已投递事件保留 7 天
	cleanupFailedAge      = 30 * 24 * time.Hour // 已放弃事件保留 30 天
	cleanupDeliveryLogAge = 30 * 24 * time.Hour // 投递日志保留 30 天
	cleanupBatchSize      = 500                 // 每批删除条数
)

type WebhookDispatcher struct {
	eventCh chan int64
	ctx     context.Context
	wg      sync.WaitGroup
}

var defaultDispatcher *WebhookDispatcher

// InitWebhookDispatcher initializes the event-driven webhook dispatcher.
func InitWebhookDispatcher(ctx context.Context) {
	d := &WebhookDispatcher{
		eventCh: make(chan int64, dispatcherChannelSize),
		ctx:     ctx,
	}
	defaultDispatcher = d

	for i := 0; i < dispatcherWorkerCount; i++ {
		d.wg.Add(1)
		go d.worker()
	}
	go d.sweepLoop()
	go d.cleanupLoop()

	// Recover any pending events left from a previous crash
	go d.recoverPending()

	g.Log().Infof(ctx, "webhook dispatcher started: %d workers, sweep every %v, cleanup every %v", dispatcherWorkerCount, dispatcherSweepInterval, cleanupInterval)
}

// NotifyNewEvent pushes an event ID to the dispatcher channel (non-blocking).
func NotifyNewEvent(eventID int64) {
	if defaultDispatcher == nil {
		return
	}
	select {
	case defaultDispatcher.eventCh <- eventID:
	default:
		g.Log().Warningf(defaultDispatcher.ctx, "webhook dispatcher channel full, event %d will be picked up by sweep", eventID)
	}
}

// worker consumes event IDs from the channel and delivers them.
func (d *WebhookDispatcher) worker() {
	defer d.wg.Done()
	for {
		select {
		case <-d.ctx.Done():
			return
		case eventID := <-d.eventCh:
			deliverEventByID(d.ctx, eventID)
		}
	}
}

// sweepLoop periodically scans for missed events (crash recovery / channel overflow).
func (d *WebhookDispatcher) sweepLoop() {
	ticker := time.NewTicker(dispatcherSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.sweep()
		}
	}
}

// sweep finds pending/failed events due for delivery and pushes them into the channel.
func (d *WebhookDispatcher) sweep() {
	var events []entity.OpnWebhookEvents
	err := dao.OpnWebhookEvents.Ctx(d.ctx).
		Where("status IN (?)", g.Slice{"pending", "failed"}).
		Where("next_retry_at <= ?", gtime.Now()).
		Where("attempts < ?", webhookMaxRetries).
		OrderAsc("next_retry_at").
		Limit(50).
		Scan(&events)
	if err != nil {
		g.Log().Error(d.ctx, "webhook sweep: scan failed:", err)
		return
	}

	for _, evt := range events {
		NotifyNewEvent(evt.Id)
	}
}

// recoverPending runs once at startup to pick up events left from a previous process.
func (d *WebhookDispatcher) recoverPending() {
	d.sweep()
}

// scheduleRetry schedules a delayed re-delivery for a failed event.
// The retry is skipped if the dispatcher context has been cancelled (e.g. during shutdown).
func scheduleRetry(eventID int64, delay time.Duration) {
	time.AfterFunc(delay, func() {
		if defaultDispatcher == nil {
			return
		}
		select {
		case <-defaultDispatcher.ctx.Done():
			return
		default:
		}
		NotifyNewEvent(eventID)
	})
}

// cleanupLoop periodically removes old delivered events, exhausted failed events, and delivery logs.
func (d *WebhookDispatcher) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.cleanup()
		}
	}
}

// cleanup removes old webhook data in batches to prevent table bloat.
// Order: delivery_logs → events (delivery_logs reference events via event_id).
func (d *WebhookDispatcher) cleanup() {
	ctx := d.ctx
	now := gtime.Now()

	// 1. 清理投递日志（保留 30 天）
	logCutoff := now.Add(-cleanupDeliveryLogAge)
	for {
		result, err := dao.OpnWebhookDeliveryLogs.Ctx(ctx).
			Where("created_at < ?", logCutoff).
			Limit(cleanupBatchSize).
			Delete()
		if err != nil {
			g.Log().Errorf(ctx, "webhook cleanup: delete delivery logs failed: %v", err)
			break
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			break
		}
		g.Log().Infof(ctx, "webhook cleanup: deleted %d delivery logs older than %v", rows, logCutoff.Format("Y-m-d"))
	}

	// 2. 清理已投递事件（保留 7 天）
	deliveredCutoff := now.Add(-cleanupDeliveredAge)
	for {
		result, err := dao.OpnWebhookEvents.Ctx(ctx).
			Where("status", "delivered").
			Where("updated_at < ?", deliveredCutoff).
			Limit(cleanupBatchSize).
			Delete()
		if err != nil {
			g.Log().Errorf(ctx, "webhook cleanup: delete delivered events failed: %v", err)
			break
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			break
		}
		g.Log().Infof(ctx, "webhook cleanup: deleted %d delivered events older than %v", rows, deliveredCutoff.Format("Y-m-d"))
	}

	// 3. 清理已放弃的重试事件（attempts >= max，保留 30 天）
	failedCutoff := now.Add(-cleanupFailedAge)
	for {
		result, err := dao.OpnWebhookEvents.Ctx(ctx).
			Where("id IN (?)", dao.OpnWebhookEvents.Ctx(ctx).
				Where("status", "failed").
				Where("attempts >= ?", webhookMaxRetries).
				Where("updated_at < ?", failedCutoff).
				Limit(cleanupBatchSize).
				Fields("id"),
			).
			Delete()
		if err != nil {
			g.Log().Errorf(ctx, "webhook cleanup: delete exhausted events failed: %v", err)
			break
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			break
		}
		g.Log().Infof(ctx, "webhook cleanup: deleted %d exhausted events older than %v", rows, failedCutoff.Format("Y-m-d"))
	}
}

// Shutdown waits for all in-flight deliveries to complete.
func (d *WebhookDispatcher) Shutdown() {
	d.wg.Wait()
}

// ShutdownWebhookDispatcher waits for all in-flight webhook deliveries to complete.
// Call this via defer after s.Run() returns.
func ShutdownWebhookDispatcher() {
	if defaultDispatcher != nil {
		defaultDispatcher.Shutdown()
	}
}
