package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relay/taskchannel"
	_ "github.com/qianfree/team-api/relay/taskchannel/ali"
	_ "github.com/qianfree/team-api/relay/taskchannel/gemini"
	_ "github.com/qianfree/team-api/relay/taskchannel/kling"
	"github.com/qianfree/team-api/relay/taskchannel/midjourney"
	_ "github.com/qianfree/team-api/relay/taskchannel/minimax"
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
	KeyRateLimitQps int
	KeyConcurrency  int
	KeyIpWhitelist  string
	KeyTotalQuota   float64
	KeyUsedQuota    float64
	ForwardingTrace *common.ForwardingTrace

	// Protocol 入站协议标识："" 默认（legacy 任务响应格式，task_ 前缀 ID）；
	// videosProtocolOpenAI 切换为 OpenAI Videos 协议（官方 Video 对象响应，video_ 前缀 ID）。
	Protocol string
	// RelayMode 任务 relay 模式：0 = 默认 RelayModeVideoGenerations（现有端点），
	// OpenAI Videos 入站传 RelayModeVideos 以区分审计与监控统计。
	RelayMode int
	// RequestEcho 协议层请求回显（OpenAI Videos 的 prompt/seconds/size），
	// 提交时随 PrivateData 落库，retrieve 时回显给客户端；nil = 无回显。
	RequestEcho any

	// Debug 渠道调试日志会话；nil = 渠道未开启调试（所有方法 nil-safe）。
	// 由 HandleTaskSubmit 在提交前创建，响应写完后经 FinalizeAndSubmit 补段4并提交
	Debug *common.DebugSession
}

func checkTaskIPWhitelist(whitelist string, clientIP string) bool {
	if whitelist == "" {
		return true
	}

	host, _, err := net.SplitHostPort(clientIP)
	if err != nil {
		host = clientIP
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")

	parsedIP := net.ParseIP(host)
	if parsedIP == nil {
		return false
	}

	for _, item := range strings.Split(whitelist, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if item == host {
			return true
		}
		if strings.Contains(item, "/") {
			_, cidr, err := net.ParseCIDR(item)
			if err == nil && cidr.Contains(parsedIP) {
				return true
			}
		}
	}

	return false
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
	start := time.Now()
	// 各出口的响应同步写完后走到 defer：补段4并提交调试记录（幂等、nil-safe）。
	// 必须用闭包——直接 defer 方法调用会在注册时求值 latency 与 rc.Debug（恒为 0/nil）
	defer func() {
		rc.Debug.FinalizeAndSubmit(time.Since(start).Milliseconds(), 0)
	}()

	// 0. QPS 限流检查（前置：只依赖认证上下文，超限请求在解析请求体之前被拒绝）
	allowed, limitLevel, _, _, _ := billingProvider.CheckRateLimit(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, rc.KeyRateLimitQps)
	if !allowed {
		writeTaskError(rc.Writer, http.StatusTooManyRequests, fmt.Sprintf("rate limit exceeded at %s level", limitLevel), "")
		return
	}

	// 1. 解析模型名
	var req map[string]json.RawMessage
	if err := json.Unmarshal(body, &req); err != nil {
		g.Log().Debugf(ctx, "HandleTaskSubmit: invalid request body, err=%v", err)
		writeTaskError(rc.Writer, http.StatusBadRequest, "invalid request body", "")
		return
	}

	modelName := ""
	if v, ok := req["model"]; ok {
		if err := json.Unmarshal(v, &modelName); err != nil {
			g.Log().Debugf(ctx, "HandleTaskSubmit: invalid model field, err=%v", err)
			writeTaskError(rc.Writer, http.StatusBadRequest, "invalid model field", "")
			return
		}
	}
	if modelName == "" {
		writeTaskError(rc.Writer, http.StatusBadRequest, "model is required", "")
		return
	}

	if rc.Scope == "read_only" {
		writeTaskError(rc.Writer, http.StatusForbidden, "API key scope denied", "")
		return
	}
	if rc.KeyIpWhitelist != "" && !checkTaskIPWhitelist(rc.KeyIpWhitelist, rc.ClientIP) {
		writeTaskError(rc.Writer, http.StatusForbidden, "IP address is not allowed", "")
		return
	}
	if !billingProvider.AcquireApiKeyConcurrent(ctx, rc.ApiKeyID, rc.KeyConcurrency) {
		writeTaskError(rc.Writer, http.StatusTooManyRequests, "API key concurrent request limit exceeded", "")
		return
	}
	defer billingProvider.ReleaseApiKeyConcurrent(ctx, rc.ApiKeyID)

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
		g.Log().Errorf(ctx, "HandleTaskSubmit: adaptor not found, providerType=%d, err=%v", providerType, err)
		writeTaskError(rc.Writer, http.StatusInternalServerError, err.Error(), "")
		return
	}

	g.Log().Debugf(ctx, "HandleTaskSubmit: modelName=%s, platform=%s, channelID=%d, channelType=%d, baseURL=%s, upstreamModel=%s", modelName, platform, channelMeta.ChannelID, channelMeta.ChannelType, channelMeta.BaseURL, channelMeta.UpstreamModelName)

	// 4. 构建 RelayInfo
	relayMode := rc.RelayMode
	if relayMode == 0 {
		relayMode = int(constant.RelayModeVideoGenerations)
	}

	// 渠道调试日志：开关开启且匹配目标过滤时创建会话（捕获段1 + 包装段4 writer），
	// 提交为单次尝试（retry_index=0），捕获器经 attemptCtx 注入传输层镜像段2/3。
	// writer 层级：审计 capture（外层，入口胶水包）← DebugClientWriter（内层），与同步链路同构
	var dbgAttempt *common.DebugAttempt
	if channelMeta.Settings.DebugLogEnabled &&
		channelMeta.Settings.DebugTargetMatch(rc.TenantID, rc.UserID, rc.ApiKeyID) {
		rc.Debug = common.NewDebugSession(rc.RequestID, rc.TenantID, rc.UserID, rc.ApiKeyID, path)
		rc.Debug.CaptureClientRequest(headers, body)
		dw := common.NewDebugClientWriter(rc.Writer)
		rc.Debug.SetClientWriter(dw)
		rc.Writer = dw
		dbgAttempt = rc.Debug.BeginTaskAttempt(channelMeta, modelName,
			relayModeString(constant.RelayMode(relayMode)), 0)
	}
	attemptCtx := ctx
	if dbgAttempt != nil {
		attemptCtx = common.WithDebugAttempt(ctx, dbgAttempt.Capture)
	}

	info := &common.RelayInfo{
		Context:         attemptCtx,
		TenantID:        rc.TenantID,
		UserID:          rc.UserID,
		ApiKeyID:        rc.ApiKeyID,
		RequestID:       rc.RequestID,
		RelayMode:       relayMode,
		OriginModelName: modelName,
		RequestURLPath:  path,
		RequestHeaders:  headers,
		StartTime:       time.Now(),
		ChannelMeta:     channelMeta,
	}
	adaptor.Init(info)
	dbgAttempt.CaptureProtocol(info)

	// 5. 校验请求
	if taskErr := adaptor.ValidateRequest(ctx, info, body); taskErr != nil {
		g.Log().Warningf(ctx, "HandleTaskSubmit: validate failed, model=%s, err=%s", modelName, taskErr.Message)
		writeTaskError(rc.Writer, taskErr.StatusCode, taskErr.Message, taskErr.ErrCode)
		return
	}

	// 6. 估算计费 + 预扣（ratios 为计费上下文：float64 乘数 + spec.* 规格事实值；
	// body 同传给 EstimateTaskCost 供参数倍率按归一化任务体匹配，命中注入 ratios）
	ratios := adaptor.EstimateBilling(ctx, info, body)
	estimatedCost, err := billingProvider.EstimateTaskCost(ctx, rc.TenantID, modelName, ratios, body)
	if err != nil {
		g.Log().Errorf(ctx, "HandleTaskSubmit: estimate cost failed, model=%s, err=%v", modelName, err)
		writeTaskError(rc.Writer, http.StatusInternalServerError, "estimate cost failed: "+err.Error(), "")
		return
	}
	g.Log().Debugf(ctx, "HandleTaskSubmit: estimatedCost=%.4f", estimatedCost.InexactFloat64())

	if err := billingProvider.CheckApiKeyQuota(ctx, rc.ApiKeyID, estimatedCost); err != nil {
		writeTaskError(rc.Writer, http.StatusPaymentRequired, "API key quota exceeded", "")
		return
	}

	preDeductAmount, err := billingProvider.PreDeductTask(ctx, rc.TenantID, rc.RequestID, estimatedCost, modelName)
	if err != nil {
		g.Log().Warningf(ctx, "HandleTaskSubmit: pre-deduct failed, model=%s, err=%v", modelName, err)
		writeTaskError(rc.Writer, http.StatusPaymentRequired, "insufficient balance", "")
		return
	}

	// 7. 构建并发送请求
	requestBody, err := adaptor.BuildRequestBody(ctx, info, body)
	if err != nil {
		if err := billingProvider.SettleTaskFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount); err != nil {
			g.Log().Errorf(ctx, "HandleTaskSubmit: SettleTaskFailed error, requestID=%s, amount=%.6f, err=%v", rc.RequestID, preDeductAmount.InexactFloat64(), err)
		}
		g.Log().Errorf(ctx, "HandleTaskSubmit: build request failed, model=%s, err=%v", modelName, err)
		dbgAttempt.MarkFinal(err)
		writeTaskError(rc.Writer, http.StatusInternalServerError, "build request failed: "+err.Error(), "")
		return
	}

	resp, err := adaptor.DoRequest(attemptCtx, info, requestBody)
	if err != nil {
		if err := billingProvider.SettleTaskFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount); err != nil {
			g.Log().Errorf(ctx, "HandleTaskSubmit: SettleTaskFailed error, requestID=%s, amount=%.6f, err=%v", rc.RequestID, preDeductAmount.InexactFloat64(), err)
		}
		// 上游请求失败（网络/超时/拒绝等）属预期内运营事件，非代码 bug：
		// 用 Warningf 避免 glog 对 ERROR+ 自动打印调用栈污染日志（与 :229 upstream response error 一致）。
		g.Log().Warningf(ctx, "HandleTaskSubmit: upstream request failed, model=%s, err=%v", modelName, err)
		dbgAttempt.MarkFinal(err)
		writeTaskError(rc.Writer, http.StatusBadGateway, helper.SafeUpstreamErrorMessage(err), "")
		return
	}
	defer resp.Body.Close()

	g.Log().Debugf(ctx, "HandleTaskSubmit: upstream responded status=%d", resp.StatusCode)

	// 8. 解析响应
	upstreamTaskID, taskData, taskErr := adaptor.DoResponse(ctx, resp, info)
	if taskErr != nil {
		if err := billingProvider.SettleTaskFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount); err != nil {
			g.Log().Errorf(ctx, "HandleTaskSubmit: SettleTaskFailed error, requestID=%s, amount=%.6f, err=%v", rc.RequestID, preDeductAmount.InexactFloat64(), err)
		}
		g.Log().Warningf(ctx, "HandleTaskSubmit: upstream response error, model=%s, status=%d, message=%q, body=%s", modelName, taskErr.StatusCode, taskErr.Message, string(taskData))
		dbgAttempt.MarkFinal(taskErr)
		writeTaskError(rc.Writer, taskErr.StatusCode, taskErr.Message, taskErr.ErrCode)
		return
	}

	// 9. 调整计费
	adjustedRatios := adaptor.AdjustBillingOnSubmit(info, taskData)
	if adjustedRatios != nil {
		newCost, _ := billingProvider.EstimateTaskCost(ctx, rc.TenantID, modelName, adjustedRatios, body)
		preDeductAmount, _ = billingProvider.AdjustTaskBilling(ctx, rc.TenantID, rc.RequestID, preDeductAmount, newCost)
	}

	// 9.5 合并最终 ratios 用于结算时还原
	finalRatios := ratios
	if adjustedRatios != nil {
		if finalRatios == nil {
			finalRatios = adjustedRatios
		} else {
			merged := make(map[string]any, len(finalRatios)+len(adjustedRatios))
			for k, v := range finalRatios {
				merged[k] = v
			}
			for k, v := range adjustedRatios {
				merged[k] = v
			}
			finalRatios = merged
		}
	}

	// 10. 生成公开任务 ID 并创建记录（OpenAI Videos 协议用 video_ 前缀对齐官方形态）
	publicTaskID := generatePublicTaskID()
	if rc.Protocol == videosProtocolOpenAI {
		publicTaskID = generatePublicTaskIDWithPrefix("video")
	}
	now := time.Now()

	privateDataMap := map[string]any{
		"upstream_task_id": upstreamTaskID,
		"task_type":        platform,
		"billing_context": map[string]any{
			"ratios":     finalRatios,
			"model_name": modelName,
			// pre_deduct 快照落 JSONB，读取端 privateData.PreDeduct 为 float64。
			// decimal 默认 MarshalJSON 带引号（"0.1"），会导致读取端反序列化整个 blob 失败，
			// 进而 Ratios 也读不出、轮询判为「invalid private data」。故此处显式转 float64。
			"pre_deduct": preDeductAmount.InexactFloat64(),
		},
	}
	if rc.RequestEcho != nil {
		privateDataMap["request_echo"] = rc.RequestEcho
	}
	privateData, _ := json.Marshal(privateDataMap)

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
		if err := billingProvider.SettleTaskFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount); err != nil {
			g.Log().Errorf(ctx, "HandleTaskSubmit: SettleTaskFailed error, requestID=%s, amount=%.6f, err=%v", rc.RequestID, preDeductAmount.InexactFloat64(), err)
		}
		g.Log().Errorf(ctx, "HandleTaskSubmit: create task record failed, publicTaskID=%s, model=%s, err=%v", publicTaskID, modelName, err)
		dbgAttempt.MarkFinal(err)
		writeTaskError(rc.Writer, http.StatusInternalServerError, "create task record failed: "+err.Error(), "")
		return
	}

	// 设置 TaskID 供外层审计使用
	rc.TaskID = publicTaskID
	// 调试日志：提交成功的最终尝试（段4 由 defer 的 FinalizeAndSubmit 补齐）
	dbgAttempt.MarkFinal(nil)

	// 11. 返回响应（OpenAI Videos 协议返回官方 Video 对象；MiniMax 官方协议返回 {"task_id"}
	//（v1 含 base_resp 信封）；阿里 DashScope 官方协议返回 {"output":{"task_id":...}}；
	// legacy 保持原格式）
	if rc.Protocol == videosProtocolOpenAI {
		writeVideosSubmitResponse(rc.Writer, publicTaskID, modelName, now, rc.RequestEcho)
		return
	}
	if rc.Protocol == minimaxVideoProtocol {
		writeMiniMaxSubmitResponse(rc.Writer, publicTaskID)
		return
	}
	if rc.Protocol == minimaxVideoV1Protocol {
		writeMiniMaxV1SubmitResponse(rc.Writer, publicTaskID)
		return
	}
	if rc.Protocol == aliVideoProtocol {
		writeAliVideoSubmitResponse(rc.Writer, publicTaskID, rc.RequestID, now)
		return
	}
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
	task, err := dataProvider.GetTaskByPublicIDAndUser(ctx, publicTaskID, rc.UserID, rc.TenantID)
	if err != nil {
		g.Log().Errorf(ctx, "HandleTaskFetch: query task failed, publicTaskID=%s, err=%v", publicTaskID, err)
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
	// sync_image「同步图片异步化」任务支持多图：把归一化的全部图片作为 data 数组吐出，
	// url 仍保留首图（向后兼容）。仅对 sync_image 任务开启——其 Data 为标准 OpenAI
	// ImageResponse；视频 / suno / mj 等平台的 Data 结构不同，不能误吐。
	if isSyncImageResultTask(task) && len(task.Data) > 0 {
		var img dto.ImageResponse
		if json.Unmarshal(task.Data, &img) == nil && len(img.Data) > 0 {
			respBody["data"] = img.Data
		}
	}
	if task.FailReason != "" {
		respBody["error"] = task.FailReason
	}
	if task.FinishTime != nil {
		respBody["completed_at"] = task.FinishTime.Unix()
	}

	writeJSON(rc.Writer, http.StatusOK, respBody)
}

// isSyncImageResultTask 判断任务是否为「同步图片异步化」任务（private_data.task_type == sync_image）。
// 仅这类任务的 Data 为归一化的 OpenAI ImageResponse，可安全地在 fetch 响应里吐 data 数组；
// 其他平台（视频 / suno / mj）的 Data 结构不同，不在此列，避免误吐内部数据。
func isSyncImageResultTask(t *common.AsyncTask) bool {
	if len(t.PrivateData) == 0 {
		return false
	}
	var pd struct {
		TaskType string `json:"task_type"`
	}
	if json.Unmarshal(t.PrivateData, &pd) != nil {
		return false
	}
	return pd.TaskType == string(constant.TaskPlatformSyncImage)
}

// writeTaskError 写入错误响应
func writeTaskError(w http.ResponseWriter, statusCode int, message, code string) {
	// 防御性兜底：抹除 message 中可能残留的上游 URL / IP（如 DoRequest 失败的 *url.Error、
	// MJ 图片代理的 http.Get 错误）。传输层域名已在主要泄露点（:215）经 SafeUpstreamErrorMessage 归一化。
	resp := map[string]any{
		"error": map[string]any{
			"type":    "invalid_request_error",
			"message": helper.RedactMessage(message),
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
	return fmt.Sprintf("task_%s", randomHex(32))
}

// generatePublicTaskIDWithPrefix 生成带指定前缀的公开任务 ID（如 OpenAI Videos 协议的 video_ 前缀）
func generatePublicTaskIDWithPrefix(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, randomHex(32))
}

func randomHex(n int) string {
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
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
	task, err := dataProvider.GetTaskByPublicIDAndUser(ctx, publicTaskID, rc.UserID, rc.TenantID)
	if err != nil {
		g.Log().Errorf(ctx, "HandleMjImageProxy: query task failed, publicTaskID=%s, err=%v", publicTaskID, err)
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
		g.Log().Warningf(ctx, "HandleMjImageProxy: invalid task data, publicTaskID=%s, err=%v", publicTaskID, err)
		writeTaskError(writer, http.StatusInternalServerError, "invalid task data", "")
		return
	}

	// 获取渠道信息
	channel, err := dataProvider.GetChannelByID(ctx, task.ChannelID)
	if err != nil || channel == nil {
		g.Log().Errorf(ctx, "HandleMjImageProxy: channel not found, channelID=%d, err=%v", task.ChannelID, err)
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
		writeTaskError(writer, resp.StatusCode, "图片获取失败", "")
		return
	}

	// 透传 Content-Type
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		writer.Header().Set("Content-Type", ct)
	}
	writer.WriteHeader(http.StatusOK)
	io.Copy(writer, resp.Body)
}
