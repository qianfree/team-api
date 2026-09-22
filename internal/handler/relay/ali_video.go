package relay

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/logic/monitor"
	"github.com/qianfree/team-api/internal/middleware"
	relay_constant "github.com/qianfree/team-api/relay/constant"
	relay_handler "github.com/qianfree/team-api/relay/handler"
)

// 阿里 DashScope 官方视频协议入站胶水（/api/v1 端点）：
// 创建请求在此完成官方请求体解析与通用任务体转换，复用任务提交管线
// （selectTaskChannel 渠道调度 + relay_handler.HandleTaskSubmit 计费/提交/落库）；
// 查询直连 relay_handler 的阿里视频处理器（回放 DashScope 官方响应形态）。
// 协议文档：docs/协议文档/阿里系列/视频-wan3.0.md

// HandleAliVideoSubmit 处理 POST /api/v1/services/aigc/video-generation/video-synthesis
// （创建视频生成任务，wan2.x / wan3.0 通用）
func HandleAliVideoSubmit(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		writeVideosError(r, 400, "request body is empty", "")
		return
	}

	// 官方请求体 → 归一结构（input.media[] 原样保留）
	req, taskErr := relay_handler.ParseAliVideoCreateRequest(body)
	if taskErr != nil {
		writeVideosError(r, taskErr.StatusCode, taskErr.Message, taskErr.ErrCode)
		return
	}
	if taskErr := relay_handler.ValidateAliVideoCreateRequest(req); taskErr != nil {
		writeVideosError(r, taskErr.StatusCode, taskErr.Message, taskErr.ErrCode)
		return
	}

	// 转成任务框架通用请求体（适配器消费 media/metadata + 双写通用词汇）
	taskBody, err := relay_handler.BuildAliVideoTaskBody(req)
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
		// 阿里 DashScope 官方协议：{"output":{"task_id":...}} 响应 + task_ 前缀 ID
		// （协议标识与 relay/handler.aliVideoProtocol 对齐，对齐 videos.go 胶水层字面量惯例）
		Protocol:  "ali_native",
		RelayMode: int(relay_constant.RelayModeAliVideo),
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

// HandleAliVideoRetrieve 处理 GET /api/v1/tasks/{task_id}（查询视频任务，回放官方响应）
func HandleAliVideoRetrieve(r *ghttp.Request) {
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

	relay_handler.HandleAliVideoRetrieve(r.Context(), taskID, rc, taskDataProvider)
}
