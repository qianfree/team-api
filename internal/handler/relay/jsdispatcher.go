// jsdispatcher.go JS 任务插件路由分发器（/pjs/{key}/{原生路径}）。
//
// 挂载方式：一条静态通配路由 + generation 原子快照匹配——运行期启停/换版插件
// 只重建 generation，永不触碰 GoFrame 路由表（gf 路由表无锁不可运行期写）。
// 提交链：先调插件 decodeRequest 得模型 → 注入 body 副本 → 复用 selectTaskChannel
// 与 relay_handler.HandleTaskSubmit 全链（预扣/提交/落库/审计不变）。
package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/jsplugin"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
	"github.com/qianfree/team-api/internal/logic/monitor"
	tasklogic "github.com/qianfree/team-api/internal/logic/task"
	taskartifact "github.com/qianfree/team-api/internal/logic/task/artifact"
	"github.com/qianfree/team-api/internal/middleware"
	relay_common "github.com/qianfree/team-api/relay/common"
	relay_constant "github.com/qianfree/team-api/relay/constant"
	relay_handler "github.com/qianfree/team-api/relay/handler"
)

// jsDispatcherPrefix JS 插件路由前缀（客户端入口：/pjs/{key}/...）。
const jsDispatcherPrefix = "/pjs"

// RegisterJSDispatcher 注册 /pjs 分发器（cmd.go 路由注册段调用，位于 relay 路由之后）。
func RegisterJSDispatcher(server *ghttp.Server) {
	server.Group(jsDispatcherPrefix, func(group *ghttp.RouterGroup) {
		// 与 /v1 relay 组同款鉴权与防护链
		group.Middleware(middleware.ApiMaintenance, middleware.ResourceGuard,
			middleware.MaintenanceMode, middleware.ApiKeyAuth, middleware.RelayBodyLimit, middleware.ContentFilter)
		group.ALL("/*path", dispatchJSPlugin)
	})
}

// dispatchJSPlugin /pjs/{key}/{native...} → generation 匹配 → submit/query 分流。
func dispatchJSPlugin(r *ghttp.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, jsDispatcherPrefix+"/")
	key, native := trimmed, "/"
	if idx := strings.Index(trimmed, "/"); idx >= 0 {
		key, native = trimmed[:idx], trimmed[idx:]
	}
	if key == "" {
		writeJSNotFound(r)
		return
	}

	plugin, route, params, ok := jsplugin.CurrentGeneration().Match(strings.ToUpper(r.Method), native)
	if !ok || plugin.Key != key {
		writeJSNotFound(r)
		return
	}

	switch route.Type {
	case contract.RouteTypeSubmit:
		handleJSTaskSubmit(r, plugin, route, params, native, nil)
	case contract.RouteTypeDynamic:
		handleJSDynamic(r, plugin, route, params, native)
	case contract.RouteTypeQuery:
		handleJSTaskQuery(r, plugin, route, params)
	default:
		writeJSNotFound(r)
	}
}

// handleJSDynamic dynamic 路由（一型两用）：解码结果定走向——kind=query 走存量任务
// 批量查询渲染（suno 式 POST /fetch 的 ids 列表），kind=submit 走标准提交流。
func handleJSDynamic(r *ghttp.Request, plugin *jsplugin.ActivePlugin, route contract.Route, params map[string]string, nativePath string) {
	body := r.GetBody()
	_, decodeFiles := parseUploadFiles(r)
	decodeCtx, _ := buildDecodeContext(r, plugin, route, params, nativePath, body, decodeFiles)
	decoded, ok := callDecodeHook(r, plugin, route, decodeCtx)
	if !ok {
		return // 错误已写出
	}
	if decoded.Kind == "query" {
		handleJSDynamicQuery(r, plugin, route, decoded.TaskIDs)
		return
	}
	handleJSTaskSubmit(r, plugin, route, params, nativePath, decoded)
}

// handleJSDynamicQuery 批量任务查询渲染：按公开任务 ID 取任务（仅本插件持有），
// 渲染钩子（别名/缺省 renderTasks）接收整批；未导出则返回默认任务视图数组。
func handleJSDynamicQuery(r *ghttp.Request, plugin *jsplugin.ActivePlugin, route contract.Route, taskIDs []string) {
	rc := &relay_handler.TaskRelayContext{
		TenantID:  middleware.GetTenantID(r.Context()),
		UserID:    middleware.GetUserID(r.Context()),
		ApiKeyID:  middleware.GetApiKeyID(r.Context()),
		RequestID: r.GetCtxVar("RequestId").String(),
		Writer:    r.Response.Writer,
	}
	tasks := make([]*relay_common.AsyncTask, 0, len(taskIDs))
	for _, id := range taskIDs {
		t, err := taskDataProvider.GetTaskByPublicIDAndUser(r.Context(), id, rc.UserID, rc.TenantID)
		// 不存在/未持有/跨插件的任务跳过（批量语义：部分命中不整体失败）
		if err != nil || t == nil || t.Platform != "js:"+plugin.Key {
			continue
		}
		tasks = append(tasks, t)
	}
	renderTasks := make([]contract.RenderTask, 0, len(tasks))
	for _, t := range tasks {
		renderTasks = append(renderTasks, buildRenderTask(r.Context(), t))
	}

	renderHook := route.ResolveRender("renderTasks")
	if plugin.HasHook(renderHook) {
		renderArgs, _ := json.Marshal(contract.RenderTasksContext{Tasks: renderTasks})
		out, err := plugin.Pool.Call(r.Context(), renderHook, renderArgs)
		if err != nil {
			writeJSHookError(r, err)
			return
		}
		raw, err := contract.ParseJSONObject(out)
		if err != nil {
			writeJSError(r, http.StatusBadGateway, renderHook+": "+err.Error())
			return
		}
		rend, err := contract.ValidateRendered(raw)
		if err != nil {
			writeJSError(r, http.StatusBadGateway, renderHook+": "+err.Error())
			return
		}
		// 状态码与内容须一次写入：WriteStatus 不带内容会把状态文本（如 "OK"）写进响应体
		r.Response.WriteStatus(rend.Status, []byte(rend.Body))
		return
	}

	// 默认渲染：任务关键信息数组
	list := make([]g.Map, 0, len(tasks))
	for _, t := range tasks {
		list = append(list, g.Map{
			"task_id": t.PublicTaskID, "status": t.Status, "progress": t.Progress,
			"fail_reason": t.FailReason, "data": json.RawMessage(orEmptyJSON(t.Data)),
		})
	}
	r.Response.WriteJson(g.Map{"list": list})
}

// handleJSTaskSubmit 提交链：decode → 注入模型 → 复用既有任务提交管线。
// preDecoded 非 nil 时跳过解码（dynamic 路由已解码为 submit 意图）。
func handleJSTaskSubmit(r *ghttp.Request, plugin *jsplugin.ActivePlugin, route contract.Route, params map[string]string, nativePath string, preDecoded *contract.DecodeResult) {
	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteStatus(400, g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "request body is empty"},
		})
		return
	}

	// 1) 插件解码（路径参数在此进入契约；钩子名取路由别名，回落 decodeRequest）。
	// multipart 提交先解析文件（FileReference：字节留宿主，解码侧只见引用）。
	planFiles, decodeFiles := parseUploadFiles(r)
	decoded := preDecoded
	if decoded == nil {
		var ok bool
		var decodeCtx contract.DecodeContext
		if decodeCtx, ok = buildDecodeContext(r, plugin, route, params, nativePath, body, decodeFiles); !ok {
			return
		}
		if decoded, ok = callDecodeHook(r, plugin, route, decodeCtx); !ok {
			return
		}
	}

	// 2) 模型注入 body 副本（selectTaskChannel/extractModelName 从 body.model 读）
	body2, err := injectModelField(body, decoded.Model)
	if err != nil {
		writeJSError(r, http.StatusBadRequest, "request body is not a JSON object")
		return
	}

	// 3) 解码计划挂 ctx：任务桥（jsbridge）在 ValidateRequest/buildSubmitRequest 消费
	plan := &jsplugin.SubmitPlan{
		Plugin:     plugin,
		Route:      route,
		Params:     params,
		NativePath: nativePath,
		Decoded:    decoded,
		Files:      planFiles,
	}
	ctx := jsplugin.WithSubmitPlan(r.Context(), plan)

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

	// 4) 渠道选择：候选约束为本插件的 JS 渠道——同名多插件（sharedModels）场景下，
	// 同一模型可能有多家插件的渠道，调度在候选层过滤而非选中后 400（顺带消灭了
	// 选中他家渠道时报晦涩绑定错误的旧路径）
	channelMeta, err := selectTaskChannelFor(r, body2, func(meta *relay_common.ChannelMeta) bool {
		return relay_constant.ProviderType(meta.ChannelType) == relay_constant.ProviderJSTask &&
			meta.Settings.PluginKey == plugin.Key
	})
	if err != nil {
		writeTaskChannelSelectionError(r, err)
		return
	}

	// renderSubmit 接线（hold-writer）：插件声明了提交渲染钩子时，标准任务响应被扣住，
	// 由钩子输出整包替换；未声明则透传（与既有行为一致）。审计 capture 仍捕获标准输出。
	renderHook := route.ResolveRender(contract.HookNameRenderSubmit)
	hold := newHoldWriter()
	capture := NewResponseCaptureWriter(hold)
	rc.Writer = capture

	registerAsyncTask(rc.RequestID, rc.TenantID, rc.UserID, rc.ProjectID, decoded.Model, channelMeta, nativePath)

	relay_handler.HandleTaskSubmit(
		ctx, body2, nativePath, r.Header,
		rc, taskDataProvider, taskBillingProvider, channelMeta,
	)

	if rc.TaskID != "" {
		monitor.SwitchToTaskID(rc.RequestID, rc.TaskID)
	} else {
		monitor.UnregisterRequest(rc.RequestID)
	}
	rc.ForwardingTrace = buildTaskForwardingTrace(nativePath, body2, channelMeta, capture.StatusCode())
	go recordTaskSubmitAudit(r, rc, capture, body2)

	// 同步即完成（parseSubmitResponse 返回 immediate，经解码计划透传）：
	// 提交成功立即终态化——结算/用量/审计/产物与轮询终态同口径（免轮询直达）
	taskStatus := "SUBMITTED"
	contentURL := ""
	if plan.Immediate != nil && rc.TaskID != "" {
		if t := finalizeImmediateTask(r.Context(), plan, plugin, rc.TaskID); t != nil {
			taskStatus = t.Status
			// 产物刚入队转存，通常尚未就绪：按进行中呈现，客户端轮询到转存完成
			if delivery := taskartifact.ResolveDelivery(r.Context(), t); delivery.Pending {
				taskStatus = string(relay_common.TaskStatusInProgress)
			} else {
				contentURL = delivery.ContentURL
			}
		}
	}

	// 提交成功且插件导出渲染钩子 → 钩子输出替换标准响应；否则透传扣住的标准输出
	if rc.TaskID != "" && plugin.HasHook(renderHook) {
		if rendered := callSubmitRenderer(r.Context(), plugin, renderHook, rc.TaskID, decoded.Model, taskStatus, contentURL); rendered != nil {
			// 状态码与内容须一次写入：WriteStatus 不带内容会把状态文本（如 "OK"）写进响应体
			r.Response.WriteStatus(rendered.Status, []byte(rendered.Body))
			hold.discard()
			return
		}
	}
	hold.flushTo(r.Response)
}

// finalizeImmediateTask 同步终态化：组 TaskInfo（状态/快照/用量事实/产物声明）并
// 复用轮询终态管线。失败仅告警（任务保留 SUBMITTED，轮询兜底——immediate 声明但
// 轮询查询会照常进行，语义安全）。
func finalizeImmediateTask(ctx context.Context, plan *jsplugin.SubmitPlan, plugin *jsplugin.ActivePlugin, publicTaskID string) *relay_common.AsyncTask {
	t, err := taskDataProvider.GetTaskByPublicID(ctx, publicTaskID)
	if err != nil || t == nil {
		g.Log().Warningf(ctx, "jsplugin immediate: load task %s: %v", publicTaskID, err)
		return nil
	}
	channel, err := taskDataProvider.GetChannelByID(ctx, t.ChannelID)
	if err != nil || channel == nil {
		g.Log().Warningf(ctx, "jsplugin immediate: load channel %d: %v", t.ChannelID, err)
		return nil
	}
	im := plan.Immediate
	info := &relay_common.TaskInfo{Progress: "100%"}
	switch im.Status {
	case "SUCCESS":
		info.Status = relay_common.TaskStatusSuccess
	case "FAILURE":
		info.Status = relay_common.TaskStatusFailure
		info.FailReason = im.FailReason
	default:
		return nil
	}
	if len(im.TaskData) > 0 {
		info.Data = im.TaskData
	} else {
		info.Data = t.Data
	}
	// 用量事实——与桥的终态映射同口径（时长信号经 OutputSecondsFact 确定性映射）
	if len(im.Usage) > 0 {
		if usageRaw, uErr := contract.ParseJSONObject(im.Usage); uErr == nil {
			if facts, fErr := contract.ValidateUsageFacts(usageRaw, plugin.Manifest); fErr == nil && len(facts) > 0 {
				usage := &relay_common.TaskMaterialUsage{Facts: facts}
				usage.OutputSeconds = plugin.Manifest.OutputSecondsFact(facts)
				info.MaterialUsage = usage
			}
		}
	}
	for _, decl := range im.Artifacts {
		info.Artifacts = append(info.Artifacts, relay_common.TaskArtifact{
			Key: decl.Key, MimeType: decl.MimeType,
			SourceType: decl.Source.Type, SourcePath: decl.Source.Path, SourceAuth: decl.Source.Auth,
		})
	}
	// 与轮询终态同口径回填 result_url（产物声明是 JS 插件成品地址的唯一来源）
	if info.Status == relay_common.TaskStatusSuccess {
		info.ResultURL = relay_common.ResultURLFromArtifacts(info.Data, info.Artifacts)
	}
	return tasklogic.FinalizeTaskImmediate(ctx, t, channel, info)
}

// callSubmitRenderer 调用提交渲染钩子（renderSubmit/别名），失败回落标准输出。
func callSubmitRenderer(ctx context.Context, plugin *jsplugin.ActivePlugin, hook, publicTaskID, model, status, contentURL string) *contract.RenderResult {
	renderArgs, _ := json.Marshal(contract.RenderTaskContext{
		Task: contract.RenderTask{
			PublicTaskID: publicTaskID,
			Model:        model,
			Status:       status,
			ContentURL:   contentURL,
		},
	})
	out, err := plugin.Pool.Call(ctx, hook, renderArgs)
	if err != nil {
		return nil
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		return nil
	}
	rend, err := contract.ValidateRendered(raw)
	if err != nil {
		return nil
	}
	return rend
}

// holdWriter 提交响应扣留器：缓冲标准任务输出，渲染钩子成功时丢弃、否则原样写出。
// 提交响应是单包 JSON（无流式），扣留不影响转发语义。实现 http.ResponseWriter
// 以便嵌入审计 capture 链（Header/WriteHeader 记录后丢弃——真实响应头由最终写出方设置）。
type holdWriter struct {
	buf    bytes.Buffer
	hdr    http.Header
	status int
}

func newHoldWriter() *holdWriter {
	return &holdWriter{hdr: make(http.Header)}
}

func (w *holdWriter) Header() http.Header { return w.hdr }

func (w *holdWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
}

func (w *holdWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }

func (w *holdWriter) flushTo(resp *ghttp.Response) {
	// 回放扣留期间记录的状态码——不回放的话，HandleTaskSubmit 写出的
	// 429/402/502 等错误会以默认 200 送达客户端，SDK 的重试语义全坏。
	// 用 WriteHeader 仅记录状态码：WriteStatus 不带内容会把状态文本（如 "OK"）
	// 写进响应体，污染后续追加的标准任务输出
	if w.status != 0 {
		resp.WriteHeader(w.status)
	}
	if w.buf.Len() == 0 {
		return
	}
	if resp.Header().Get("Content-Type") == "" {
		resp.Header().Set("Content-Type", "application/json")
	}
	resp.Write(w.buf.Bytes())
}

func (w *holdWriter) discard() { w.buf.Reset() }

// parseUploadFiles multipart 提交的文件解析：planFiles（ref→字节载荷，宿主持有）
// 与 decodeFiles（引用描述，进解码契约）。约定：字段名 body 为 JSON 请求体原文，
// 其余文本字段并入 body 顶层（可解析为 JSON 的尝试解析）；文件 ref 命名对齐
// new-api——request_file:{field}（首个）/ request_file:{field}#{i}（后续）。
// 非 multipart 请求返回空。总量受 server.clientMaxBodySize 约束（RelayBodyLimit 预检）。
func parseUploadFiles(r *ghttp.Request) (map[string]jsplugin.FilePayload, []contract.FileRef) {
	if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		return nil, nil
	}
	mf := r.Request.MultipartForm
	if mf == nil {
		if err := r.Request.ParseMultipartForm(32 << 20); err != nil {
			return nil, nil
		}
		mf = r.Request.MultipartForm
	}
	if mf == nil || len(mf.File) == 0 {
		return nil, nil
	}
	planFiles := make(map[string]jsplugin.FilePayload, len(mf.File))
	decodeFiles := make([]contract.FileRef, 0, len(mf.File))
	for field, headers := range mf.File {
		for i, fh := range headers {
			ref := "request_file:" + field
			if i > 0 {
				ref = fmt.Sprintf("request_file:%s#%d", field, i)
			}
			f, err := fh.Open()
			if err != nil {
				continue
			}
			data, readErr := io.ReadAll(f)
			_ = f.Close()
			if readErr != nil || len(data) == 0 {
				continue
			}
			head := data
			if len(head) > 512 {
				head = head[:512]
			}
			mimeType := http.DetectContentType(head)
			planFiles[ref] = jsplugin.FilePayload{Filename: fh.Filename, MimeType: mimeType, Data: data}
			decodeFiles = append(decodeFiles, contract.FileRef{
				Ref: ref, Field: field, Filename: fh.Filename,
				MimeType: mimeType, Size: fh.Size,
			})
		}
	}
	return planFiles, decodeFiles
}

// buildDecodeContext 组装解码上下文（multipart 时 body 为字段合并对象 + files 引用）。
func buildDecodeContext(r *ghttp.Request, plugin *jsplugin.ActivePlugin, route contract.Route, params map[string]string, nativePath string, body []byte, decodeFiles []contract.FileRef) (contract.DecodeContext, bool) {
	ctx := contract.DecodeContext{
		Path:   nativePath,
		Method: strings.ToUpper(r.Method),
		Params: params,
		Body:   json.RawMessage(body),
		Files:  decodeFiles,
	}
	if len(decodeFiles) > 0 {
		// multipart：body 字段（JSON 原文）+ 其余文本字段合并为对象
		merged := map[string]any{}
		rawBody := ""
		if mf := r.Request.MultipartForm; mf != nil && len(mf.Value["body"]) > 0 {
			rawBody = mf.Value["body"][0]
		}
		if rawBody != "" {
			var parsed map[string]any
			if json.Unmarshal([]byte(rawBody), &parsed) == nil {
				for k, v := range parsed {
					merged[k] = v
				}
			} else {
				merged["body"] = rawBody
			}
		}
		if mf := r.Request.MultipartForm; mf != nil {
			for k, vals := range mf.Value {
				if k == "body" || len(vals) == 0 {
					continue
				}
				var parsed any
				if json.Unmarshal([]byte(vals[0]), &parsed) == nil {
					merged[k] = parsed
				} else {
					merged[k] = vals[0]
				}
			}
		}
		if len(merged) > 0 {
			if b, err := json.Marshal(merged); err == nil {
				ctx.Body = b
			}
		}
	}
	return ctx, true
}

// callDecodeHook 调用解码钩子（别名优先）并校验返回；错误时已写出响应。
func callDecodeHook(r *ghttp.Request, plugin *jsplugin.ActivePlugin, route contract.Route, decodeCtx contract.DecodeContext) (*contract.DecodeResult, bool) {
	decodeHook := route.ResolveDecode()
	decodeArgs, _ := json.Marshal(decodeCtx)
	out, err := plugin.Pool.Call(r.Context(), decodeHook, decodeArgs)
	if err != nil {
		writeJSHookError(r, err)
		return nil, false
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		writeJSError(r, http.StatusBadGateway, decodeHook+": "+err.Error())
		return nil, false
	}
	decoded, err := contract.ValidateDecoded(raw, plugin.Manifest, route.Type == contract.RouteTypeDynamic)
	if err != nil {
		writeJSError(r, http.StatusBadRequest, decodeHook+": "+err.Error())
		return nil, false
	}
	return decoded, true
}

// buildRenderTask 任务行 → 渲染视图（单任务/批量共用）。成功任务的登记产物路径
// 替换为统一内容链接（渲染期替换不落库），并附主产物链接 contentUrl；上游已完成但
// 产物仍在转存 OSS 时呈现为进行中（99%）且产物路径置空，转存完成后才给完成态。
func buildRenderTask(ctx context.Context, t *relay_common.AsyncTask) contract.RenderTask {
	data, delivery := taskartifact.RenderData(ctx, t)
	status, progress := t.Status, t.Progress
	if delivery.Pending {
		status, progress = string(relay_common.TaskStatusInProgress), "99%"
	}
	return contract.RenderTask{
		PublicTaskID: t.PublicTaskID,
		TaskID:       extractUpstreamTaskID(t.PrivateData),
		Model:        t.ModelName,
		Status:       status,
		Progress:     progress,
		FailReason:   t.FailReason,
		Data:         data,
		ContentURL:   delivery.ContentURL,
	}
}

// handleJSTaskQuery 查询链：按公开任务 ID 取任务 → renderTask 渲染（钩子名取路由
// 别名，缺省透传任务关键信息）。
func handleJSTaskQuery(r *ghttp.Request, plugin *jsplugin.ActivePlugin, route contract.Route, params map[string]string) {
	// 路由参数名任意（:task_id 惯例，:request_id 等）：缺省取唯一参数值
	taskID := params["task_id"]
	if taskID == "" && len(params) == 1 {
		for _, v := range params {
			taskID = v
		}
	}
	if taskID == "" {
		writeJSError(r, http.StatusBadRequest, "task id is required")
		return
	}
	rc := &relay_handler.TaskRelayContext{
		TenantID:  middleware.GetTenantID(r.Context()),
		UserID:    middleware.GetUserID(r.Context()),
		ApiKeyID:  middleware.GetApiKeyID(r.Context()),
		RequestID: r.GetCtxVar("RequestId").String(),
		Writer:    r.Response.Writer,
	}
	task, err := taskDataProvider.GetTaskByPublicIDAndUser(r.Context(), taskID, rc.UserID, rc.TenantID)
	if err != nil || task == nil {
		writeJSError(r, http.StatusNotFound, "task not found")
		return
	}
	// 任务必须归属当前插件（跨插件任务按不存在处理，不泄露存在性）
	if task.Platform != "js:"+plugin.Key {
		writeJSError(r, http.StatusNotFound, "task not found")
		return
	}

	renderCtx, _ := json.Marshal(contract.RenderTaskContext{Task: buildRenderTask(r.Context(), task)})

	renderHook := route.ResolveRender(contract.HookNameRenderTask)
	if plugin.HasHook(renderHook) {
		out, err := plugin.Pool.Call(r.Context(), renderHook, renderCtx)
		if err != nil {
			writeJSHookError(r, err)
			return
		}
		raw, err := contract.ParseJSONObject(out)
		if err != nil {
			writeJSError(r, http.StatusBadGateway, renderHook+": "+err.Error())
			return
		}
		rend, err := contract.ValidateRendered(raw)
		if err != nil {
			writeJSError(r, http.StatusBadGateway, renderHook+": "+err.Error())
			return
		}
		// 状态码与内容须一次写入：WriteStatus 不带内容会把状态文本（如 "OK"）写进响应体
		r.Response.WriteStatus(rend.Status, []byte(rend.Body))
		return
	}

	// 默认渲染：透传任务关键信息
	r.Response.WriteJson(g.Map{
		"task_id":     task.PublicTaskID,
		"status":      task.Status,
		"progress":    task.Progress,
		"fail_reason": task.FailReason,
		"data":        json.RawMessage(orEmptyJSON(task.Data)),
	})
}

// ---------------------------------------------------------------------------
// 辅助
// ---------------------------------------------------------------------------

func writeJSNotFound(r *ghttp.Request) {
	r.Response.WriteStatus(404, g.Map{
		"error":      g.Map{"type": "invalid_request_error", "message": "js plugin route not found"},
		"request_id": r.GetCtxVar("RequestId").String(),
	})
}

func writeJSError(r *ghttp.Request, status int, message string) {
	r.Response.WriteStatus(status, g.Map{
		"error":      g.Map{"type": "js_plugin_error", "message": message},
		"request_id": r.GetCtxVar("RequestId").String(),
	})
}

// writeJSHookError 钩子执行错误（502）：消息经 SanitizeHookMessage 净化——插件异常
// 是面向用户的业务文案通道，但可能意外内嵌密钥/签名片段，脱敏+截断后透出，不泛化。
func writeJSHookError(r *ghttp.Request, err error) {
	writeJSError(r, http.StatusBadGateway, jsplugin.SanitizeHookMessage(err.Error()))
}

// injectModelField 把模型名写入 body 副本（不改原请求）。
func injectModelField(body []byte, model string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	modelJSON, _ := json.Marshal(model)
	m["model"] = modelJSON
	return json.Marshal(m)
}

// extractUpstreamTaskID 从 PrivateData 提取上游任务 ID（渲染上下文用）。
func extractUpstreamTaskID(privateData json.RawMessage) string {
	var pd struct {
		UpstreamTaskID string `json:"upstream_task_id"`
	}
	_ = json.Unmarshal(privateData, &pd)
	return pd.UpstreamTaskID
}

func orEmptyJSON(data json.RawMessage) string {
	if len(data) == 0 {
		return "{}"
	}
	return string(data)
}

// context compile guard：确保本文件与 task.go 同包共享的符号存在。
var _ = context.Background
var _ error = relay_common.ErrChannelUnavailable
