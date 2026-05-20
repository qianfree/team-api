package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/taskchannel"
	_ "github.com/qianfree/team-api/relay/taskchannel/ali"
	_ "github.com/qianfree/team-api/relay/taskchannel/kling"
	"github.com/qianfree/team-api/relay/taskchannel/midjourney"
	_ "github.com/qianfree/team-api/relay/taskchannel/sora"
	_ "github.com/qianfree/team-api/relay/taskchannel/suno"
	_ "github.com/qianfree/team-api/relay/taskchannel/volcengine"
)

// TaskRelayContext 异步任务 relay 上下文
type TaskRelayContext struct {
	TenantID        int64
	UserID          int64
	ApiKeyID        int64
	ProjectID       int64
	TaskID          string // 由 HandleTaskSubmit 在创建任务后设置
	RequestID       string
	Writer          http.ResponseWriter
	Scope           string
	ClientIP        string
	ForwardingTrace *common.ForwardingTrace
}

// HandleTaskSubmit 异步任务提交管线
func HandleTaskSubmit(
	ctx context.Context,
	body []byte,
	path string,
	headers http.Header,
	rc *TaskRelayContext,
	dataProvider common.TaskDataProvider,
	billingProvider common.TaskBillingProvider,
	channelMeta *common.ChannelMeta,
) {
	// 1. 解析模型名
	var req map[string]json.RawMessage
	if err := json.Unmarshal(body, &req); err != nil {
		writeTaskError(rc.Writer, http.StatusBadRequest, "invalid request body", "")
		return
	}

	modelName := ""
	if v, ok := req["model"]; ok {
		if err := json.Unmarshal(v, &modelName); err != nil {
			writeTaskError(rc.Writer, http.StatusBadRequest, "invalid model field", "")
			return
		}
	}
	if modelName == "" {
		writeTaskError(rc.Writer, http.StatusBadRequest, "model is required", "")
		return
	}

	// 2. 确定任务平台
	providerType := constant.ProviderType(channelMeta.ChannelType)
	platform, ok := constant.ProviderTypeToTaskPlatform(providerType)
	if !ok {
		g.Log().Warningf(ctx, "HandleTaskSubmit: unsupported task platform, channelType=%d, modelName=%s", channelMeta.ChannelType, modelName)
		writeTaskError(rc.Writer, http.StatusBadRequest, "unsupported task platform", "")
		return
	}

	// 3. 获取 TaskAdaptor
	adaptor, err := taskchannel.GetAdaptor(providerType)
	if err != nil {
		writeTaskError(rc.Writer, http.StatusInternalServerError, err.Error(), "")
		return
	}

	g.Log().Debugf(ctx, "HandleTaskSubmit: modelName=%s, platform=%s, channelID=%d, channelType=%d, baseURL=%s, upstreamModel=%s", modelName, platform, channelMeta.ChannelID, channelMeta.ChannelType, channelMeta.BaseURL, channelMeta.UpstreamModelName)

	// 4. 构建 RelayInfo
	info := &common.RelayInfo{
		Context:         ctx,
		TenantID:        rc.TenantID,
		UserID:          rc.UserID,
		ApiKeyID:        rc.ApiKeyID,
		RequestID:       rc.RequestID,
		RelayMode:       int(constant.RelayModeVideoGenerations),
		OriginModelName: modelName,
		RequestURLPath:  path,
		RequestHeaders:  headers,
		StartTime:       time.Now(),
		ChannelMeta:     channelMeta,
	}
	adaptor.Init(info)

	// 5. 校验请求
	if taskErr := adaptor.ValidateRequest(ctx, info, body); taskErr != nil {
		g.Log().Warningf(ctx, "HandleTaskSubmit: validate failed, model=%s, err=%s", modelName, taskErr.Message)
		writeTaskError(rc.Writer, taskErr.StatusCode, taskErr.Message, taskErr.ErrCode)
		return
	}

	// 6. 估算计费 + 预扣
	ratios := adaptor.EstimateBilling(ctx, info, body)
	estimatedCost, err := billingProvider.EstimateTaskCost(ctx, rc.TenantID, modelName, ratios)
	if err != nil {
		g.Log().Errorf(ctx, "HandleTaskSubmit: estimate cost failed, model=%s, err=%v", modelName, err)
		writeTaskError(rc.Writer, http.StatusInternalServerError, "estimate cost failed: "+err.Error(), "")
		return
	}
	g.Log().Debugf(ctx, "HandleTaskSubmit: estimatedCost=%.4f", estimatedCost)

	preDeductAmount, err := billingProvider.PreDeductTask(ctx, rc.TenantID, rc.RequestID, estimatedCost, modelName)
	if err != nil {
		g.Log().Warningf(ctx, "HandleTaskSubmit: pre-deduct failed, model=%s, err=%v", modelName, err)
		writeTaskError(rc.Writer, http.StatusPaymentRequired, "insufficient balance", "")
		return
	}

	// 7. 构建并发送请求
	requestBody, err := adaptor.BuildRequestBody(ctx, info, body)
	if err != nil {
		billingProvider.SettleTaskFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
		g.Log().Errorf(ctx, "HandleTaskSubmit: build request failed, model=%s, err=%v", modelName, err)
		writeTaskError(rc.Writer, http.StatusInternalServerError, "build request failed: "+err.Error(), "")
		return
	}

	resp, err := adaptor.DoRequest(ctx, info, requestBody)
	if err != nil {
		billingProvider.SettleTaskFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
		g.Log().Errorf(ctx, "HandleTaskSubmit: upstream request failed, model=%s, err=%v", modelName, err)
		writeTaskError(rc.Writer, http.StatusBadGateway, "upstream request failed: "+err.Error(), "")
		return
	}
	defer resp.Body.Close()

	g.Log().Debugf(ctx, "HandleTaskSubmit: upstream responded status=%d", resp.StatusCode)

	// 8. 解析响应
	upstreamTaskID, taskData, taskErr := adaptor.DoResponse(ctx, resp, info)
	if taskErr != nil {
		billingProvider.SettleTaskFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
		g.Log().Warningf(ctx, "HandleTaskSubmit: upstream response error, model=%s, status=%d, message=%q, body=%s", modelName, taskErr.StatusCode, taskErr.Message, string(taskData))
		writeTaskError(rc.Writer, taskErr.StatusCode, taskErr.Message, taskErr.ErrCode)
		return
	}

	// 9. 调整计费
	adjustedRatios := adaptor.AdjustBillingOnSubmit(info, taskData)
	if adjustedRatios != nil {
		newCost, _ := billingProvider.EstimateTaskCost(ctx, rc.TenantID, modelName, adjustedRatios)
		preDeductAmount, _ = billingProvider.AdjustTaskBilling(ctx, rc.TenantID, rc.RequestID, preDeductAmount, newCost)
	}

	// 9.5 合并最终 ratios 用于结算时还原
	finalRatios := ratios
	if adjustedRatios != nil {
		if finalRatios == nil {
			finalRatios = adjustedRatios
		} else {
			merged := make(map[string]float64, len(finalRatios)+len(adjustedRatios))
			for k, v := range finalRatios {
				merged[k] = v
			}
			for k, v := range adjustedRatios {
				merged[k] = v
			}
			finalRatios = merged
		}
	}

	// 10. 生成公开任务 ID 并创建记录
	publicTaskID := generatePublicTaskID()
	now := time.Now()

	privateData, _ := json.Marshal(map[string]any{
		"upstream_task_id": upstreamTaskID,
		"task_type":        platform,
		"billing_context": map[string]any{
			"ratios":     finalRatios,
			"model_name": modelName,
			"pre_deduct": preDeductAmount,
		},
	})

	task := &common.AsyncTask{
		PublicTaskID:    publicTaskID,
		RequestID:       rc.RequestID,
		Platform:        string(platform),
		Action:          "generate",
		Status:          "SUBMITTED",
		Progress:        "0%",
		TenantID:        rc.TenantID,
		UserID:          rc.UserID,
		ApiKeyID:        rc.ApiKeyID,
		ChannelID:       channelMeta.ChannelID,
		ModelName:       modelName,
		UpstreamModel:   channelMeta.UpstreamModelName,
		PreDeductAmount: preDeductAmount,
		Data:            taskData,
		PrivateData:     privateData,
		SubmitTime:      &now,
	}

	if err := dataProvider.CreateTask(ctx, task); err != nil {
		billingProvider.SettleTaskFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
		writeTaskError(rc.Writer, http.StatusInternalServerError, "create task record failed: "+err.Error(), "")
		return
	}

	// 设置 TaskID 供外层审计使用
	rc.TaskID = publicTaskID

	// 11. 返回响应
	respBody := map[string]any{
		"id":         publicTaskID,
		"status":     "SUBMITTED",
		"model":      modelName,
		"created_at": now.Unix(),
	}
	writeJSON(rc.Writer, http.StatusOK, respBody)
}

// HandleTaskFetch 异步任务查询管线
func HandleTaskFetch(
	ctx context.Context,
	publicTaskID string,
	rc *TaskRelayContext,
	dataProvider common.TaskDataProvider,
) {
	task, err := dataProvider.GetTaskByPublicIDAndUser(ctx, publicTaskID, rc.UserID)
	if err != nil {
		writeTaskError(rc.Writer, http.StatusInternalServerError, "query task failed", "")
		return
	}
	if task == nil {
		writeTaskError(rc.Writer, http.StatusNotFound, "task not found", "")
		return
	}

	respBody := map[string]any{
		"id":         task.PublicTaskID,
		"status":     task.Status,
		"model":      task.ModelName,
		"created_at": task.CreatedAt.Unix(),
	}
	if task.Progress != "" {
		respBody["progress"] = task.Progress
	}
	if task.ResultURL != "" {
		respBody["url"] = task.ResultURL
	}
	if task.FailReason != "" {
		respBody["error"] = task.FailReason
	}
	if task.FinishTime != nil {
		respBody["completed_at"] = task.FinishTime.Unix()
	}

	writeJSON(rc.Writer, http.StatusOK, respBody)
}

// writeTaskError 写入错误响应
func writeTaskError(w http.ResponseWriter, statusCode int, message, code string) {
	resp := map[string]any{
		"error": map[string]any{
			"type":    "invalid_request_error",
			"message": message,
		},
	}
	if code != "" {
		resp["error"].(map[string]any)["code"] = code
	}
	writeJSON(w, statusCode, resp)
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// generatePublicTaskID 生成公开任务 ID
func generatePublicTaskID() string {
	return fmt.Sprintf("task_%s", randomHex(12))
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

// HandleMjImageProxy Midjourney 图片代理
func HandleMjImageProxy(
	ctx context.Context,
	publicTaskID string,
	rc *TaskRelayContext,
	dataProvider common.TaskDataProvider,
	writer http.ResponseWriter,
) {
	task, err := dataProvider.GetTaskByPublicIDAndUser(ctx, publicTaskID, rc.UserID)
	if err != nil {
		writeTaskError(writer, http.StatusInternalServerError, "query task failed", "")
		return
	}
	if task == nil {
		writeTaskError(writer, http.StatusNotFound, "task not found", "")
		return
	}

	// 从 PrivateData 提取上游任务 ID
	var private struct {
		UpstreamTaskID string `json:"upstream_task_id"`
	}
	if err := json.Unmarshal(task.PrivateData, &private); err != nil || private.UpstreamTaskID == "" {
		writeTaskError(writer, http.StatusInternalServerError, "invalid task data", "")
		return
	}

	// 获取渠道信息
	channel, err := dataProvider.GetChannelByID(ctx, task.ChannelID)
	if err != nil || channel == nil {
		writeTaskError(writer, http.StatusInternalServerError, "channel not found", "")
		return
	}

	// 代理获取图片
	resp, err := midjourney.FetchImage(channel.BaseURL, channel.ApiKey, private.UpstreamTaskID)
	if err != nil {
		writeTaskError(writer, http.StatusBadGateway, "fetch image failed: "+err.Error(), "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeTaskError(writer, resp.StatusCode, "upstream image fetch failed", "")
		return
	}

	// 透传 Content-Type
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		writer.Header().Set("Content-Type", ct)
	}
	writer.WriteHeader(http.StatusOK)
	io.Copy(writer, resp.Body)
}
