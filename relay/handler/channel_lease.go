package handler

import (
	"context"
	"sync"
	"time"

	"github.com/qianfree/team-api/relay/common"
)

type channelLease struct {
	provider  common.DataProvider
	channelID int64
	requestID string
	stop      chan struct{}
	once      sync.Once
}

func acquireChannelLease(ctx context.Context, provider common.DataProvider, selection *common.ChannelSelection, requestID string) (*channelLease, bool) {
	if selection.MaxConcurrency <= 0 {
		return nil, true
	}
	if !provider.AcquireChannelSlot(ctx, selection.ChannelID, selection.MaxConcurrency, requestID) {
		return nil, false
	}
	lease := &channelLease{
		provider:  provider,
		channelID: selection.ChannelID,
		requestID: requestID,
		stop:      make(chan struct{}),
	}
	go lease.refreshLoop()
	return lease, true
}

func (l *channelLease) refreshLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			l.provider.RefreshChannelSlot(ctx, l.channelID, l.requestID)
			cancel()
		case <-l.stop:
			return
		}
	}
}

func (l *channelLease) Release() {
	if l == nil {
		return
	}
	l.once.Do(func() {
		close(l.stop)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		l.provider.ReleaseChannelSlot(ctx, l.channelID, l.requestID)
	})
}
