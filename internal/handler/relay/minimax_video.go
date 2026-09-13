package relay

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/logic/monitor"
	"github.com/qianfree/team-api/internal/middleware"
	relay_constant "github.com/qianfree/team-api/relay/constant"
	relay_handler "github.com/qianfree/team-api/relay/handler"
)

// MiniMax 官方视频协议入站胶水（/v2 端点）：
// 创建请求在此完成官方请求体解析与通用任务体转换，复用任务提交管线
// （selectTaskChannel 渠道调度 + relay_handler.HandleTaskSubmit 计费/提交/落库）；
// 查询/删除直连 relay_handler 的 MiniMax 视频处理器。
// 协议文档：docs/modeldocs/minimax/video/

// HandleMiniMaxVideoSubmit 处理 POST /v2/video_generation（创建视频生成任务）
func HandleMiniMaxVideoSubmit(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		writeVideosError(r, 400, "request body is empty", "")
		return
	}

	// 官方请求体 → 归一结构（content[] 原样保留，派生 prompt）
	req, taskErr := relay_handler.ParseMiniMaxVideoCreateRequest(body)
	if taskErr != nil {
		writeVideosError(r, taskErr.StatusCode, taskErr.Message, taskErr.ErrCode)
		return
	}
	if taskErr := relay_handler.ValidateMiniMaxVideoCreateRequest(req); taskErr != nil {
		writeVideosError(r, taskErr.StatusCode, taskErr.Message, taskErr.ErrCode)
		return
	}

	// 转成任务框架通用请求体（适配器消费 content/resolution/duration + 双写通用词汇）
	taskBody, err := relay_handler.BuildMiniMaxVideoTaskBody(req)
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
		// MiniMax 官方协议：{"task_id"} 响应 + task_ 前缀 ID + 请求回显落库
		// （协议标识与 relay/handler.minimaxVideoProtocol 对齐，对齐 videos.go 胶水层字面量惯例）
		Protocol:    "minimax_native",
		RelayMode:   int(relay_constant.RelayModeMiniMaxVideo),
		RequestEcho: relay_handler.NewMiniMaxRequestEcho(req),
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

// HandleMiniMaxVideoRetrieve 处理 GET /v2/query/video_generation/{task_id}（查询视频任务）
func HandleMiniMaxVideoRetrieve(r *ghttp.Request) {
	taskID := r.Get("task_id").String()
	if taskID == "" {
		writeVideosError(r, 400, "task id is required", "")
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

	relay_handler.HandleMiniMaxVideoRetrieve(r.Context(), taskID, rc, taskDataProvider)
}

// HandleMiniMaxVideoCancel 处理 DELETE /v2/video_generation/{task_id}（取消排队中的视频任务）
func HandleMiniMaxVideoCancel(r *ghttp.Request) {
	taskID := r.Get("task_id").String()
	if taskID == "" {
		writeVideosError(r, 400, "task id is required", "")
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

	relay_handler.HandleMiniMaxVideoCancel(r.Context(), taskID, rc, taskDataProvider)
}

// HandleMiniMaxVideoV1Submit 处理 POST /v1/video_generation（MiniMax v1 官方协议创建，
// 文生/图生/首尾帧/主体参考四形态共用端点，扁平字段）
func HandleMiniMaxVideoV1Submit(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		writeVideosError(r, 400, "request body is empty", "")
		return
	}

	req, taskErr := relay_handler.ParseMiniMaxVideoV1CreateRequest(body)
	if taskErr != nil {
		writeVideosError(r, taskErr.StatusCode, taskErr.Message, taskErr.ErrCode)
		return
	}
	if taskErr := relay_handler.ValidateMiniMaxVideoV1CreateRequest(req); taskErr != nil {
		writeVideosError(r, taskErr.StatusCode, taskErr.Message, taskErr.ErrCode)
		return
	}

	taskBody, err := relay_handler.BuildMiniMaxVideoV1TaskBody(req)
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
		// MiniMax v1 官方协议：{task_id, base_resp} 响应 + task_ 前缀 ID + 请求回显落库
		Protocol:    "minimax_native_v1",
		RelayMode:   int(relay_constant.RelayModeMiniMaxVideo),
		RequestEcho: relay_handler.NewMiniMaxV1RequestEcho(req),
	}

	// 选择渠道（模型权限校验 + 调度引擎，与 /v1/video/generations 同一链路）
	channelMeta, err := selectTaskChannel(r, taskBody)
	if err != nil {
		writeTaskChannelSelectionError(r, err)
		return
	}

	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	registerAsyncTask(rc.RequestID, rc.TenantID, rc.UserID, rc.ProjectID, req.Model, channelMeta, r.URL.Path)

	relay_handler.HandleTaskSubmit(
		r.Context(), taskBody, r.URL.Path, r.Header,
		rc, taskDataProvider, taskBillingProvider, channelMeta,
	)

	if rc.TaskID != "" {
		monitor.SwitchToTaskID(rc.RequestID, rc.TaskID)
	} else {
		monitor.UnregisterRequest(rc.RequestID)
	}

	rc.ForwardingTrace = buildTaskForwardingTrace(r.URL.Path, taskBody, channelMeta, capture.StatusCode())
	go recordTaskSubmitAudit(r, rc, capture, taskBody)
}

// HandleMiniMaxVideoV1Retrieve 处理 GET /v1/query/video_generation?task_id=（v1 官方协议任务查询）
func HandleMiniMaxVideoV1Retrieve(r *ghttp.Request) {
	taskID := r.Get("task_id").String()
	if taskID == "" {
		writeVideosError(r, 400, "task id is required", "")
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

	relay_handler.HandleMiniMaxVideoV1Retrieve(r.Context(), taskID, rc, taskDataProvider)
}
