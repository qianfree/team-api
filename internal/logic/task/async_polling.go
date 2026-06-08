package task

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/monitor"
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

var pollingWg sync.WaitGroup

// StartAsyncPolling 启动异步任务轮询 goroutine
func StartAsyncPolling(ctx context.Context) {
	g.Log().Info(ctx, "Starting async task polling...")
	pollingWg.Add(1)
	go func() {
		defer pollingWg.Done()
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

// StopAsyncPolling 等待轮询 goroutine 完全退出，在服务停机时调用。
func StopAsyncPolling() {
	pollingWg.Wait()
}

// pollOnce 执行一次轮询
func pollOnce(ctx context.Context) {
	if !HasActiveTasks() {
		return
	}

	// 1. 处理超时任务
	handleTimedOutTasks(ctx)

	// 2. 重试未结算的终态任务
	handleUnsettledTasks(ctx)

	// 3. 轮询非终态任务
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
		monitor.UnregisterRequestByTaskID(t.PublicTaskID)

		// 退还预扣费用
		if t.PreDeductAmount > 0 {
			taskBilling := billing.NewTaskBillingProvider()
			if err := taskBilling.SettleTaskFailed(ctx, t.TenantID, t.RequestID, t.PreDeductAmount); err != nil {
				g.Log().Warningf(ctx, "poll: refund timed-out task %s: %v", t.PublicTaskID, err)
			}
			billing.CleanupPreDeduct(ctx, t.TenantID, t.RequestID+"_adjust")
		}
		g.Log().Infof(ctx, "poll: task %s timed out", t.PublicTaskID)

		// 查询渠道信息并记录用量日志
		var ch *common.ChannelBasicInfo
		if t.ChannelID > 0 {
			ch, _ = DefaultAsyncProvider.GetChannelByID(ctx, t.ChannelID)
		}
		recordTaskUsage(t, ch, false, "task timed out", nil)

		// 更新审计记录
		recordTaskCompletionAudit(t, "TIMEOUT", "", nil)
	}
}

// handleUnsettledTasks 重试终态但未结算的任务
func handleUnsettledTasks(ctx context.Context) {
	tasks, err := DefaultAsyncProvider.GetUnsettledTasks(ctx, timeoutBatchSize)
	if err != nil {
		g.Log().Warningf(ctx, "poll: get unsettled tasks: %v", err)
		return
	}

	for _, t := range tasks {
		var pd privateData
		if err := json.Unmarshal(t.PrivateData, &pd); err != nil || pd.UpstreamTaskID == "" {
			// 无法恢复，直接退还预扣
			g.Log().Warningf(ctx, "poll: unsettled task %s has invalid private_data, refunding", t.PublicTaskID)
			taskBilling := billing.NewTaskBillingProvider()
			if err := taskBilling.SettleTaskFailed(ctx, t.TenantID, t.RequestID, t.PreDeductAmount); err != nil {
				g.Log().Errorf(ctx, "poll: refund unsettled task %s: %v", t.PublicTaskID, err)
			} else {
				t.BillingSettled = true
				DefaultAsyncProvider.UpdateTask(ctx, t)
			}
			billing.CleanupPreDeduct(ctx, t.TenantID, t.RequestID+"_adjust")
			continue
		}

		if t.Status == "SUCCESS" {
			// 成功任务：用 ActualCost（已在上次轮询中计算）结算
			actualCost := t.ActualCost
			if actualCost <= 0 {
				actualCost = t.PreDeductAmount
			}
			taskBilling := billing.NewTaskBillingProvider()
			_, err := taskBilling.SettleTaskSuccess(ctx, t.TenantID, t.UserID, t.ApiKeyID, t.ChannelID,
				t.ModelName, t.RequestID, actualCost, t.PreDeductAmount,
				0, 0, pd.BillingContext.Ratios, t.PublicTaskID)
			if err != nil {
				g.Log().Warningf(ctx, "poll: retry settle task %s: %v", t.PublicTaskID, err)
			} else {
				t.BillingSettled = true
				t.ActualCost = actualCost
				DefaultAsyncProvider.UpdateTask(ctx, t)
				g.Log().Infof(ctx, "poll: retried settlement for task %s", t.PublicTaskID)
			}
		} else {
			// 失败任务：退还预扣
			taskBilling := billing.NewTaskBillingProvider()
			if err := taskBilling.SettleTaskFailed(ctx, t.TenantID, t.RequestID, t.PreDeductAmount); err != nil {
				g.Log().Warningf(ctx, "poll: retry refund task %s: %v", t.PublicTaskID, err)
			} else {
				t.BillingSettled = true
				DefaultAsyncProvider.UpdateTask(ctx, t)
				g.Log().Infof(ctx, "poll: retried refund for task %s", t.PublicTaskID)
			}
			billing.CleanupPreDeduct(ctx, t.TenantID, t.RequestID+"_adjust")
		}
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
		processChannelTasks(ctx, channelID, channelTasks)
	}
}

// processChannelTasks 处理同一渠道的任务
func processChannelTasks(ctx context.Context, channelID int64, tasks []*common.AsyncTask) {
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

// privateData PrivateData 反序列化结构
type privateData struct {
	UpstreamTaskID string `json:"upstream_task_id"`
	TaskType       string `json:"task_type"`
	BillingContext struct {
		Ratios    map[string]float64 `json:"ratios"`
		ModelName string             `json:"model_name"`
		PreDeduct float64            `json:"pre_deduct"`
	} `json:"billing_context"`
}

// pollSingleTask 轮询单个任务
func pollSingleTask(ctx context.Context, adaptor common.TaskAdaptor, channel *common.ChannelBasicInfo, task *common.AsyncTask) {
	// 从 PrivateData 提取上游任务 ID 和计费上下文
	var pd privateData
	if err := json.Unmarshal(task.PrivateData, &pd); err != nil || pd.UpstreamTaskID == "" {
		g.Log().Warningf(ctx, "poll: invalid private data for task %s", task.PublicTaskID)
		return
	}

	// 查询上游状态
	taskData, _ := json.Marshal(map[string]any{
		"task_id": pd.UpstreamTaskID,
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
		monitor.UnregisterRequestByTaskID(task.PublicTaskID)
	}

	// 处理终态
	if taskInfo.Status == common.TaskStatusSuccess {
		task.ResultURL = taskInfo.ResultURL
		DefaultAsyncProvider.UpdateTask(ctx, task) // 更新 result_url

		// 结算计费
		var settleResult *common.SettlementResult
		if task.PreDeductAmount > 0 && !task.BillingSettled {
			taskBilling := billing.NewTaskBillingProvider()
			actualCost := task.PreDeductAmount

			// 优先用上游返回的 ActualCost
			if taskInfo.ActualCost > 0 {
				actualCost = taskInfo.ActualCost
			} else if taskInfo.TotalTokens > 0 && pd.BillingContext.Ratios != nil {
				// 用上游 total_tokens + 保存的 ratios 重算
				if tokenCost, err := taskBilling.RecalculateByTokens(ctx, task.TenantID, task.ModelName, taskInfo.TotalTokens, pd.BillingContext.Ratios); err == nil && tokenCost > 0 {
					actualCost = tokenCost
				}
			}

			task.ActualCost = actualCost
			settleResult, err = taskBilling.SettleTaskSuccess(ctx, task.TenantID, task.UserID, task.ApiKeyID, task.ChannelID, task.ModelName, task.RequestID, actualCost, task.PreDeductAmount, taskInfo.TotalTokens, taskInfo.CompletionTokens, pd.BillingContext.Ratios, task.PublicTaskID)
			if err != nil {
				g.Log().Warningf(ctx, "poll: settle task %s: %v", task.PublicTaskID, err)
			} else {
				task.BillingSettled = true
				DefaultAsyncProvider.UpdateTask(ctx, task)
			}
			billing.CleanupPreDeduct(ctx, task.TenantID, task.RequestID+"_adjust")
		}
		g.Log().Infof(ctx, "poll: task %s completed", task.PublicTaskID)

		// 记录用量日志
		recordTaskUsage(task, channel, true, "", settleResult)

		// 更新审计记录
		recordTaskCompletionAudit(task, "SUCCESS", string(body), upstreamRespHeaders(resp))

	} else if taskInfo.Status == common.TaskStatusFailure {
		// 退还预扣费用
		if task.PreDeductAmount > 0 && !task.BillingSettled {
			taskBilling := billing.NewTaskBillingProvider()
			if err := taskBilling.SettleTaskFailed(ctx, task.TenantID, task.RequestID, task.PreDeductAmount); err != nil {
				g.Log().Warningf(ctx, "poll: refund failed task %s: %v", task.PublicTaskID, err)
			} else {
				task.BillingSettled = true
				DefaultAsyncProvider.UpdateTask(ctx, task)
			}
			billing.CleanupPreDeduct(ctx, task.TenantID, task.RequestID+"_adjust")
		}
		g.Log().Infof(ctx, "poll: task %s failed: %s", task.PublicTaskID, task.FailReason)

		// 记录用量日志
		recordTaskUsage(task, channel, false, task.FailReason, nil)

		// 更新审计记录
		recordTaskCompletionAudit(task, "FAILURE", string(body), upstreamRespHeaders(resp))
	}
}

// recordTaskUsage 异步记录视频/音乐等异步任务的用量日志
func recordTaskUsage(task *common.AsyncTask, channel *common.ChannelBasicInfo, success bool, errMsg string, settleResult *common.SettlementResult) {
	latencyMs := 0
	if task.SubmitTime != nil && task.FinishTime != nil {
		latencyMs = int(task.FinishTime.Sub(*task.SubmitTime).Milliseconds())
	}

	// 提取渠道名称和类型
	var channelName string
	var channelType int
	if channel != nil {
		channelName = channel.Name
		channelType = channel.Type
	}

	status := "success"
	if !success {
		status = "error"
	}

	record := &common.UsageRecord{
		TenantID:         task.TenantID,
		UserID:           task.UserID,
		ApiKeyID:         task.ApiKeyID,
		ChannelID:        task.ChannelID,
		ChannelName:      channelName,
		ChannelType:      channelType,
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
		BillingSource:    "task",
		TaskID:           task.PublicTaskID,
	}

	// 从结算结果填充计费快照
	if settleResult != nil {
		record.BillingSnapshot = settleResult.BillingSnapshot
		record.BillingSummary = settleResult.BillingSummary
		record.BillingMode = settleResult.BillingMode
		record.BillingSource = settleResult.BillingSource
		record.RateMultiplier = settleResult.RateMultiplier
		record.InputCost = settleResult.InputCost
		record.OutputCost = settleResult.OutputCost
		record.RefundAmount = settleResult.RefundAmount
		record.SupplementAmount = settleResult.SupplementAmount
		record.Currency = "USD"
	} else {
		// 失败/超时任务：从定价中获取计费模式
		billingMode := "per_request"
		if pricing, err := billing.GetModelPrice(context.Background(), task.TenantID, task.ModelName); err == nil {
			if pricing.BillingMode != "" {
				billingMode = pricing.BillingMode
			}
		}
		record.BillingMode = billingMode
	}

	relay.NewDataProvider().RecordUsage(context.Background(), record)
}

// recordTaskCompletionAudit 更新提交阶段写入的审计记录，补充异步任务最终结果
func recordTaskCompletionAudit(task *common.AsyncTask, status string, resultBody string, upstreamHeaders map[string]string) {
	now := time.Now()

	// 计算从任务提交到完成的端到端延迟
	latencyMs := 0
	if task.SubmitTime != nil {
		latencyMs = int(now.Sub(*task.SubmitTime).Milliseconds())
	}

	relay.NewDataProvider().UpdateTaskAudit(context.Background(), &common.AuditRecord{
		TenantID:            task.TenantID,
		TaskID:              task.PublicTaskID,
		TaskStatus:          status,
		TaskResult:          resultBody,
		TaskUpstreamHeaders: upstreamHeaders,
		TaskCompletedAt:     &now,
		LatencyMs:           latencyMs,
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
