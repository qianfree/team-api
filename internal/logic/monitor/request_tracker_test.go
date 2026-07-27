package monitor

import (
	"sync"
	"testing"
	"time"
)

func TestSnapshotBandwidthConcurrent(t *testing.T) {
	tracker := &requestTracker{}
	tracker.bytesIn.Store(1024)
	tracker.bytesOut.Store(2048)

	base := time.Now()
	var workers sync.WaitGroup
	for i := range 100 {
		workers.Add(1)
		go func(offset int) {
			defer workers.Done()
			tracker.snapshotBandwidth(base.Add(time.Duration(offset) * time.Millisecond))
		}(i)
	}
	workers.Wait()

	tracker.bytesIn.Add(100)
	tracker.bytesOut.Add(200)
	snapshot := tracker.snapshotBandwidth(base.Add(time.Second))
	if snapshot.BytesInPerSec < 0 || snapshot.BytesOutPerSec < 0 {
		t.Fatalf("bandwidth rates must not be negative: %+v", snapshot)
	}
}
