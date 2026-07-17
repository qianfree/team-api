package task

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

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
)

var (
	syncImageQueue chan *SyncImageJob
	syncImageStop  chan struct{}
	syncImageWg    sync.WaitGroup

	channelInflightMu sync.Mutex
	channelInflight   = make(map[int64]int)

	// 池状态计数器（进程内累计，重启归零；供实时监控面板展示）。
	syncImageBusy      atomic.Int64 // 忙碌 worker 数（瞬时）
	syncImageEnqueued  atomic.Int64 // 累计入队成功
	syncImageRejected  atomic.Int64 // 累计拒绝（队列满 → 429 退款）
	syncImageSucceeded atomic.Int64 // 累计 worker 处理成功
	syncImageFailed    atomic.Int64 // 累计 worker 处理失败

	syncImageFileSvc   *lcommon.FileService
	syncImageBilling   common.TaskBillingProvider
	syncImageRelayProv common.DataProvider
)

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
	syncImageBilling = billing.NewTaskBillingProvider()
	syncImageRelayProv = relay.NewDataProvider()

	// 对象存储在启动时构造一次共享（无状态，可安全共享）。未配置时置 nil，
	// b64_json 任务在无存储时会走 FAILURE+退款（见 buildImageResult）。
	if fs, err := lcommon.NewFileServiceFromConfig(ctx); err != nil {
		g.Log().Warningf(ctx, "sync_image: object storage not configured (%v); b64_json image tasks will fail until storage is set", err)
	} else {
		syncImageFileSvc = fs
	}

	for i := 0; i < syncImageWorkerCount; i++ {
		syncImageWg.Add(1)
		go syncImageWorkerLoop(i)
	}
	g.Log().Infof(ctx, "sync_image: started %d workers (queue=%d)", syncImageWorkerCount, syncImageQueueSize)

	// 注册状态取数函数，供管理后台实时监控面板展示池状态。
	monitor.RegisterSyncImagePoolProvider(SyncImageWorkerStats)
}

// StopSyncImageWorkers 停机时优雅关闭；在途任务完成后退出，队列内未开始的任务留 QUEUED，
// 由超时兜底网最终 FAILURE+退款。
func StopSyncImageWorkers() {
	if syncImageStop != nil {
		close(syncImageStop)
	}
	syncImageWg.Wait()
}

// EnqueueSyncImageJob 非阻塞入队；队列满返回 false，供提交侧退款+429。
func EnqueueSyncImageJob(job *SyncImageJob) bool {
	if syncImageQueue == nil {
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

// runSyncImageJob 每任务 recover 兜 panic，防单任务崩溃拖垮 worker。
func runSyncImageJob(workerID int, job *SyncImageJob) {
	syncImageBusy.Add(1)
	defer func() {
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
		failSyncImageJob(ctx, job, sel, fmt.Sprintf("build result: %v", err))
		return
	}

	now := time.Now()
	// per_request 图片：实际费用 == 预扣（RecalculateByTokens 对 per_request 返回 0，无 token 重算）。
	actualCost := job.PreDeductAmount

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

	// 2. 结算钱包
	settleResult, serr := syncImageBilling.SettleTaskSuccess(ctx, job.TenantID, job.UserID, job.ApiKeyID, sel.ChannelID,
		job.Model, job.RequestID, actualCost, job.PreDeductAmount, 0, 0, job.Ratios, job.PublicTaskID)
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
	recordTaskUsage(buildUsageTask(job, sel, "SUCCESS", actualCost, now), chBasic, true, "", settleResult)
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
	recordTaskUsage(buildUsageTask(job, sel, "FAILURE", 0, now), chBasic, false, reason, nil)
}

// buildImageResult 解析上游图片响应，返回结果 URL 与归一化后的响应体（供落库 Data）。
// b64_json 无条件 re-host；url 按配置开关（默认透传，开启则下载 re-host）。
func buildImageResult(ctx context.Context, job *SyncImageJob, body []byte) (resultURL string, normalized []byte, err error) {
	var imgResp dto.ImageResponse
	if e := json.Unmarshal(body, &imgResp); e != nil {
		return "", nil, fmt.Errorf("parse image response: %w", e)
	}
	if len(imgResp.Data) == 0 {
		return "", nil, fmt.Errorf("empty image data")
	}
	if len(imgResp.Data) > 1 {
		g.Log().Warningf(ctx, "sync_image: task %s returned %d images; only the first is surfaced via ResultURL", job.PublicTaskID, len(imgResp.Data))
	}

	first := imgResp.Data[0]
	switch {
	case first.B64JSON != "":
		data, e := base64.StdEncoding.DecodeString(first.B64JSON)
		if e != nil {
			return "", nil, fmt.Errorf("decode b64_json: %w", e)
		}
		resultURL, err = rehostImage(ctx, job, data, "image/png", ".png")
	case first.URL != "":
		if shouldRehostURL(ctx) {
			resultURL, err = rehostFromURL(ctx, job, first.URL)
		} else {
			resultURL = first.URL
		}
	default:
		return "", nil, fmt.Errorf("no url or b64_json in image response")
	}
	if err != nil {
		return "", nil, err
	}

	// 归一化落库：只存单张结果 URL，避免把大体积 b64 原文写进 data 列。
	normalized, _ = json.Marshal(dto.ImageResponse{
		Created: imgResp.Created,
		Data:    []dto.ImageData{{URL: resultURL, RevisedPrompt: first.RevisedPrompt}},
	})
	return resultURL, normalized, nil
}

func rehostImage(ctx context.Context, job *SyncImageJob, data []byte, contentType, ext string) (string, error) {
	if syncImageFileSvc == nil {
		return "", fmt.Errorf("object storage not configured, cannot re-host image")
	}
	rec, err := syncImageFileSvc.Upload(ctx, &lcommon.FileUpload{
		Reader:      bytes.NewReader(data),
		Filename:    job.PublicTaskID + ext,
		ContentType: contentType,
		Size:        int64(len(data)),
		TenantID:    job.TenantID,
		UserID:      job.UserID,
	})
	if err != nil {
		return "", err
	}
	return syncImageFileSvc.GetDownloadURL(ctx, rec.ID)
}

func rehostFromURL(ctx context.Context, job *SyncImageJob, url string) (string, error) {
	if syncImageFileSvc == nil {
		return "", fmt.Errorf("object storage not configured, cannot re-host image")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
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
	return rehostImage(ctx, job, data, contentType, extFromContentType(contentType))
}

func shouldRehostURL(ctx context.Context) bool {
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

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
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
