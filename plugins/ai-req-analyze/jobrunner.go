package aireqanalyze

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
)

// 异步任务执行器：进程内单 flight 队列，同时最多 jobWorkerCount 个任务在跑。
// 任务状态机 pending → running → completed/failed 全程落库；
// 服务重启后残留的 pending/running 由插件 Init 标记 failed（队列不跨重启）。
//
// 竞态防护：worker 认领任务用「status=pending 条件更新」，affected=0 视为
// 已被其他实例/重复入队认领，直接跳过——多副本部署下天然安全。

const (
	jobWorkerCount = 2
	jobQueueSize   = 100
)

var (
	runnerMu   sync.Mutex
	jobQueue   chan int64
	runnerStop chan struct{}
	runnerWG   sync.WaitGroup
	runnerOn   bool
)

// ensureJobRunner 惰性启动 worker 池（首个任务提交时）。
// 禁用插件后再次启用可重新启动（runnerMu + runnerOn 支持启停循环）。
func ensureJobRunner() {
	runnerMu.Lock()
	defer runnerMu.Unlock()
	if runnerOn {
		return
	}
	jobQueue = make(chan int64, jobQueueSize)
	runnerStop = make(chan struct{})
	runnerOn = true
	for i := 0; i < jobWorkerCount; i++ {
		runnerWG.Add(1)
		go jobWorker(jobQueue, runnerStop)
	}
	glog.Info(context.Background(), "ai-req-analyze: job runner started")
}

// stopJobRunner 停止 worker 池并等待在跑任务结束（插件禁用/销毁时调用）。
// 队列中未开始的任务保持 pending 状态落库，由下次 Init 的残留清理标记 failed。
func stopJobRunner() {
	runnerMu.Lock()
	defer runnerMu.Unlock()
	if !runnerOn {
		return
	}
	runnerOn = false
	close(runnerStop)
	runnerWG.Wait()
	glog.Info(context.Background(), "ai-req-analyze: job runner stopped")
}

func jobWorker(queue <-chan int64, stop <-chan struct{}) {
	defer runnerWG.Done()
	for {
		select {
		case <-stop:
			return
		case jobID := <-queue:
			safeExecuteJob(jobID)
		}
	}
}

// enqueueJob 投递任务；队列满时返回错误（由 API 层转成友好提示）。
func enqueueJob(jobID int64) error {
	ensureJobRunner()
	select {
	case jobQueue <- jobID:
		return nil
	default:
		return errQueueFull
	}
}

var errQueueFull = &queueFullError{}

type queueFullError struct{}

func (e *queueFullError) Error() string {
	return "分析任务队列已满，请等待现有任务完成后再发起"
}

// safeExecuteJob 带 panic 防护执行任务（goroutine 内 panic 会静默杀掉 worker）。
func safeExecuteJob(jobID int64) {
	defer func() {
		if r := recover(); r != nil {
			glog.Errorf(context.Background(), "ai-req-analyze: 任务执行 panic job=%d: %v", jobID, r)
			failJob(context.Background(), jobID, "任务执行异常中断")
		}
	}()
	executeJob(context.Background(), jobID)
}

// executeJob 认领并执行单个任务。
func executeJob(ctx context.Context, jobID int64) {
	var job JobEntity
	err := g.DB().Model(tableJobs).Ctx(ctx).Where("id", jobID).Scan(&job)
	if err != nil || job.Id == 0 {
		glog.Warningf(ctx, "ai-req-analyze: 任务不存在或加载失败 job=%d err=%v", jobID, err)
		return
	}
	if job.Status != JobStatusPending {
		return // 已完成/失败/被其他 worker 认领
	}

	// 条件更新认领：仅 pending → running 成功者执行
	result, err := g.DB().Model(tableJobs).Ctx(ctx).
		Where("id", jobID).Where("status", JobStatusPending).
		Data(JobDO{Status: JobStatusRunning, ErrorMsg: ""}).
		Update()
	if err != nil {
		glog.Warningf(ctx, "ai-req-analyze: 认领任务失败 job=%d: %v", jobID, err)
		return
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		failJob(ctx, jobID, "加载插件配置失败: "+err.Error())
		return
	}
	if err := cfg.Validate(); err != nil {
		failJob(ctx, jobID, err.Error())
		return
	}

	reportID, err := runAnalysis(ctx, &job, cfg, NewAIClient(cfg))
	if err != nil {
		failJob(ctx, jobID, err.Error())
		return
	}

	if _, err := g.DB().Model(tableJobs).Ctx(ctx).
		Where("id", jobID).
		Data(JobDO{Status: JobStatusCompleted, ReportId: reportID, Progress: 100, ErrorMsg: ""}).
		Update(); err != nil {
		glog.Errorf(ctx, "ai-req-analyze: 更新任务完成状态失败 job=%d: %v", jobID, err)
	}
	glog.Infof(ctx, "ai-req-analyze: 任务完成 job=%d report=%d", jobID, reportID)
}

// failJob 标记任务失败。
func failJob(ctx context.Context, jobID int64, msg string) {
	if _, err := g.DB().Model(tableJobs).Ctx(ctx).
		Where("id", jobID).
		Data(JobDO{Status: JobStatusFailed, ErrorMsg: msg}).
		Update(); err != nil {
		glog.Errorf(ctx, "ai-req-analyze: 更新任务失败状态失败 job=%d: %v", jobID, err)
	}
}

// markStaleJobsFailed 清理服务重启残留的任务（插件 Init 时调用）。
func markStaleJobsFailed(ctx context.Context) {
	result, err := g.DB().Model(tableJobs).Ctx(ctx).
		Where("status IN (?)", []string{JobStatusPending, JobStatusRunning}).
		Data(JobDO{Status: JobStatusFailed, ErrorMsg: "服务重启导致任务中断，请重新发起"}).
		Update()
	if err != nil {
		glog.Warningf(ctx, "ai-req-analyze: 清理残留任务失败: %v", err)
		return
	}
	if rows, _ := result.RowsAffected(); rows > 0 {
		glog.Infof(ctx, "ai-req-analyze: 已将 %d 个重启残留任务标记为失败", rows)
	}
}
