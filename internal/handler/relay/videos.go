package relay

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/logic/monitor"
	"github.com/qianfree/team-api/internal/middleware"
	relay_constant "github.com/qianfree/team-api/relay/constant"
	relay_handler "github.com/qianfree/team-api/relay/handler"
)

// OpenAI Videos 协议入站胶水（/v1/videos 端点）：
// 创建请求在此完成 multipart/JSON 解析与通用任务体转换，复用任务提交管线
// （selectTaskChannel 渠道调度 + relay_handler.HandleTaskSubmit 计费/提交/落库）；
// 查询/删除/内容下载直连 relay_handler 的 Videos 处理器。

// HandleVideoCreate 处理 POST /v1/videos（创建视频生成任务）
func HandleVideoCreate(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteStatus(400, g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "request body is empty"},
		})
		return
	}

	// OpenAI Videos 请求（multipart/JSON）→ 归一结构（官方 SDK 一律 multipart，必须支持）
	req, taskErr := relay_handler.ParseVideosCreateRequest(body, r.Header.Get("Content-Type"))
	if taskErr != nil {
		writeVideosError(r, taskErr.StatusCode, taskErr.Message, taskErr.ErrCode)
		return
	}
	if taskErr := relay_handler.ValidateVideosCreateRequest(req); taskErr != nil {
		writeVideosError(r, taskErr.StatusCode, taskErr.Message, taskErr.ErrCode)
		return
	}

	// 转成任务框架通用请求体（适配器消费 model/prompt/seconds/metadata）
	taskBody, err := relay_handler.BuildVideosTaskBody(req)
	if err != nil {
		writeVideosError(r, 500, "build task body failed", "")
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
		// OpenAI Videos 协议：官方 Video 对象响应 + video_ 前缀 ID + 请求回显落库
		Protocol:    "openai_videos",
		RelayMode:   int(relay_constant.RelayModeVideos),
		RequestEcho: relay_handler.NewVideosRequestEcho(req),
	}

	// 选择渠道（模型权限校验 + 调度引擎，与 /v1/video/generations 同一链路）
	channelMeta, err := selectTaskChannel(r, taskBody)
	if err != nil {
		writeTaskChannelSelectionError(r, err)
		return
	}

	// 响应捕获用于审计（对齐 HandleTaskSubmit 模式）
	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	registerAsyncTask(rc.RequestID, rc.TenantID, rc.UserID, rc.ProjectID, req.Model, channelMeta, r.URL.Path)

	relay_handler.HandleTaskSubmit(
		r.Context(), taskBody, r.URL.Path, r.Header,
		rc, taskDataProvider, taskBillingProvider, channelMeta,
	)

	// 提交成功后切换为任务 ID 跟踪（任务生命周期远超 HTTP 请求）
	if rc.TaskID != "" {
		monitor.SwitchToTaskID(rc.RequestID, rc.TaskID)
	} else {
		monitor.UnregisterRequest(rc.RequestID)
	}

	rc.ForwardingTrace = buildTaskForwardingTrace(r.URL.Path, taskBody, channelMeta, capture.StatusCode())
	go recordTaskSubmitAudit(r, rc, capture, taskBody)
}

// HandleVideoRetrieve 处理 GET /v1/videos/{video_id}（查询视频任务）
func HandleVideoRetrieve(r *ghttp.Request) {
	videoID := r.Get("video_id").String()
	if videoID == "" {
		writeVideosError(r, 400, "video id is required", "")
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

	relay_handler.HandleVideosRetrieve(r.Context(), videoID, rc, taskDataProvider)
}

// HandleVideoDelete 处理 DELETE /v1/videos/{video_id}（删除视频任务）
func HandleVideoDelete(r *ghttp.Request) {
	videoID := r.Get("video_id").String()
	if videoID == "" {
		writeVideosError(r, 400, "video id is required", "")
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

	relay_handler.HandleVideosDelete(r.Context(), videoID, rc, taskDataProvider)
}

// HandleVideoContent 处理 GET /v1/videos/{video_id}/content（下载成品视频）
func HandleVideoContent(r *ghttp.Request) {
	videoID := r.Get("video_id").String()
	if videoID == "" {
		writeVideosError(r, 400, "video id is required", "")
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

	relay_handler.HandleVideosContent(r.Context(), videoID, r.Get("variant", "video").String(), rc, taskDataProvider)
}

// writeVideosError OpenAI 官方错误格式输出（与 writeTaskError 同构，供胶水层解析阶段使用）
func writeVideosError(r *ghttp.Request, statusCode int, message, code string) {
	errObj := g.Map{
		"type":    "invalid_request_error",
		"message": message,
	}
	if code != "" {
		errObj["code"] = code
	}
	r.Response.WriteStatus(statusCode, g.Map{"error": errObj})
}
