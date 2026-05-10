package tenant

import (
	"context"
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
)

type WebhookDispatcher struct {
	eventCh chan int64
	ctx     context.Context
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
		go d.worker()
	}
	go d.sweepLoop()

	// Recover any pending events left from a previous crash
	go d.recoverPending()

	g.Log().Infof(ctx, "webhook dispatcher started: %d workers, sweep every %v", dispatcherWorkerCount, dispatcherSweepInterval)
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
func scheduleRetry(eventID int64, delay time.Duration) {
	time.AfterFunc(delay, func() {
		NotifyNewEvent(eventID)
	})
}
