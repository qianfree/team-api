package task

import (
	"context"
	"sync/atomic"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
)

var activeTaskCount atomic.Int64

// IncrActiveTask increments the active async task counter.
func IncrActiveTask() {
	activeTaskCount.Add(1)
}

// DecrActiveTask decrements the active async task counter.
func DecrActiveTask() {
	if activeTaskCount.Add(-1) < 0 {
		activeTaskCount.Store(0)
	}
}

// HasActiveTasks returns true if there are active async tasks.
func HasActiveTasks() bool {
	return activeTaskCount.Load() > 0
}

// InitActiveCount initializes the counter from the database at startup.
func InitActiveCount(ctx context.Context) {
	count, err := dao.TskModelTasks.Ctx(ctx).
		Where("status NOT IN (?, ?)", "SUCCESS", "FAILURE").
		Count()
	if err != nil {
		g.Log().Warningf(ctx, "init active task count failed: %v", err)
		return
	}
	activeTaskCount.Store(int64(count))
	g.Log().Infof(ctx, "active async task count initialized: %d", count)
}
