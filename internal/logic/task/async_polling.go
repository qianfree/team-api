package task

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/taskchannel"
)

const (
	pollInterval     = 15 * time.Second
	taskTimeout      = 30 * time.Minute
	pollBatchSize    = 500
	timeoutBatchSize = 100
)

// StartAsyncPolling 启动异步任务轮询 goroutine
func StartAsyncPolling(ctx context.Context) {
	g.Log().Info(ctx, "Starting async task polling...")
	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				g.Log().Info(ctx, "Async task polling stopped")
				return
			case <-ticker.C:
				pollOnce(ctx)
			}
		}
	}()
}

// pollOnce 执行一次轮询
func pollOnce(ctx context.Context) {
	if !HasActiveTasks() {
		return
	}

	// 1. 处理超时任务
	handleTimedOutTasks(ctx)

	// 2. 轮询非终态任务
	tasks, err := DefaultAsyncProvider.GetNonTerminalTasks(ctx, pollBatchSize)
	if err != nil {
		g.Log().Warningf(ctx, "poll: get non-terminal tasks: %v", err)
		return
	}

	if len(tasks) == 0 {
		return
	}

	g.Log().Debugf(ctx, "poll: processing %d active tasks", len(tasks))

	// 3. 按 platform 分组处理
	platformGroups := make(map[string][]*common.AsyncTask)
	for _, t := range tasks {
		platformGroups[t.Platform] = append(platformGroups[t.Platform], t)
	}

	for platform, platformTasks := range platformGroups {
		processPlatformTasks(ctx, platform, platformTasks)
	}
}

// handleTimedOutTasks 处理超时任务
func handleTimedOutTasks(ctx context.Context) {
	cutoff := time.Now().Add(-taskTimeout).Unix()
	tasks, err := DefaultAsyncProvider.GetTimedOutTasks(ctx, cutoff, timeoutBatchSize)
	if err != nil {
		g.Log().Warningf(ctx, "poll: get timed-out tasks: %v", err)
		return
	}

	for _, t := range tasks {
		t.Status = "FAILURE"
		t.FailReason = "task timed out"
		now := time.Now()
		t.FinishTime = &now

		if err := DefaultAsyncProvider.UpdateTaskCAS(ctx, t, t.Status); err != nil {
			g.Log().Warningf(ctx, "poll: mark timeout task %s: %v", t.PublicTaskID, err)
			continue
		}
		DecrActiveTask()

		// 退还预扣费用
		if t.PreDeductAmount > 0 {
			taskBilling := billing.NewTaskBillingProvider()
			if err := taskBilling.SettleTaskFailed(ctx, t.TenantID, t.PublicTaskID, t.PreDeductAmount); err != nil {
				g.Log().Warningf(ctx, "poll: refund timed-out task %s: %v", t.PublicTaskID, err)
			}
		}
		g.Log().Infof(ctx, "poll: task %s timed out", t.PublicTaskID)

		// 更新审计记录
		recordTaskCompletionAudit(t, "TIMEOUT", "", nil)
	}
}

// processPlatformTasks 处理同一平台的任务
func processPlatformTasks(ctx context.Context, platform string, tasks []*common.AsyncTask) {
	// 按 channel_id 分组
	channelGroups := make(map[int64][]*common.AsyncTask)
	for _, t := range tasks {
		channelGroups[t.ChannelID] = append(channelGroups[t.ChannelID], t)
	}

	for channelID, channelTasks := range channelGroups {
		processChannelTasks(ctx, platform, channelID, channelTasks)
	}
}

// processChannelTasks 处理同一渠道的任务
func processChannelTasks(ctx context.Context, platform string, channelID int64, tasks []*common.AsyncTask) {
	// 获取渠道信息
	channel, err := DefaultAsyncProvider.GetChannelByID(ctx, channelID)
	if err != nil || channel == nil {
		g.Log().Warningf(ctx, "poll: get channel %d: %v", channelID, err)
		return
	}

	providerType := constant.ProviderType(channel.Type)
	adaptor, err := taskchannel.GetAdaptor(providerType)
	if err != nil {
		g.Log().Warningf(ctx, "poll: get adaptor for channel %d: %v", channelID, err)
		return
	}

	for _, t := range tasks {
		pollSingleTask(ctx, adaptor, channel, t)
	}
}

// pollSingleTask 轮询单个任务
func pollSingleTask(ctx context.Context, adaptor common.TaskAdaptor, channel *common.ChannelBasicInfo, task *common.AsyncTask) {
	// 从 PrivateData 提取上游任务 ID
	var private struct {
		UpstreamTaskID string `json:"upstream_task_id"`
	}
	if err := json.Unmarshal(task.PrivateData, &private); err != nil || private.UpstreamTaskID == "" {
		g.Log().Warningf(ctx, "poll: invalid private data for task %s", task.PublicTaskID)
		return
	}

	// 查询上游状态
	taskData, _ := json.Marshal(map[string]any{
		"task_id": private.UpstreamTaskID,
	})

	resp, err := adaptor.FetchTask(channel.BaseURL, channel.ApiKey, taskData)
	if err != nil {
		g.Log().Warningf(ctx, "poll: fetch task %s: %v", task.PublicTaskID, err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		g.Log().Warningf(ctx, "poll: read response for task %s: %v", task.PublicTaskID, err)
		return
	}

	// 解析结果
	taskInfo, err := adaptor.ParseTaskResult(body)
	if err != nil {
		g.Log().Warningf(ctx, "poll: parse result for task %s: %v", task.PublicTaskID, err)
		return
	}

	// CAS 更新状态
	oldStatus := task.Status
	task.Status = string(taskInfo.Status)
	task.Progress = taskInfo.Progress
	task.Data = body

	if taskInfo.FailReason != "" {
		task.FailReason = taskInfo.FailReason
	}

	now := time.Now()
	switch {
	case taskInfo.Status == common.TaskStatusInProgress:
		task.StartTime = &now
	case taskInfo.Status.IsTerminal():
		task.FinishTime = &now
		if task.StartTime == nil {
			task.StartTime = &now
		}
		// 保留上游返回的 token 用量
		if taskInfo.TotalTokens > 0 {
			task.CompletionTokens = taskInfo.CompletionTokens
			task.PromptTokens = taskInfo.PromptTokens
			task.TotalTokens = taskInfo.TotalTokens
		}
	}

	if err := DefaultAsyncProvider.UpdateTaskCAS(ctx, task, oldStatus); err != nil {
		g.Log().Debugf(ctx, "poll: CAS conflict for task %s (status changed by another process): %v", task.PublicTaskID, err)
		return
	}

	if taskInfo.Status.IsTerminal() {
		DecrActiveTask()
	}

	// 处理终态
	if taskInfo.Status == common.TaskStatusSuccess {
		task.ResultURL = taskInfo.ResultURL
		DefaultAsyncProvider.UpdateTask(ctx, task) // 更新 result_url

		// 结算计费
		if task.PreDeductAmount > 0 && !task.BillingSettled {
			taskBilling := billing.NewTaskBillingProvider()
			actualCost := task.PreDeductAmount
			if taskInfo.ActualCost > 0 {
				actualCost = taskInfo.ActualCost
			}
			task.ActualCost = actualCost
			if err := taskBilling.SettleTaskSuccess(ctx, task.TenantID, task.UserID, task.ApiKeyID, task.ChannelID, task.ModelName, task.PublicTaskID, actualCost, task.PreDeductAmount); err != nil {
				g.Log().Warningf(ctx, "poll: settle task %s: %v", task.PublicTaskID, err)
			} else {
				task.BillingSettled = true
				DefaultAsyncProvider.UpdateTask(ctx, task)
			}
		}
		g.Log().Infof(ctx, "poll: task %s completed", task.PublicTaskID)

		// 记录用量日志
		recordTaskUsage(task, true, "")

		// 更新审计记录
		recordTaskCompletionAudit(task, "SUCCESS", string(body), upstreamRespHeaders(resp))

	} else if taskInfo.Status == common.TaskStatusFailure {
		// 退还预扣费用
		if task.PreDeductAmount > 0 && !task.BillingSettled {
			taskBilling := billing.NewTaskBillingProvider()
			if err := taskBilling.SettleTaskFailed(ctx, task.TenantID, task.PublicTaskID, task.PreDeductAmount); err != nil {
				g.Log().Warningf(ctx, "poll: refund failed task %s: %v", task.PublicTaskID, err)
			} else {
				task.BillingSettled = true
				DefaultAsyncProvider.UpdateTask(ctx, task)
			}
		}
		g.Log().Infof(ctx, "poll: task %s failed: %s", task.PublicTaskID, task.FailReason)

		// 记录用量日志
		recordTaskUsage(task, false, task.FailReason)

		// 更新审计记录
		recordTaskCompletionAudit(task, "FAILURE", string(body), upstreamRespHeaders(resp))
	}
}

// recordTaskUsage 异步记录视频/音乐等异步任务的用量日志
func recordTaskUsage(task *common.AsyncTask, success bool, errMsg string) {
	latencyMs := 0
	if task.SubmitTime != nil && task.FinishTime != nil {
		latencyMs = int(task.FinishTime.Sub(*task.SubmitTime).Milliseconds())
	}

	status := "success"
	if !success {
		status = "error"
	}

	relay.NewDataProvider().RecordUsage(context.Background(), &common.UsageRecord{
		TenantID:         task.TenantID,
		UserID:           task.UserID,
		ApiKeyID:         task.ApiKeyID,
		ChannelID:        task.ChannelID,
		ModelName:        task.ModelName,
		RelayMode:        int(constant.RelayModeVideoGenerations),
		RequestType:      3, // async
		LatencyMs:        float64(latencyMs),
		IsStream:         false,
		Success:          success,
		RequestID:        task.RequestID,
		Status:           status,
		ErrorMessage:     errMsg,
		PromptTokens:     task.PromptTokens,
		CompletionTokens: task.CompletionTokens,
		TotalTokens:      task.TotalTokens,
		TotalCost:        task.ActualCost,
		ActualCost:       task.ActualCost,
		PreDeductAmount:  task.PreDeductAmount,
		BillingMode:      "per_request",
		BillingSource:    "task",
		TaskID:           task.PublicTaskID,
	})
}

// recordTaskCompletionAudit 更新提交阶段写入的审计记录，补充异步任务最终结果
func recordTaskCompletionAudit(task *common.AsyncTask, status string, resultBody string, upstreamHeaders map[string]string) {
	now := time.Now()
	relay.NewDataProvider().UpdateTaskAudit(context.Background(), &common.AuditRecord{
		TenantID:            task.TenantID,
		TaskID:              task.PublicTaskID,
		TaskStatus:          status,
		TaskResult:          resultBody,
		TaskUpstreamHeaders: upstreamHeaders,
		TaskCompletedAt:     &now,
	})
}

// upstreamRespHeaders 从上游 HTTP 响应中提取响应头
func upstreamRespHeaders(resp *http.Response) map[string]string {
	if resp == nil || resp.Header == nil {
		return nil
	}
	headers := make(map[string]string)
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}
	return headers
}
