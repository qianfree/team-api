package task

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"

	"github.com/qianfree/team-api/internal/logic/billing"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/monitor"
	"github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/relay/channel"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
)

// 同步图片厂商「异步化」worker 池。
//
// 背景：OpenAI/DALL·E 等图片厂商同步阻塞返回（10–60s）。走 /v1/images/generations/async
// 端点时，提交阶段立即建 QUEUED 任务并返回 task_id，真正持有上游长连接的动作交给这里的
// 有界 worker 池（默认 100）在后台执行，客户端轮询 /fetch 取结果。
//
// 三层并发控制：① 全局池（worker 数硬上限）② 单渠道（channelInflight vs MaxConcurrency）
// ③ 渠道间（GetChannelForModel 优先级/权重 + 饱和渠道 exclude 重选）。

const (
	syncImageWorkerCount = 100  // 全局 worker 池大小（层①）
	syncImageQueueSize   = 1000 // 有界队列容量，满则提交侧退款+429
	syncImageMaxAttempts = 16   // 单任务选渠道最大尝试次数（防全饱和时空转）
	syncImageDownloadCap = 20 << 20

	// syncImageShutdownWait 停机时等待在途任务收尾的最长时间。超过则强制停止，
	// 把仍未完成的在途任务标记为超时（TIMEOUT）并退款，不再无限等待上游返回。
	syncImageShutdownWait = 3 * time.Minute
)

var (
	syncImageQueue    chan *SyncImageJob
	syncImageStop     chan struct{} // 关闭 → worker 不再领取新任务，处理完在途任务即退出
	syncImageWg       sync.WaitGroup
	syncImageShutdown atomic.Bool // 停机中：拒绝新任务入队

	// 本实例在途任务登记表（TaskID -> job）。用于停机超时时，只对**本实例**正在执行的
	// 任务做超时+退款，避免误伤多实例部署下其他实例的 IN_PROGRESS 任务。
	syncImageRunningMu sync.Mutex
	syncImageRunning   = make(map[int64]*SyncImageJob)

	channelInflightMu sync.Mutex
	channelInflight   = make(map[int64]int)

	// 池状态计数器（进程内累计，重启归零；供实时监控面板展示）。
	syncImageBusy      atomic.Int64 // 忙碌 worker 数（瞬时）
	syncImageEnqueued  atomic.Int64 // 累计入队成功
	syncImageRejected  atomic.Int64 // 累计拒绝（队列满 → 429 退款）
	syncImageSucceeded atomic.Int64 // 累计 worker 处理成功
	syncImageFailed    atomic.Int64 // 累计 worker 处理失败

	syncImageFileSvc   *lcommon.FileService
	syncImageFileSvcMu sync.Mutex // 保护 syncImageFileSvc 的惰性重建
	syncImageBilling   common.TaskBillingProvider
	syncImageRelayProv common.DataProvider
)

// acquireSyncImageFileSvc 返回共享的对象存储 FileService，惰性重建。
//
// 对象存储配置存于数据库（sys_options），由管理后台**运行时**配置。若进程启动时存储尚未
// 配置，StartSyncImageWorkers 首次构造会失败、syncImageFileSvc 为 nil。此处按需重建：一旦
// 存储在管理后台配置好，下一个 b64_json 图片任务即可成功取到 FileService，**无需重启进程**。
// 这与 admin / data_governance 每次按数据库配置重建 FileService 的行为对齐——只有本 worker
// 池此前把启动时的构造结果缓存成唯一真相源，才导致「配置后仍报未配置」。
//
// 声明为包级变量，便于单测在无数据库环境下覆盖（避免触达 sys_options 查询）。
var acquireSyncImageFileSvc = func(ctx context.Context) (*lcommon.FileService, error) {
	syncImageFileSvcMu.Lock()
	defer syncImageFileSvcMu.Unlock()
	if syncImageFileSvc != nil {
		return syncImageFileSvc, nil
	}
	fs, err := lcommon.NewFileServiceFromConfig(ctx)
	if err != nil {
		return nil, err
	}
	syncImageFileSvc = fs
	return fs, nil
}

// SyncImageJob 同步图片任务的内存载荷（请求体只放内存，不落库，无需崩溃重放）。
type SyncImageJob struct {
	TaskID          int64 // tsk_model_tasks 主键
	PublicTaskID    string
	RequestID       string
	TenantID        int64
	UserID          int64
	ApiKeyID        int64
	ProjectID       int64
	Model           string
	RequestBody     []byte
	PreDeductAmount float64
	Ratios          map[string]float64
	SubmitTime      time.Time
}

// StartSyncImageWorkers 启动 worker 池，与 StartAsyncPolling 同期在 cmd.go 接线。
func StartSyncImageWorkers(ctx context.Context) {
	syncImageQueue = make(chan *SyncImageJob, syncImageQueueSize)
	syncImageStop = make(chan struct{})
	syncImageShutdown.Store(false)
	syncImageRunningMu.Lock()
	syncImageRunning = make(map[int64]*SyncImageJob)
	syncImageRunningMu.Unlock()
	syncImageBilling = billing.NewTaskBillingProvider()
	syncImageRelayProv = relay.NewDataProvider()

	// 对象存储配置存于数据库，可能在进程启动后才由管理后台配置。这里尝试预热一次；
	// 未配置时保持 nil，后续任务运行时由 acquireSyncImageFileSvc 惰性重建（无需重启）。
	if fs, err := lcommon.NewFileServiceFromConfig(ctx); err != nil {
		g.Log().Warningf(ctx, "sync_image: object storage not yet configured (%v); will lazily rebuild when a b64_json image task runs", err)
	} else {
		syncImageFileSvcMu.Lock()
		syncImageFileSvc = fs
		syncImageFileSvcMu.Unlock()
	}

	for i := 0; i < syncImageWorkerCount; i++ {
		syncImageWg.Add(1)
		go syncImageWorkerLoop(i)
	}
	g.Log().Infof(ctx, "sync_image: started %d workers (queue=%d)", syncImageWorkerCount, syncImageQueueSize)

	// 注册状态取数函数，供管理后台实时监控面板展示池状态。
	monitor.RegisterSyncImagePoolProvider(SyncImageWorkerStats)
}

// StopSyncImageWorkers 停机：
//  1. 置停机标志，拒绝新任务入队，通知 worker 不再领取新任务；
//  2. 将队列中未开始的排队任务全部置为 FAILURE 并退款；
//  3. 有界等待在途任务收尾（最长 syncImageShutdownWait）；超过则强制停止，
//     把仍未完成的在途任务标记为超时（TIMEOUT）并退款，不再无限等待上游返回。
func StopSyncImageWorkers() {
	if syncImageQueue == nil {
		return
	}
	ctx := gctx.New()

	// 1. 拒绝新入队，并通知 worker 停止领取新任务。
	syncImageShutdown.Store(true)
	// 幂等：避免重复 close panic。
	select {
	case <-syncImageStop:
	default:
		close(syncImageStop)
	}

	// 2. 将队列中未开始的排队任务全部失败并退款。
	//    此时 worker 收到 stop 后不再领取新任务；即便有个别 worker 在关闭瞬间抢走一条，
	//    channel 接收是原子的，该任务转为在途（由步骤 3 处理），不会与本次排干重复。
	failQueuedOnShutdown(ctx)

	// 3. 有界等待在途任务收尾。
	done := make(chan struct{})
	go func() {
		syncImageWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.Log().Info(ctx, "sync_image: 所有在途任务已完成，worker 池已停止")
	case <-time.After(syncImageShutdownWait):
		// 超过等待上限：强制停止，未完成的在途任务标记为超时并退款。
		// 残留 worker goroutine 仍卡在上游，随进程退出而终止；其后续收尾会因 CAS
		// 谓词失配而 no-op，不会与这里的超时处理重复结算。
		g.Log().Warningf(ctx, "sync_image: 等待在途任务超过 %v，强制停止；未完成任务标记为超时并退款", syncImageShutdownWait)
		timeoutRunningOnShutdown(ctx)
	}
}

// failQueuedOnShutdown 排干队列，把未开始的排队任务全部置为 FAILURE 并退款。
func failQueuedOnShutdown(ctx context.Context) {
	failed := 0
	for {
		select {
		case job := <-syncImageQueue:
			if job == nil {
				continue
			}
			failQueuedSyncImageJob(ctx, job, "server shutting down")
			failed++
		default:
			if failed > 0 {
				g.Log().Infof(ctx, "sync_image: 停机将 %d 个未开始的排队任务置为失败并退款", failed)
			}
			return
		}
	}
}

// timeoutRunningOnShutdown 停机超时：把本实例所有仍未完成的在途任务标记为超时并退款。
func timeoutRunningOnShutdown(ctx context.Context) {
	syncImageRunningMu.Lock()
	jobs := make([]*SyncImageJob, 0, len(syncImageRunning))
	for _, j := range syncImageRunning {
		jobs = append(jobs, j)
	}
	syncImageRunningMu.Unlock()

	timedOut := 0
	for _, job := range jobs {
		if timeoutRunningSyncImageJob(ctx, job) {
			timedOut++
		}
	}
	if timedOut > 0 {
		g.Log().Warningf(ctx, "sync_image: 停机强制将 %d 个在途任务标记为超时并退款", timedOut)
	}
}

// timeoutRunningSyncImageJob 把单个在途任务标记为超时并退款（语义对齐 handleTimedOutTasks）：
// DB status 置 FAILURE(task timed out)、审计 task_status 置 TIMEOUT。CAS 谓词 IN_PROGRESS
// 保护：若残留 worker 已抢先落终态，这里 no-op，避免重复退款/重复减 active 计数。
func timeoutRunningSyncImageJob(ctx context.Context, job *SyncImageJob) bool {
	now := time.Now()
	const reason = "task timed out"

	if err := DefaultAsyncProvider.UpdateTaskCAS(ctx, &common.AsyncTask{
		ID:         job.TaskID,
		Status:     "FAILURE",
		FailReason: reason,
		FinishTime: &now,
	}, "IN_PROGRESS"); err != nil {
		return false // 已被 worker 收尾或其他方抢占
	}

	settled := true
	if job.PreDeductAmount > 0 {
		if err := syncImageBilling.SettleTaskFailed(ctx, job.TenantID, job.RequestID, job.PreDeductAmount); err != nil {
			g.Log().Warningf(ctx, "sync_image: timeout task %s refund failed (unsettled net will retry): %v", job.PublicTaskID, err)
			settled = false
		}
	}
	if settled {
		_ = DefaultAsyncProvider.UpdateTask(ctx, &common.AsyncTask{
			ID:             job.TaskID,
			Status:         "FAILURE",
			FailReason:     reason,
			FinishTime:     &now,
			BillingSettled: true,
		})
	}

	syncImageFailed.Add(1)
	DecrActiveTask()
	monitor.UnregisterRequestByTaskID(job.PublicTaskID)

	usageTask := buildUsageTask(job, nil, "FAILURE", 0, now)
	recordTaskUsage(usageTask, nil, false, reason, nil)
	// 闭环审计：把提交阶段的 SUBMITTED 审计记录更新为 TIMEOUT（与超时兜底网一致）。
	recordTaskCompletionAudit(usageTask, "TIMEOUT", reason, nil)
	return true
}

// EnqueueSyncImageJob 非阻塞入队；队列满返回 false，供提交侧退款+429。
func EnqueueSyncImageJob(job *SyncImageJob) bool {
	if syncImageQueue == nil || syncImageShutdown.Load() {
		syncImageRejected.Add(1)
		return false
	}
	select {
	case syncImageQueue <- job:
		syncImageEnqueued.Add(1)
		return true
	default:
		syncImageRejected.Add(1)
		return false
	}
}

// SyncImageWorkerStats 返回 worker 池当前状态快照，供管理后台实时监控面板展示。
// 在 StartSyncImageWorkers 中注册进 monitor 包（Provider 注入，避免 monitor → task 导入环）。
func SyncImageWorkerStats() monitor.SyncImagePoolSnapshot {
	channelInflightMu.Lock()
	inflight := make(map[int64]int, len(channelInflight))
	for k, v := range channelInflight {
		inflight[k] = v
	}
	channelInflightMu.Unlock()

	qLen, qCap := 0, 0
	if syncImageQueue != nil {
		qLen, qCap = len(syncImageQueue), cap(syncImageQueue)
	}

	return monitor.SyncImagePoolSnapshot{
		WorkerTotal:     syncImageWorkerCount,
		WorkerBusy:      int(syncImageBusy.Load()),
		QueueDepth:      qLen,
		QueueCap:        qCap,
		Enqueued:        syncImageEnqueued.Load(),
		Rejected:        syncImageRejected.Load(),
		Succeeded:       syncImageSucceeded.Load(),
		Failed:          syncImageFailed.Load(),
		ChannelInflight: inflight,
	}
}

func syncImageWorkerLoop(workerID int) {
	defer syncImageWg.Done()
	for {
		// 优先响应停机：一旦收到停机信号，不再领取新的排队任务。已在执行中的在途任务
		// 由 runSyncImageJob 同步跑完（syncImageWg 会等待），未开始的排队任务由
		// StopSyncImageWorkers 在 worker 全部退出后统一置为 FAILURE。
		select {
		case <-syncImageStop:
			return
		default:
		}

		select {
		case <-syncImageStop:
			return
		case job := <-syncImageQueue:
			if job == nil {
				return
			}
			runSyncImageJob(workerID, job)
		}
	}
}

// failQueuedSyncImageJob 停机时把仍处于 QUEUED（未开始）的任务置为 FAILURE 并退款。
// 用 CAS QUEUED->FAILURE 保护：若该任务已被某 worker 抢走（IN_PROGRESS/终态），这里 no-op，
// 由该 worker 负责结算，避免重复退款。
func failQueuedSyncImageJob(ctx context.Context, job *SyncImageJob, reason string) {
	now := time.Now()
	reason = truncateStr(reason, 500)

	if err := DefaultAsyncProvider.UpdateTaskCAS(ctx, &common.AsyncTask{
		ID:         job.TaskID,
		Status:     "FAILURE",
		FailReason: reason,
		FinishTime: &now,
	}, "QUEUED"); err != nil {
		return
	}

	settled := true
	if job.PreDeductAmount > 0 {
		if err := syncImageBilling.SettleTaskFailed(ctx, job.TenantID, job.RequestID, job.PreDeductAmount); err != nil {
			g.Log().Warningf(ctx, "sync_image: shutdown-fail task %s refund failed (unsettled net will retry): %v", job.PublicTaskID, err)
			settled = false
		}
	}
	if settled {
		_ = DefaultAsyncProvider.UpdateTask(ctx, &common.AsyncTask{
			ID:             job.TaskID,
			Status:         "FAILURE",
			FailReason:     reason,
			FinishTime:     &now,
			BillingSettled: true,
		})
	}

	syncImageFailed.Add(1)
	DecrActiveTask()
	monitor.UnregisterRequestByTaskID(job.PublicTaskID)

	usageTask := buildUsageTask(job, nil, "FAILURE", 0, now)
	recordTaskUsage(usageTask, nil, false, reason, nil)
	// 闭环审计：把提交阶段的 SUBMITTED 审计记录更新为 FAILURE。
	recordTaskCompletionAudit(usageTask, "FAILURE", reason, nil)
}

// runSyncImageJob 每任务 recover 兜 panic，防单任务崩溃拖垮 worker。
func runSyncImageJob(workerID int, job *SyncImageJob) {
	syncImageBusy.Add(1)
	// 登记为本实例在途任务，供停机超时时定向处理；defer 保证 panic 也能摘除。
	syncImageRunningMu.Lock()
	syncImageRunning[job.TaskID] = job
	syncImageRunningMu.Unlock()
	defer func() {
		syncImageRunningMu.Lock()
		delete(syncImageRunning, job.TaskID)
		syncImageRunningMu.Unlock()
		syncImageBusy.Add(-1)
		if r := recover(); r != nil {
			g.Log().Errorf(gctx.New(), "sync_image: worker %d panic on task %s: %v", workerID, job.PublicTaskID, r)
		}
	}()
	processSyncImageJob(job)
}

func processSyncImageJob(job *SyncImageJob) {
	ctx := gctx.New()

	// 1. CAS QUEUED -> IN_PROGRESS（与超时网互斥）
	now := time.Now()
	if err := DefaultAsyncProvider.UpdateTaskCAS(ctx, &common.AsyncTask{
		ID:        job.TaskID,
		Status:    "IN_PROGRESS",
		Progress:  "0%",
		StartTime: &now,
	}, "QUEUED"); err != nil {
		g.Log().Warningf(ctx, "sync_image: task %s CAS QUEUED->IN_PROGRESS failed (already taken): %v", job.PublicTaskID, err)
		return
	}

	// 2. 选渠道循环 + 复刻管线（失败换渠道重试）
	var exclude []int64
	lastErr := "no available channel"
	for attempt := 0; attempt < syncImageMaxAttempts; attempt++ {
		sel, err := syncImageRelayProv.GetChannelForModel(ctx, job.TenantID, job.UserID, job.Model, exclude)
		if err != nil {
			lastErr = fmt.Sprintf("select channel: %v", err)
			break // 无更多候选
		}

		// 层②：per-channel 容量。MaxConcurrency<=0 视为不限。
		if !tryOccupyChannel(sel.ChannelID, sel.MaxConcurrency) {
			exclude = append(exclude, sel.ChannelID)
			syncImageJitter()
			continue
		}

		ok, memW, perr := runImagePipelineWithRelease(ctx, job, sel)
		if ok {
			settleSyncImageSuccess(ctx, job, sel, memW)
			return
		}

		// 失败：降健康度 + 排除该渠道 + 抖动退避重选
		syncImageRelayProv.UpdateChannelHealth(ctx, sel.ChannelID, false, 0)
		syncImageRelayProv.IncrementConsecutiveFailure(ctx, sel.ChannelID)
		exclude = append(exclude, sel.ChannelID)
		lastErr = perr
		syncImageJitter()
	}

	// 3. 全饱和/无候选/重试耗尽 → FAILURE + 退款
	failSyncImageJob(ctx, job, nil, lastErr)
}

// runImagePipelineWithRelease 保证无论成功/失败/panic 都释放 per-channel 槽（defer）。
func runImagePipelineWithRelease(ctx context.Context, job *SyncImageJob, sel *common.ChannelSelection) (ok bool, memW *memResponseWriter, failReason string) {
	defer decInflight(sel.ChannelID)
	return runImagePipeline(ctx, job, sel)
}

// runImagePipeline 复刻 RelayHandler 的最小管线，把上游响应捕获到内存 writer。
func runImagePipeline(ctx context.Context, job *SyncImageJob, sel *common.ChannelSelection) (bool, *memResponseWriter, string) {
	info := &common.RelayInfo{
		Context:         ctx,
		TenantID:        job.TenantID,
		UserID:          job.UserID,
		ApiKeyID:        job.ApiKeyID,
		ProjectID:       job.ProjectID,
		RequestID:       job.RequestID,
		RelayMode:       int(constant.RelayModeImagesGenerations),
		IsStream:        false,
		OriginModelName: job.Model,
		BaseModelName:   job.Model,
		RequestURLPath:  "/v1/images/generations",
		RequestHeaders: http.Header{
			"Content-Type": []string{"application/json"},
			"Accept":       []string{"application/json"},
		},
		StartTime:     time.Now(),
		StreamStatus:  common.NewStreamStatus(),
		InboundFormat: constant.RelayFormatOpenAI,
		ClientFormat:  constant.RelayFormatOpenAI,
		ChannelMeta: &common.ChannelMeta{
			ChannelID:         sel.ChannelID,
			ChannelType:       sel.ChannelType,
			ChannelName:       sel.ChannelName,
			BaseURL:           sel.BaseURL,
			ApiKey:            sel.ApiKey,
			UpstreamModelName: sel.UpstreamModelName,
			IsModelMapped:     sel.IsModelMapped,
			Settings:          sel.Settings,
		},
	}

	adaptor := channel.GetAdaptor(sel.ChannelType)
	if adaptor == nil {
		return false, nil, fmt.Sprintf("no adaptor for channelType %d", sel.ChannelType)
	}
	adaptor.Init(info)

	reader, err := adaptor.ConvertRequest(ctx, info, job.RequestBody)
	if err != nil {
		return false, nil, fmt.Sprintf("convert request: %v", err)
	}

	// 上游长连接用后台 context（不随停机被斩断），阻塞 10–60s。
	resp, err := adaptor.DoRequest(context.WithoutCancel(ctx), info, reader)
	if err != nil {
		return false, nil, fmt.Sprintf("do request: %v", err)
	}

	memW := newMemResponseWriter()
	if _, err := adaptor.DoResponse(ctx, resp, info, memW); err != nil {
		return false, memW, fmt.Sprintf("do response: %v", err)
	}
	if memW.StatusCode() != http.StatusOK {
		return false, memW, fmt.Sprintf("upstream status %d: %s", memW.StatusCode(), truncateStr(string(memW.Bytes()), 300))
	}
	return true, memW, ""
}

// settleSyncImageSuccess 成功收尾：先 CAS 赢终态，再结算钱包，最后标记 billing_settled。
// 崩溃窗口（终态已写、settled 未写）由 handleUnsettledTasks 的 sync_image 分支兜底。
func settleSyncImageSuccess(ctx context.Context, job *SyncImageJob, sel *common.ChannelSelection, memW *memResponseWriter) {
	resultURL, normalized, err := buildImageResult(ctx, job, memW.Bytes())
	if err != nil {
		failSyncImageJob(ctx, job, sel, imageFailReason(err))
		return
	}

	// 回填上游返回的 token 用量：gpt-image-1/2 等在响应体 usage 里带 input/output/total_tokens，
	// DALL·E 等则没有。据此重算实际费用（token 计费模型多退少补），并写入计费/用量记录，
	// 避免「同步转异步后 token 恒为 0、费用只等于预扣」的问题。
	promptTokens, completionTokens, totalTokens := extractImageUsage(memW.Bytes())

	now := time.Now()
	// 默认实际费用 == 预扣（按次计费图片，如 DALL·E，无 token 用量）。
	actualCost := job.PreDeductAmount
	// 上游返回了 token 用量 → 按真实 token 重算；仅 token 计费模型返回 >0，按次模型返回 0 保持预扣。
	if totalTokens > 0 {
		if tokenCost, rerr := syncImageBilling.RecalculateByTokens(ctx, job.TenantID, job.Model, totalTokens, job.Ratios); rerr == nil && tokenCost > 0 {
			actualCost = tokenCost
		}
	}

	// 1. 赢得终态（billing_settled=false 先落）
	if err := DefaultAsyncProvider.UpdateTaskCAS(ctx, &common.AsyncTask{
		ID:         job.TaskID,
		Status:     "SUCCESS",
		Progress:   "100%",
		ResultURL:  resultURL,
		Data:       normalized,
		FinishTime: &now,
	}, "IN_PROGRESS"); err != nil {
		// 已被超时网抢占，放弃结算（抢占方负责退款/DecrActiveTask）
		g.Log().Warningf(ctx, "sync_image: task %s terminal CAS failed (preempted): %v", job.PublicTaskID, err)
		return
	}

	// 2. 结算钱包（传真实 token 用量：驱动 bil_records.output_tokens 与计费快照的 token 明细）
	settleResult, serr := syncImageBilling.SettleTaskSuccess(ctx, job.TenantID, job.UserID, job.ApiKeyID, sel.ChannelID,
		job.Model, job.RequestID, actualCost, job.PreDeductAmount, totalTokens, completionTokens, job.Ratios, job.PublicTaskID)
	if serr != nil {
		// 保留 billing_settled=false，由未结算兜底网重放结算
		g.Log().Warningf(ctx, "sync_image: task %s settle success failed (unsettled net will retry): %v", job.PublicTaskID, serr)
	} else {
		// 3. 标记已结算
		_ = DefaultAsyncProvider.UpdateTask(ctx, &common.AsyncTask{
			ID:             job.TaskID,
			Status:         "SUCCESS",
			Progress:       "100%",
			ResultURL:      resultURL,
			Data:           normalized,
			FinishTime:     &now,
			BillingSettled: true,
			ActualCost:     actualCost,
		})
		syncImageBilling.IncrApiKeyQuotaUsed(ctx, job.ApiKeyID, actualCost)
	}

	// 4. 收尾
	syncImageSucceeded.Add(1)
	DecrActiveTask()
	monitor.UnregisterRequestByTaskID(job.PublicTaskID)
	syncImageRelayProv.UpdateChannelHealth(ctx, sel.ChannelID, true, 0)

	chBasic := &common.ChannelBasicInfo{ID: sel.ChannelID, Type: sel.ChannelType, Name: sel.ChannelName}
	usageTask := buildUsageTask(job, sel, "SUCCESS", actualCost, now)
	usageTask.PromptTokens = promptTokens
	usageTask.CompletionTokens = completionTokens
	usageTask.TotalTokens = totalTokens
	recordTaskUsage(usageTask, chBasic, true, "", settleResult)
	// 闭环审计：把提交阶段写入的 SUBMITTED 审计记录更新为终态（与 pollSingleTask 一致），
	// 否则请求审计日志里的任务状态会一直停留在「已提交」。
	recordTaskCompletionAudit(usageTask, "SUCCESS", string(normalized), nil)
}

// failSyncImageJob 失败收尾：CAS 赢终态 → 退款 → 标记已结算 → 用量/计数。
func failSyncImageJob(ctx context.Context, job *SyncImageJob, sel *common.ChannelSelection, reason string) {
	now := time.Now()
	reason = truncateStr(reason, 500)

	if err := DefaultAsyncProvider.UpdateTaskCAS(ctx, &common.AsyncTask{
		ID:         job.TaskID,
		Status:     "FAILURE",
		FailReason: reason,
		FinishTime: &now,
	}, "IN_PROGRESS"); err != nil {
		g.Log().Warningf(ctx, "sync_image: task %s fail CAS failed (already finalized): %v", job.PublicTaskID, err)
		return
	}

	settled := true
	if job.PreDeductAmount > 0 {
		if err := syncImageBilling.SettleTaskFailed(ctx, job.TenantID, job.RequestID, job.PreDeductAmount); err != nil {
			g.Log().Warningf(ctx, "sync_image: task %s refund failed (unsettled net will retry): %v", job.PublicTaskID, err)
			settled = false
		}
	}
	if settled {
		_ = DefaultAsyncProvider.UpdateTask(ctx, &common.AsyncTask{
			ID:             job.TaskID,
			Status:         "FAILURE",
			FailReason:     reason,
			FinishTime:     &now,
			BillingSettled: true,
		})
	}

	syncImageFailed.Add(1)
	DecrActiveTask()
	monitor.UnregisterRequestByTaskID(job.PublicTaskID)

	var chBasic *common.ChannelBasicInfo
	if sel != nil {
		chBasic = &common.ChannelBasicInfo{ID: sel.ChannelID, Type: sel.ChannelType, Name: sel.ChannelName}
	}
	usageTask := buildUsageTask(job, sel, "FAILURE", 0, now)
	recordTaskUsage(usageTask, chBasic, false, reason, nil)
	// 闭环审计：更新提交阶段的 SUBMITTED 审计记录为 FAILURE。
	recordTaskCompletionAudit(usageTask, "FAILURE", reason, nil)
}

// buildImageResult 解析上游图片响应，re-host **全部**图片，返回首图 URL（向后兼容单值
// ResultURL）与归一化后的多图响应体（供落库 Data；fetch 端点据此吐出 data 数组）。
// b64_json 无条件 re-host；url 按配置开关（默认透传，开启则下载 re-host）。
//
// 采用 all-or-nothing：任一张解码 / re-host 失败即整体返回错误，由调用方走 FAILURE + 全额
// 退款（与单图行为一致）。计费按次扁平，多图不额外计费，与同步端点口径保持一致。
func buildImageResult(ctx context.Context, job *SyncImageJob, body []byte) (resultURL string, normalized []byte, err error) {
	var imgResp dto.ImageResponse
	if e := json.Unmarshal(body, &imgResp); e != nil {
		return "", nil, fmt.Errorf("parse image response: %w", e)
	}
	if len(imgResp.Data) == 0 {
		return "", nil, fmt.Errorf("empty image data")
	}

	outData := make([]dto.ImageData, 0, len(imgResp.Data))
	for i, img := range imgResp.Data {
		var url string
		switch {
		case img.B64JSON != "":
			data, e := base64.StdEncoding.DecodeString(img.B64JSON)
			if e != nil {
				return "", nil, fmt.Errorf("decode b64_json[%d]: %w", i, e)
			}
			if url, e = rehostImage(ctx, job, data, "image/png", ".png", i); e != nil {
				return "", nil, fmt.Errorf("rehost b64_json[%d]: %w", i, e)
			}
		case img.URL != "":
			if rehostURLEnabled(ctx) {
				if url, err = rehostFromURL(ctx, job, img.URL, i); err != nil {
					return "", nil, fmt.Errorf("rehost url[%d]: %w", i, err)
				}
			} else {
				url = img.URL
			}
		default:
			return "", nil, fmt.Errorf("no url or b64_json in image response at index %d", i)
		}
		outData = append(outData, dto.ImageData{URL: url, RevisedPrompt: img.RevisedPrompt})
	}

	resultURL = outData[0].URL
	// 归一化落库：存全部结果 URL（不含大体积 b64 原文），供 fetch 吐 data 数组。
	normalized, _ = json.Marshal(dto.ImageResponse{
		Created: imgResp.Created,
		Data:    outData,
	})
	return resultURL, normalized, nil
}

// extractImageUsage 从上游图片响应体解析 token 用量。gpt-image-1/2 在 usage 里返回
// input_tokens / output_tokens / total_tokens；DALL·E 等无 usage 字段，返回全 0。
// total 缺失时用 input+output 兜底。映射：prompt=input、completion=output。
func extractImageUsage(body []byte) (prompt, completion, total int) {
	var resp struct {
		Usage *dto.ImageUsage `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || resp.Usage == nil {
		return 0, 0, 0
	}
	prompt = resp.Usage.InputTokens
	completion = resp.Usage.OutputTokens
	total = resp.Usage.TotalTokens
	if total == 0 {
		total = prompt + completion
	}
	return prompt, completion, total
}

// rehostImage 把单张图片字节上传对象存储并返回下载 URL。index 用于区分同一任务的多张图片，
// 避免多图共用同一对象键相互覆盖。
// imageFailReason 把「结果处理」阶段的内部错误映射为**面向用户**的失败原因（写入 task
// FailReason → fetch 端点的 error 字段 → 在线体验/客户端可见）。
//
// 对象存储未配置是最常见、且用户可操作的错误：图片其实已在上游生成成功，只是网关无处保存
// （b64_json 必须 re-host 到对象存储）。对这种情况给出中文友好提示，引导去系统设置配置
// 对象存储；其余错误（上传/下载失败、上游响应异常等）保留技术细节，便于运维排查。
func imageFailReason(err error) string {
	if errors.Is(err, lcommon.ErrStorageNotConfigured) {
		return "图片已生成，但平台尚未配置对象存储（OSS/S3/COS），无法保存图片。请联系管理员在系统设置中配置对象存储后重试。"
	}
	return "build result: " + err.Error()
}

func rehostImage(ctx context.Context, job *SyncImageJob, data []byte, contentType, ext string, index int) (string, error) {
	fs, err := acquireSyncImageFileSvc(ctx)
	if err != nil {
		return "", fmt.Errorf("object storage not configured, cannot re-host image: %w", err)
	}
	rec, err := fs.Upload(ctx, &lcommon.FileUpload{
		Reader:      bytes.NewReader(data),
		Filename:    fmt.Sprintf("%s_%d%s", job.PublicTaskID, index, ext),
		ContentType: contentType,
		Size:        int64(len(data)),
		TenantID:    job.TenantID,
		UserID:      job.UserID,
	})
	if err != nil {
		return "", err
	}
	return fs.GetDownloadURL(ctx, rec.ID)
}

// syncImageDownloadClient 用于从上游返回的图片 URL 下载**已生成好的**图片做 re-host。
// 注意：这不是图片生成本身——生成走 adaptor 的 nonStreamClient（300s 超时），此处仅下载
// 成品图片文件。显式设置超时，避免 http.DefaultClient 无超时时慢速/挂起的图片服务器长期
// 占用 worker（io.LimitReader 只限体积不限时间）。120s 对 ≤20MB 的成品图很充裕，且远低于
// 生成超时——放宽是为了避免「生成已成功却因下载慢被判失败+退款」的资损。
var syncImageDownloadClient = &http.Client{Timeout: 120 * time.Second}

func rehostFromURL(ctx context.Context, job *SyncImageJob, url string, index int) (string, error) {
	// 先确认存储可用，避免存储未配置时白白下载一遍成品图。
	if _, err := acquireSyncImageFileSvc(ctx); err != nil {
		return "", fmt.Errorf("object storage not configured, cannot re-host image: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := syncImageDownloadClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download image status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, syncImageDownloadCap))
	if err != nil {
		return "", err
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}
	return rehostImage(ctx, job, data, contentType, extFromContentType(contentType), index)
}

// rehostURLEnabled 判断上游返回 url 时是否下载 re-host（默认透传）。
// 声明为包级变量而非普通函数，便于单测覆盖多图 url 透传路径（无需初始化配置中心）。
var rehostURLEnabled = func(ctx context.Context) bool {
	return lcommon.Config().GetBool(ctx, "sync_image_rehost_url")
}

// buildUsageTask 构造供 recordTaskUsage 使用的 AsyncTask（用量日志需要的字段）。
func buildUsageTask(job *SyncImageJob, sel *common.ChannelSelection, status string, actualCost float64, finish time.Time) *common.AsyncTask {
	submit := job.SubmitTime
	t := &common.AsyncTask{
		ID:              job.TaskID,
		PublicTaskID:    job.PublicTaskID,
		RequestID:       job.RequestID,
		Platform:        string(constant.TaskPlatformSyncImage),
		Status:          status,
		TenantID:        job.TenantID,
		UserID:          job.UserID,
		ApiKeyID:        job.ApiKeyID,
		ModelName:       job.Model,
		PreDeductAmount: job.PreDeductAmount,
		ActualCost:      actualCost,
		SubmitTime:      &submit,
		FinishTime:      &finish,
	}
	if sel != nil {
		t.ChannelID = sel.ChannelID
		t.UpstreamModel = sel.UpstreamModelName
	}
	return t
}

// tryOccupyChannel 层②：容量未满则占槽并返回 true。MaxConcurrency<=0 视为不限。
func tryOccupyChannel(channelID int64, maxConcurrency int) bool {
	channelInflightMu.Lock()
	defer channelInflightMu.Unlock()
	if maxConcurrency > 0 && channelInflight[channelID] >= maxConcurrency {
		return false
	}
	channelInflight[channelID]++
	return true
}

func decInflight(channelID int64) {
	channelInflightMu.Lock()
	defer channelInflightMu.Unlock()
	if channelInflight[channelID] > 0 {
		channelInflight[channelID]--
	}
	if channelInflight[channelID] <= 0 {
		delete(channelInflight, channelID)
	}
}

func syncImageJitter() {
	time.Sleep(time.Duration(50+rand.Intn(150)) * time.Millisecond)
}

// truncateStr 将字符串截断到不超过 max 字节，且按 rune 边界截断——避免把多字节 UTF-8
// 字符（如中文）从中间切断，产生非法字节写入 DB 文本列。回退到 max 之内最后一个完整 rune。
func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	truncated := s[:max]
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated
}

func extFromContentType(ct string) string {
	switch {
	case bytes.Contains([]byte(ct), []byte("jpeg")), bytes.Contains([]byte(ct), []byte("jpg")):
		return ".jpg"
	case bytes.Contains([]byte(ct), []byte("webp")):
		return ".webp"
	default:
		return ".png"
	}
}
