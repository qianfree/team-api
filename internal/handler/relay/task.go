package relay

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/logic/billing"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/monitor"
	"github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/internal/logic/task"
	"github.com/qianfree/team-api/internal/middleware"
	relay_common "github.com/qianfree/team-api/relay/common"
	relay_constant "github.com/qianfree/team-api/relay/constant"
	relay_handler "github.com/qianfree/team-api/relay/handler"
)

var (
	taskDataProvider    = task.DefaultAsyncProvider
	taskBillingProvider = billing.NewTaskBillingProvider()
	relayDataProvider   = relay.NewDataProvider()
)

// HandleTaskSubmit 处理异步任务提交（POST /v1/video/generations, POST /suno/submit/:action）
func HandleTaskSubmit(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteStatus(400, g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "request body is empty"},
		})
		return
	}

	rc := &relay_handler.TaskRelayContext{
		TenantID:        middleware.GetTenantID(r.Context()),
		UserID:          middleware.GetUserID(r.Context()),
		ApiKeyID:        middleware.GetApiKeyID(r.Context()),
		ProjectID:       middleware.GetProjectID(r.Context()),
		RequestID:       r.GetCtxVar("RequestId").String(),
		Writer:          r.Response.Writer,
		Scope:           r.GetCtxVar("ApiKeyScope").String(),
		ClientIP:        r.GetClientIp(),
		KeyRateLimitQps: r.GetCtxVar(middleware.CtxKeyApiKeyRateLimitQps).Int(),
		KeyConcurrency:  r.GetCtxVar(middleware.CtxKeyApiKeyRateLimitConcurrency).Int(),
		KeyIpWhitelist:  r.GetCtxVar(middleware.CtxKeyApiKeyIpWhitelist).String(),
		KeyTotalQuota:   r.GetCtxVar(middleware.CtxKeyApiKeyTotalQuota).Float64(),
		KeyUsedQuota:    r.GetCtxVar(middleware.CtxKeyApiKeyUsedQuota).Float64(),
	}

	// 选择渠道
	channelMeta, err := selectTaskChannel(r, body)
	if err != nil {
		statusCode := 503
		errType := "server_error"
		errMsg := err.Error()

		if err == relay_common.ErrTenantModelNotEnabled {
			statusCode = 403
			errType = "permission_denied"
			errMsg = "当前租户未启用该模型，请联系管理员"
		} else if err == relay_common.ErrMemberModelNotAllowed {
			statusCode = 403
			errType = "permission_denied"
			errMsg = "当前成员无权使用该模型"
		} else if err == relay_common.ErrChannelUnavailable {
			statusCode = 503
			errType = "server_error"
			errMsg = "该模型暂无可用的渠道，请稍后重试或联系管理员"
		}

		r.Response.WriteStatus(statusCode, g.Map{
			"error": g.Map{"type": errType, "message": errMsg},
		})
		return
	}

	// 记录任务提交审计日志（使用 capture 模式以支持异步写入）
	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	modelName := extractModelName(body)
	registerAsyncTask(rc.RequestID, rc.TenantID, rc.UserID, rc.ProjectID, modelName, channelMeta, r.URL.Path)

	relay_handler.HandleTaskSubmit(
		r.Context(), body, r.URL.Path, r.Header,
		rc, taskDataProvider, taskBillingProvider, channelMeta,
	)

	// 提交成功后切换为任务 ID 跟踪（任务生命周期远超 HTTP 请求）
	if rc.TaskID != "" {
		monitor.SwitchToTaskID(rc.RequestID, rc.TaskID)
	} else {
		monitor.UnregisterRequest(rc.RequestID)
	}

	// 构建转发路径追踪（任务提交阶段只有一次渠道选择）
	rc.ForwardingTrace = buildTaskForwardingTrace(r.URL.Path, body, channelMeta, capture.StatusCode())

	// 异步记录审计日志
	go recordTaskSubmitAudit(r, rc, capture, body)
}

// HandleTaskFetch 处理异步任务查询（GET /v1/video/generations/:id, GET /suno/fetch/:id）
func HandleTaskFetch(r *ghttp.Request) {
	taskID := extractTaskID(r)
	if taskID == "" {
		r.Response.WriteStatus(400, g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "task id is required"},
		})
		return
	}

	rc := &relay_handler.TaskRelayContext{
		TenantID:  middleware.GetTenantID(r.Context()),
		UserID:    middleware.GetUserID(r.Context()),
		ApiKeyID:  middleware.GetApiKeyID(r.Context()),
		ProjectID: middleware.GetProjectID(r.Context()),
		RequestID: r.GetCtxVar("RequestId").String(),
		Writer:    r.Response.Writer,
	}

	relay_handler.HandleTaskFetch(r.Context(), taskID, rc, taskDataProvider)
}

// HandleSunoFetchBatch 处理 Suno 批量查询（POST /suno/fetch）
func HandleSunoFetchBatch(r *ghttp.Request) {
	// Suno 批量查询使用标准 HandleTaskFetch 的逻辑
	// 前端传入 JSON body 包含 task_id 列表
	taskID := r.Get("task_id").String()
	if taskID != "" {
		rc := &relay_handler.TaskRelayContext{
			TenantID:  middleware.GetTenantID(r.Context()),
			UserID:    middleware.GetUserID(r.Context()),
			ApiKeyID:  middleware.GetApiKeyID(r.Context()),
			ProjectID: middleware.GetProjectID(r.Context()),
			RequestID: r.GetCtxVar("RequestId").String(),
			Writer:    r.Response.Writer,
		}
		relay_handler.HandleTaskFetch(r.Context(), taskID, rc, taskDataProvider)
		return
	}
	r.Response.WriteStatus(400, g.Map{
		"error": g.Map{"type": "invalid_request_error", "message": "task_id is required"},
	})
}

// extractTaskID 从 URL 路径中提取任务 ID
func extractTaskID(r *ghttp.Request) string {
	path := r.URL.Path
	// /v1/video/generations/:id
	if idx := strings.LastIndex(path, "/"); idx != -1 {
		id := path[idx+1:]
		if strings.HasPrefix(id, "task_") {
			return id
		}
	}
	// 尝试从路由参数获取
	return r.Get("task_id").String()
}

// selectTaskChannel 为异步任务选择渠道（复用实时请求的渠道调度逻辑）
func selectTaskChannel(r *ghttp.Request, body []byte) (*relay_common.ChannelMeta, error) {
	// 从请求体提取模型名
	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.Model == "" {
		return nil, relay_common.ErrChannelUnavailable
	}

	ctx := r.Context()
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)

	selection, err := relayDataProvider.GetChannelForModel(ctx, tenantID, userID, req.Model, nil)
	if err != nil {
		return nil, err
	}

	// 检查成员模型范围
	if allowed, err := relayDataProvider.CheckMemberModelAccess(ctx, tenantID, userID, req.Model); err != nil {
		return nil, err
	} else if !allowed {
		return nil, relay_common.ErrMemberModelNotAllowed
	}

	return &relay_common.ChannelMeta{
		ChannelID:         selection.ChannelID,
		ChannelType:       selection.ChannelType,
		ChannelName:       selection.ChannelName,
		BaseURL:           selection.BaseURL,
		ApiKey:            selection.ApiKey,
		UpstreamModelName: selection.UpstreamModelName,
		IsModelMapped:     selection.IsModelMapped,
		Settings:          selection.Settings,
	}, nil
}

// HandleMjSubmit 处理 Midjourney 任务提交（POST /mj/submit/:action）
func HandleMjSubmit(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteStatus(400, g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "request body is empty"},
		})
		return
	}

	rc := &relay_handler.TaskRelayContext{
		TenantID:        middleware.GetTenantID(r.Context()),
		UserID:          middleware.GetUserID(r.Context()),
		ApiKeyID:        middleware.GetApiKeyID(r.Context()),
		ProjectID:       middleware.GetProjectID(r.Context()),
		RequestID:       r.GetCtxVar("RequestId").String(),
		Writer:          r.Response.Writer,
		Scope:           r.GetCtxVar("ApiKeyScope").String(),
		ClientIP:        r.GetClientIp(),
		KeyRateLimitQps: r.GetCtxVar(middleware.CtxKeyApiKeyRateLimitQps).Int(),
		KeyConcurrency:  r.GetCtxVar(middleware.CtxKeyApiKeyRateLimitConcurrency).Int(),
		KeyIpWhitelist:  r.GetCtxVar(middleware.CtxKeyApiKeyIpWhitelist).String(),
		KeyTotalQuota:   r.GetCtxVar(middleware.CtxKeyApiKeyTotalQuota).Float64(),
		KeyUsedQuota:    r.GetCtxVar(middleware.CtxKeyApiKeyUsedQuota).Float64(),
	}

	channelMeta, err := selectTaskChannel(r, body)
	if err != nil {
		r.Response.WriteStatus(503, g.Map{
			"error": g.Map{"type": "server_error", "message": err.Error()},
		})
		return
	}

	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	modelName := extractModelName(body)
	registerAsyncTask(rc.RequestID, rc.TenantID, rc.UserID, rc.ProjectID, modelName, channelMeta, r.URL.Path)

	relay_handler.HandleTaskSubmit(
		r.Context(), body, r.URL.Path, r.Header,
		rc, taskDataProvider, taskBillingProvider, channelMeta,
	)

	if rc.TaskID != "" {
		monitor.SwitchToTaskID(rc.RequestID, rc.TaskID)
	} else {
		monitor.UnregisterRequest(rc.RequestID)
	}

	rc.ForwardingTrace = buildTaskForwardingTrace(r.URL.Path, body, channelMeta, capture.StatusCode())
	go recordTaskSubmitAudit(r, rc, capture, body)
}

// HandleAliImageSubmit 处理异步图片生成任务提交（POST /v1/images/generations/async）
func HandleAliImageSubmit(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteStatus(400, g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "request body is empty"},
		})
		return
	}

	rc := &relay_handler.TaskRelayContext{
		TenantID:        middleware.GetTenantID(r.Context()),
		UserID:          middleware.GetUserID(r.Context()),
		ApiKeyID:        middleware.GetApiKeyID(r.Context()),
		ProjectID:       middleware.GetProjectID(r.Context()),
		RequestID:       r.GetCtxVar("RequestId").String(),
		Writer:          r.Response.Writer,
		Scope:           r.GetCtxVar("ApiKeyScope").String(),
		ClientIP:        r.GetClientIp(),
		KeyRateLimitQps: r.GetCtxVar(middleware.CtxKeyApiKeyRateLimitQps).Int(),
		KeyConcurrency:  r.GetCtxVar(middleware.CtxKeyApiKeyRateLimitConcurrency).Int(),
		KeyIpWhitelist:  r.GetCtxVar(middleware.CtxKeyApiKeyIpWhitelist).String(),
		KeyTotalQuota:   r.GetCtxVar(middleware.CtxKeyApiKeyTotalQuota).Float64(),
		KeyUsedQuota:    r.GetCtxVar(middleware.CtxKeyApiKeyUsedQuota).Float64(),
	}

	channelMeta, err := selectTaskChannel(r, body)
	if err != nil {
		r.Response.WriteStatus(503, g.Map{
			"error": g.Map{"type": "server_error", "message": err.Error()},
		})
		return
	}

	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	modelName := extractModelName(body)
	registerAsyncTask(rc.RequestID, rc.TenantID, rc.UserID, rc.ProjectID, modelName, channelMeta, r.URL.Path)

	// 异步图片端点分流。判「是否真有异步图片上游」用 constant.IsAsyncImageModel（唯一真相源，
	// 与 sync gate / async_image 标记同源）——目前仅阿里 image-synthesis（wanx / qwen-image /
	// wan2.2-t2i 等）为 true，走 taskchannel 提交拿 task_id。
	//
	// 其余图片模型上游都是同步阻塞返回，交 sync_image worker 池包装成可轮询任务（worker 直连各自
	// 同步 channel.Adaptor）：阿里 multimodal（qwen-image-2.x/z-image/wan2.6-t2i/wan2.7-image）恒走；
	// OpenAI / Gemini(imagen) / 火山(seedream) 等同步厂商受「同步图片异步化」总开关约束。
	//
	// 不能用 ProviderTypeToTaskPlatform 笼统判「异步厂商」——Gemini/火山的 taskchannel 是视频
	//（veo/seedance）专用，把它们的图片模型送进去会被当视频构造请求而失败。
	providerType := relay_constant.ProviderType(channelMeta.ChannelType)
	isAliSyncMultimodal := providerType == relay_constant.ProviderAli &&
		relay_constant.IsAliSyncMultimodalImageModel(channelMeta.UpstreamModelName)
	syncImageEnabled := lcommon.Config().GetBool(r.Context(), "sync_image_async_enabled")

	switch {
	case relay_constant.IsAsyncImageModel(providerType, channelMeta.UpstreamModelName):
		// 真·异步图片上游（阿里 image-synthesis）：taskchannel 提交 + 轮询
		relay_handler.HandleTaskSubmit(
			r.Context(), body, r.URL.Path, r.Header,
			rc, taskDataProvider, taskBillingProvider, channelMeta,
		)
	case isAliSyncMultimodal || syncImageEnabled:
		// 同步阻塞图片：worker 池异步化（阿里 multimodal 恒走；其余同步厂商受开关约束）
		HandleSyncImageSubmit(r, body, rc, channelMeta)
	default:
		// 同步厂商 + 「同步图片异步化」关闭：异步端点不支持，提示改用同步端点
		writeSyncImageError(rc.Writer, 400, "async image generation is disabled for this model; call POST /v1/images/generations instead")
	}

	if rc.TaskID != "" {
		monitor.SwitchToTaskID(rc.RequestID, rc.TaskID)
	} else {
		monitor.UnregisterRequest(rc.RequestID)
	}

	rc.ForwardingTrace = buildTaskForwardingTrace(r.URL.Path, body, channelMeta, capture.StatusCode())
	go recordTaskSubmitAudit(r, rc, capture, body)
}

// registerAsyncTask 注册异步任务到实时监控
func registerAsyncTask(requestID string, tenantID, userID, projectID int64, modelName string, channelMeta *relay_common.ChannelMeta, path string) {
	monitor.RegisterRequest(&monitor.TrackedRequest{
		RequestID:   requestID,
		TenantID:    tenantID,
		UserID:      userID,
		ProjectID:   projectID,
		ModelName:   modelName,
		ChannelID:   channelMeta.ChannelID,
		ChannelName: channelMeta.ChannelName,
		IsStream:    false,
		StartTime:   time.Now(),
		Path:        path,
		IsAsyncTask: true,
	})
}

// HandleMjFetch 处理 Midjourney 任务查询（GET /mj/task/:id/fetch）
func HandleMjFetch(r *ghttp.Request) {
	taskID := r.Get("task_id").String()
	if taskID == "" {
		taskID = extractMjTaskID(r)
	}
	if taskID == "" {
		r.Response.WriteStatus(400, g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "task id is required"},
		})
		return
	}

	rc := &relay_handler.TaskRelayContext{
		TenantID:  middleware.GetTenantID(r.Context()),
		UserID:    middleware.GetUserID(r.Context()),
		ApiKeyID:  middleware.GetApiKeyID(r.Context()),
		ProjectID: middleware.GetProjectID(r.Context()),
		RequestID: r.GetCtxVar("RequestId").String(),
		Writer:    r.Response.Writer,
	}

	relay_handler.HandleTaskFetch(r.Context(), taskID, rc, taskDataProvider)
}

// HandleMjImage 处理 Midjourney 图片代理（GET /mj/image/:id）
func HandleMjImage(r *ghttp.Request) {
	taskID := r.Get("task_id").String()
	if taskID == "" {
		taskID = extractMjTaskID(r)
	}
	if taskID == "" {
		r.Response.WriteStatus(400, g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "task id is required"},
		})
		return
	}

	rc := &relay_handler.TaskRelayContext{
		TenantID:  middleware.GetTenantID(r.Context()),
		UserID:    middleware.GetUserID(r.Context()),
		ApiKeyID:  middleware.GetApiKeyID(r.Context()),
		ProjectID: middleware.GetProjectID(r.Context()),
		RequestID: r.GetCtxVar("RequestId").String(),
		Writer:    r.Response.Writer,
	}

	relay_handler.HandleMjImageProxy(r.Context(), taskID, rc, taskDataProvider, r.Response.Writer)
}

// extractMjTaskID 从 MJ URL 路径提取任务 ID
func extractMjTaskID(r *ghttp.Request) string {
	path := r.URL.Path
	parts := strings.Split(path, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.HasPrefix(parts[i], "task_") {
			return parts[i]
		}
	}
	return ""
}

// recordTaskSubmitAudit 记录异步任务提交的审计日志
func recordTaskSubmitAudit(r *ghttp.Request, rc *relay_handler.TaskRelayContext, capture *ResponseCaptureWriter, body []byte) {
	relayDataProvider.RecordAudit(context.Background(), &relay_common.AuditRecord{
		TenantID:        rc.TenantID,
		UserID:          rc.UserID,
		ApiKeyID:        rc.ApiKeyID,
		ProjectID:       rc.ProjectID,
		RequestID:       rc.RequestID,
		Method:          "POST",
		Path:            r.URL.Path,
		QueryParams:     r.URL.RawQuery,
		StatusCode:      capture.StatusCode(),
		ClientIP:        rc.ClientIP,
		UserAgent:       r.Header.Get("User-Agent"),
		RequestBody:     string(body),
		ResponseBody:    capture.Body(),
		IsStream:        false,
		RequestHeaders:  captureRequestHeaders(r),
		ResponseHeaders: capture.ResponseHeaders(),
		TaskID:          rc.TaskID,
		TaskStatus:      "SUBMITTED",
		ForwardingTrace: rc.ForwardingTrace,
	})
}

// buildTaskForwardingTrace 从渠道选择结果构建转发路径追踪
func buildTaskForwardingTrace(path string, body []byte, channelMeta *relay_common.ChannelMeta, statusCode int) *relay_common.ForwardingTrace {
	var modelName string
	var req struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(body, &req) == nil {
		modelName = req.Model
	}

	providerName := ""
	if pt := relay_constant.ProviderType(channelMeta.ChannelType); pt > 0 {
		providerName = pt.String()
	}

	return &relay_common.ForwardingTrace{
		EntryPath:      path,
		EntryFormat:    "openai",
		RequestedModel: modelName,
		UpstreamModel:  channelMeta.UpstreamModelName,
		ModelMapped:    channelMeta.IsModelMapped,
		TotalAttempts:  1,
		Hops: []relay_common.ForwardingHop{
			{
				Attempt:       1,
				ChannelID:     channelMeta.ChannelID,
				ChannelName:   channelMeta.ChannelName,
				ChannelType:   channelMeta.ChannelType,
				Provider:      providerName,
				BaseURL:       channelMeta.BaseURL,
				UpstreamModel: channelMeta.UpstreamModelName,
				ModelMapped:   channelMeta.IsModelMapped,
				Success:       statusCode == 200,
			},
		},
	}
}

// extractModelName 从请求体中提取模型名称
func extractModelName(body []byte) string {
	var req struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(body, &req) == nil {
		return req.Model
	}
	return ""
}
