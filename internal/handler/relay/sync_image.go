package relay

import (
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/task"
	relay_common "github.com/qianfree/team-api/relay/common"
	relay_constant "github.com/qianfree/team-api/relay/constant"
	relay_handler "github.com/qianfree/team-api/relay/handler"
)

// HandleSyncImageSubmit 同步图片厂商的「异步化」提交管线。
//
// 由 HandleAliImageSubmit 在识别到同步厂商（ProviderTypeToTaskPlatform 未命中）时调用：
// 执行 gate → 强制非流式 → 估价 → 预扣 → 建 QUEUED 任务行 → 入 worker 队列，
// 客户端毫秒级拿到 task_id 后即释放连接，真正的上游长连接由 task 包的 worker 池后台持有。
//
// 与 relay_handler.HandleTaskSubmit 的区别：不在提交阶段调用上游（同步厂商无上游任务 ID），
// 渠道也不在提交时绑定（channel_id=0，worker dispatch 时按最新健康度/容量重选）。
func HandleSyncImageSubmit(r *ghttp.Request, body []byte, rc *relay_handler.TaskRelayContext, channelMeta *relay_common.ChannelMeta) {
	ctx := r.Context()
	w := rc.Writer

	modelName := extractModelName(body)
	if modelName == "" {
		writeSyncImageError(w, http.StatusBadRequest, "model is required")
		return
	}

	// 1. gate（复刻 HandleTaskSubmit 的 scope / IP 白名单 / 限流 / key 并发）
	if rc.Scope == "read_only" {
		writeSyncImageError(w, http.StatusForbidden, "API key scope denied")
		return
	}
	if !checkSyncImageIPWhitelist(rc.KeyIpWhitelist, rc.ClientIP) {
		writeSyncImageError(w, http.StatusForbidden, "IP address is not allowed")
		return
	}
	if allowed, level, _, _, _ := taskBillingProvider.CheckRateLimit(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, rc.KeyRateLimitQps); !allowed {
		writeSyncImageError(w, http.StatusTooManyRequests, fmt.Sprintf("rate limit exceeded at %s level", level))
		return
	}
	if !taskBillingProvider.AcquireApiKeyConcurrent(ctx, rc.ApiKeyID, rc.KeyConcurrency) {
		writeSyncImageError(w, http.StatusTooManyRequests, "API key concurrent request limit exceeded")
		return
	}
	// 提交很快即返回，并发槽在提交结束即释放（上游长连接由 worker 持有，不占此槽）。
	defer taskBillingProvider.ReleaseApiKeyConcurrent(ctx, rc.ApiKeyID)

	// Fast-fail：对象存储未配置、且本请求**必然**需要存储保存结果（b64 结果 / 强制 re-host）时，
	// 提交阶段即返回友好提示——避免白打一次上游生成（生成成功却因无处保存而失败退款，且用户
	// 要空等 10–60s 才知道是配置问题）。url 透传类请求（未开启 re-host 且非 b64）不需要存储，
	// 不在此拦截，仍可正常完成。
	if !lcommon.IsStorageConfigured(ctx) {
		rehostOn := lcommon.Config().GetBool(ctx, "sync_image_rehost_url")
		if rehostOn || requestForcesB64(body, modelName) {
			writeSyncImageError(w, http.StatusServiceUnavailable,
				"平台尚未配置对象存储（OSS/S3/COS），无法保存生成的图片，请联系管理员在系统设置中配置对象存储后重试")
			return
		}
	}

	// 2. 强制非流式：剥离 stream 字段，防 DoResponse 走 SSE 分支
	cleanBody := stripStreamField(body)

	// 3. 估价（图片走 per_request 价）
	estimatedCost, err := taskBillingProvider.EstimateTaskCost(ctx, rc.TenantID, modelName, nil)
	if err != nil {
		writeSyncImageError(w, http.StatusInternalServerError, "estimate cost failed: "+err.Error())
		return
	}

	// 4. 配额 + 预扣
	if err := taskBillingProvider.CheckApiKeyQuota(ctx, rc.ApiKeyID, estimatedCost); err != nil {
		writeSyncImageError(w, http.StatusPaymentRequired, "API key quota exceeded")
		return
	}
	preDeduct, err := taskBillingProvider.PreDeductTask(ctx, rc.TenantID, rc.RequestID, estimatedCost, modelName)
	if err != nil {
		writeSyncImageError(w, http.StatusPaymentRequired, "insufficient balance")
		return
	}

	// 5. 建 QUEUED 任务行（channel_id=0，dispatch 时绑定）
	publicTaskID := generateSyncImagePublicID()
	now := time.Now()

	// 从渠道类型推导实际的供应商平台（ali / gemini / volcengine / openai 等），用于任务日志展示。
	providerType := relay_constant.ProviderType(channelMeta.ChannelType)
	actualPlatform, ok := relay_constant.ProviderTypeToTaskPlatform(providerType)
	if !ok {
		// 未命中映射（如 openai、claude 等）：用供应商类型的可读名作平台名。
		// 注意不能写 TaskPlatform(providerType)——那是 int→string 的 rune 转换，会得到
		// 不可见控制字符（如 providerType=1 → "\x01"）而非 "openai"。取 String() 并转小写，
		// 与已映射平台（"ali"/"gemini"）的小写风格保持一致。
		actualPlatform = relay_constant.TaskPlatform(strings.ToLower(providerType.String()))
	}

	privateData, _ := json.Marshal(map[string]any{
		"task_type": string(relay_constant.TaskPlatformSyncImage),
		"billing_context": map[string]any{
			"ratios":     nil,
			"model_name": modelName,
			// pre_deduct 快照落 JSONB，读取端为 float64；decimal 默认带引号 MarshalJSON
			// 会导致读取端反序列化失败，故显式转 float64。
			"pre_deduct": preDeduct.InexactFloat64(),
		},
	})
	asyncTask := &relay_common.AsyncTask{
		PublicTaskID:    publicTaskID,
		RequestID:       rc.RequestID,
		Platform:        string(actualPlatform),
		Action:          "generate",
		Status:          "QUEUED",
		Progress:        "0%",
		TenantID:        rc.TenantID,
		UserID:          rc.UserID,
		ApiKeyID:        rc.ApiKeyID,
		ChannelID:       0,
		ModelName:       modelName,
		PreDeductAmount: preDeduct,
		PrivateData:     privateData,
		SubmitTime:      &now,
	}
	if err := taskDataProvider.CreateTask(ctx, asyncTask); err != nil {
		_ = taskBillingProvider.SettleTaskFailed(ctx, rc.TenantID, rc.RequestID, preDeduct)
		writeSyncImageError(w, http.StatusInternalServerError, "create task record failed: "+err.Error())
		return
	}

	// CreateTask 不回填自增 ID（Postgres 无 LastInsertId），回查取任务行主键供 worker CAS 使用。
	created, err := taskDataProvider.GetTaskByPublicID(ctx, publicTaskID)
	if err != nil || created == nil {
		// 此时任务行已 CreateTask 成功落库（QUEUED、billing_settled=false、active 计数已 +1），
		// 但缺主键无法在这里安全地 CAS 收尾。**不能**在此直接退款：UnfreezePreDeduct 非幂等，
		// 与超时兜底网 handleTimedOutTasks 会形成二次退款（侵蚀他人冻结额、可用余额虚高）。
		// 故此处只返回错误，把「CAS→FAILURE + 退款 + DecrActiveTask」交给超时兜底网做恰好一次结算
		// （handleTimedOutTasks 无平台过滤，会捕获这类 QUEUED 孤儿行）。
		g.Log().Errorf(ctx, "sync_image: load created task %s failed, deferring settlement to timeout sweeper: %v", publicTaskID, err)
		writeSyncImageError(w, http.StatusInternalServerError, "create task record failed")
		return
	}
	rc.TaskID = publicTaskID

	// 6. 入队（非阻塞）；队列满 → 退款 + CAS→FAILURE + DecrActive + 429
	job := &task.SyncImageJob{
		TaskID:          created.ID,
		PublicTaskID:    publicTaskID,
		RequestID:       rc.RequestID,
		TenantID:        rc.TenantID,
		UserID:          rc.UserID,
		ApiKeyID:        rc.ApiKeyID,
		ProjectID:       rc.ProjectID,
		Model:           modelName,
		RequestBody:     cleanBody,
		PreDeductAmount: preDeduct,
		Ratios:          nil,
		SubmitTime:      now,
	}
	if !task.EnqueueSyncImageJob(job) {
		_ = taskBillingProvider.SettleTaskFailed(ctx, rc.TenantID, rc.RequestID, preDeduct)
		_ = taskDataProvider.UpdateTaskCAS(ctx, &relay_common.AsyncTask{
			ID:             created.ID,
			Status:         "FAILURE",
			FailReason:     "sync image worker queue full",
			BillingSettled: true,
			FinishTime:     &now,
		}, "QUEUED")
		task.DecrActiveTask()
		rc.TaskID = ""
		writeSyncImageError(w, http.StatusTooManyRequests, "server busy, please retry later")
		return
	}

	// 7. 返回 QUEUED（客户端连接在此释放）
	writeSyncImageJSON(w, http.StatusOK, map[string]any{
		"id":         publicTaskID,
		"status":     "QUEUED",
		"model":      modelName,
		"created_at": now.Unix(),
	})
}

// requestForcesB64 判断该图片请求是否**必然**产生需要 re-host 的 base64 结果，即使
// 未开启 re-host 开关也一定用到对象存储：
//   - response_format=b64_json：显式要求 base64；
//   - gpt-image 系列：上游只返回 b64_json（不支持 url）。
//
// 其余情况（默认 / response_format=url）在未开启 re-host 时可 url 透传，不强制需要存储，
// 因此不在提交阶段 fast-fail，避免误伤纯 url 透传的模型。
func requestForcesB64(body []byte, model string) bool {
	var req struct {
		ResponseFormat string `json:"response_format"`
	}
	_ = json.Unmarshal(body, &req)
	if strings.EqualFold(strings.TrimSpace(req.ResponseFormat), "b64_json") {
		return true
	}
	return strings.HasPrefix(strings.ToLower(model), "gpt-image")
}

// stripStreamField 移除请求体中的 stream 字段（强制非流式）。
func stripStreamField(body []byte) []byte {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}
	if _, ok := m["stream"]; !ok {
		return body
	}
	delete(m, "stream")
	cleaned, err := json.Marshal(m)
	if err != nil {
		return body
	}
	return cleaned
}

// checkSyncImageIPWhitelist 复刻 HandleTaskSubmit 的 IP 白名单校验（支持精确 IP 与 CIDR）。
func checkSyncImageIPWhitelist(whitelist, clientIP string) bool {
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
			if _, cidr, err := net.ParseCIDR(item); err == nil && cidr.Contains(parsedIP) {
				return true
			}
		}
	}
	return false
}

func generateSyncImagePublicID() string {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return "task_" + hex.EncodeToString(b)
}

func writeSyncImageError(w http.ResponseWriter, statusCode int, message string) {
	writeSyncImageJSON(w, statusCode, map[string]any{
		"error": map[string]any{
			"type":    "invalid_request_error",
			"message": message,
		},
	})
}

// writeSyncImageErrorWithCode 与 writeSyncImageError 相同，但在错误体中附带机器可读的 code，
// 供前端做条件分支（如在线体验「异步被禁用 → 优雅降级到同步端点」）。
func writeSyncImageErrorWithCode(w http.ResponseWriter, statusCode int, code, message string) {
	writeSyncImageJSON(w, statusCode, map[string]any{
		"error": map[string]any{
			"type":    "invalid_request_error",
			"code":    code,
			"message": message,
		},
	})
}

func writeSyncImageJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}
