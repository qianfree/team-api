package common

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/glog"
)

// EventType defines the type of event.
type EventType string

// Event represents a domain event.
type Event struct {
	Type      EventType       `json:"type"`
	Payload   any             `json:"payload"`
	TenantID  int64           `json:"tenant_id,omitempty"`
	UserID    int64           `json:"user_id,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
	Context   context.Context `json:"-"`
}

// EventHandler is a function that handles an event.
type EventHandler func(ctx context.Context, event *Event) error

var (
	handlers   = make(map[EventType][]EventHandler)
	handlersMu sync.RWMutex
)

// RegisterHandler registers an event handler for a given event type.
// Multiple handlers can be registered for the same event type.
func RegisterHandler(eventType EventType, handler EventHandler) {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	handlers[eventType] = append(handlers[eventType], handler)
}

// Publish publishes an event to all registered handlers.
// Handlers are called synchronously in order. If a handler returns an error,
// it is logged but does not stop other handlers from executing.
func Publish(ctx context.Context, event *Event) {
	if event.Context == nil {
		event.Context = ctx
	}

	handlersMu.RLock()
	handlerList, ok := handlers[event.Type]
	handlersMu.RUnlock()

	if !ok || len(handlerList) == 0 {
		return
	}

	for _, handler := range handlerList {
		if err := handler(event.Context, event); err != nil {
			g.Log().Errorf(event.Context, "event handler error: type=%s, err=%v", event.Type, err)
		}
	}

	// Also log the event for debugging
	payload, _ := json.Marshal(event.Payload)
	g.Log().Debugf(ctx, "event published: type=%s, tenant_id=%d, user_id=%d, payload=%s",
		event.Type, event.TenantID, event.UserID, string(payload))
}

// PublishAsync publishes an event asynchronously in a goroutine.
func PublishAsync(event *Event) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				glog.Errorf(gctx.New(), "panic in async event handler: %v", r)
			}
		}()
		ctx := gctx.New()
		if event.Context != nil {
			ctx = event.Context
		}
		Publish(ctx, event)
	}()
}
