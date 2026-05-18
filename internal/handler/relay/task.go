package relay

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/internal/logic/task"
	"github.com/qianfree/team-api/internal/middleware"
	"github.com/qianfree/team-api/relay/common"
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
		TenantID:  middleware.GetTenantID(r.Context()),
		UserID:    middleware.GetUserID(r.Context()),
		ApiKeyID:  middleware.GetApiKeyID(r.Context()),
		RequestID: r.GetCtxVar("RequestId").String(),
		Writer:    r.Response.Writer,
		Scope:     r.GetCtxVar("ApiKeyScope").String(),
		ClientIP:  r.GetClientIp(),
	}

	// 选择渠道
	channelMeta, err := selectTaskChannel(r, body)
	if err != nil {
		statusCode := 503
		errType := "server_error"
		errMsg := err.Error()

		if err == common.ErrTenantModelNotEnabled {
			statusCode = 403
			errType = "permission_denied"
			errMsg = "当前租户未启用该模型，请联系管理员"
		} else if err == common.ErrChannelUnavailable {
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

	relay_handler.HandleTaskSubmit(
		r.Context(), body, r.URL.Path, r.Header,
		rc, taskDataProvider, taskBillingProvider, channelMeta,
	)

	// 异步记录审计日志
	tenantID := rc.TenantID
	userID := rc.UserID
	apiKeyID := rc.ApiKeyID
	requestID := rc.RequestID
	path := r.URL.Path
	clientIP := rc.ClientIP
	userAgent := r.Header.Get("User-Agent")
	statusCode := capture.StatusCode()
	responseBody := capture.Body()
	go func() {
		relayDataProvider.RecordAudit(context.Background(), &common.AuditRecord{
			TenantID:     tenantID,
			UserID:       userID,
			ApiKeyID:     apiKeyID,
			RequestID:    requestID,
			Method:       "POST",
			Path:         path,
			StatusCode:   statusCode,
			ClientIP:     clientIP,
			UserAgent:    userAgent,
			RequestBody:  string(body),
			ResponseBody: responseBody,
		})
	}()
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
func selectTaskChannel(r *ghttp.Request, body []byte) (*common.ChannelMeta, error) {
	// 从请求体提取模型名
	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.Model == "" {
		return nil, common.ErrChannelUnavailable
	}

	ctx := r.Context()
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)

	selection, err := relayDataProvider.GetChannelForModel(ctx, tenantID, userID, req.Model, nil)
	if err != nil {
		return nil, err
	}

	return &common.ChannelMeta{
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
		TenantID:  middleware.GetTenantID(r.Context()),
		UserID:    middleware.GetUserID(r.Context()),
		ApiKeyID:  middleware.GetApiKeyID(r.Context()),
		RequestID: r.GetCtxVar("RequestId").String(),
		Writer:    r.Response.Writer,
		Scope:     r.GetCtxVar("ApiKeyScope").String(),
		ClientIP:  r.GetClientIp(),
	}

	channelMeta, err := selectTaskChannel(r, body)
	if err != nil {
		r.Response.WriteStatus(503, g.Map{
			"error": g.Map{"type": "server_error", "message": err.Error()},
		})
		return
	}

	relay_handler.HandleTaskSubmit(
		r.Context(), body, r.URL.Path, r.Header,
		rc, taskDataProvider, taskBillingProvider, channelMeta,
	)
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
