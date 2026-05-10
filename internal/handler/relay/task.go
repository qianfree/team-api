package relay

import (
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/task"
	"github.com/qianfree/team-api/internal/middleware"
	"github.com/qianfree/team-api/relay/common"
	relay_handler "github.com/qianfree/team-api/relay/handler"
)

var (
	taskDataProvider    = task.DefaultAsyncProvider
	taskBillingProvider = billing.NewTaskBillingProvider()
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

// selectTaskChannel 为异步任务选择渠道
func selectTaskChannel(r *ghttp.Request, body []byte) (*common.ChannelMeta, error) {
	// 复用现有的渠道调度逻辑
	// 暂时返回一个占位符，后续集成到 scheduler
	return &common.ChannelMeta{
		ChannelID: 1,
		BaseURL:   "https://api.openai.com",
		ApiKey:    "",
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
