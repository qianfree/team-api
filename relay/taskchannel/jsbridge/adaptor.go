// Package jsbridge 把 JS 任务插件适配为 common.TaskAdaptor，注册进 taskchannel：
// 提交链消费分发器经 ctx 传入的 SubmitPlan（decode 产物），轮询链按 taskData.platform
// 路由到具体插件池。见 docs/design/JS异步任务插件设计方案-v1.md。
package jsbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/qianfree/team-api/internal/jsplugin"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/taskchannel"
)

func init() {
	taskchannel.Register(constant.ProviderJSTask, func() common.TaskAdaptor {
		return &Adaptor{}
	})
}

// jsPlatformPrefix 任务平台标识前缀（task_handler 提交时写入 task.Platform）。
const jsPlatformPrefix = "js:"

// Adaptor JS 任务插件桥。提交链实例由 HandleTaskSubmit 经工厂创建、
// 生命周期覆盖一次提交；轮询链实例按渠道批次创建，跨任务复用
// （lastQuery 承接 FetchTask → ParseTaskResult 的任务引用传递）。
type Adaptor struct {
	info      *common.RelayInfo
	pluginKey string

	// 提交链状态
	plan       *jsplugin.SubmitPlan
	submitDesc *contract.BuildRequestResult

	// 轮询链状态（FetchTask 后紧跟 ParseTaskResult，顺序由 pollSingleTask 保证）
	lastQueryTasks []contract.QueryTaskRef
}

// ---------------------------------------------------------------------------
// 提交链
// ---------------------------------------------------------------------------

func (a *Adaptor) planPath() string {
	if a.plan == nil {
		return ""
	}
	return a.plan.NativePath
}

func (a *Adaptor) planParams() map[string]string {
	if a.plan == nil {
		return nil
	}
	return a.plan.Params
}

func (a *Adaptor) planModel() string {
	if a.plan == nil || a.plan.Decoded == nil {
		return ""
	}
	return a.plan.Decoded.Model
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
	a.pluginKey = info.ChannelMeta.Settings.PluginKey
	// 渠道未显式配置 plugin_key 时按模型解析（v1 模型对插件独占，见设计方案 §五）
	if a.pluginKey == "" && info.OriginModelName != "" {
		if resolved, ok := jsplugin.ResolvePluginKeyForModel(info.OriginModelName); ok {
			a.pluginKey = resolved
		}
	}
}

// ValidateRequest 从 ctx 取分发器的解码计划；缺失说明请求不经 /pjs 分发器进入
// （如直连内部重放），v1 拒绝。
func (a *Adaptor) ValidateRequest(ctx context.Context, info *common.RelayInfo, _ []byte) *common.TaskError {
	plan := jsplugin.SubmitPlanFromContext(ctx)
	if plan == nil {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "js task submit requires /pjs dispatcher"}
	}
	if plan.Plugin.Key != a.pluginKey {
		return &common.TaskError{StatusCode: http.StatusBadRequest,
			Message: fmt.Sprintf("channel bound to plugin %q but request decoded for %q", a.pluginKey, plan.Plugin.Key)}
	}
	a.plan = plan
	return nil
}

// EstimateBilling 计费上下文（ratios）：
//   - 插件声明 usageSchema 且导出 estimateUsage 时：数值事实进 "formula_facts"
//     （formula 模式经 u() 消费；随 billing_context 冻结供结算期合并）；
//     unit=second → spec.duration（时长信号）、unit=count → count（张数信号）
//   - 其余情况：乘数 1.0（模型级默认价）
func (a *Adaptor) EstimateBilling(ctx context.Context, _ *common.RelayInfo, _ []byte) map[string]any {
	ratios := map[string]any{"base": 1.0}
	if a.plan == nil || !a.plan.Plugin.Pool.HasHook("estimateUsage") {
		return ratios
	}
	args, err := json.Marshal(contract.EstimateUsageContext{
		Body:   a.plan.Decoded.RequestBody,
		Model:  a.plan.Decoded.Model,
		Params: a.plan.Params,
	})
	if err != nil {
		return ratios
	}
	out, err := a.plan.Plugin.Pool.Call(ctx, "estimateUsage", args)
	if err != nil {
		return ratios // 估算失败不阻断提交（fail-safe 默认价，结算期多退少补）
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		return ratios
	}
	applyUsageFacts(ratios, raw, a.plan.Plugin.Manifest)
	return ratios
}

// applyUsageFacts 把数值事实映射进 ratios：formula_facts（u() 通道）+ unit 信号。
func applyUsageFacts(ratios map[string]any, raw map[string]json.RawMessage, manifest *contract.Manifest) {
	facts, err := contract.ValidateUsageFacts(raw, manifest)
	if err != nil || len(facts) == 0 {
		return
	}
	ratios["formula_facts"] = facts
	for _, field := range manifest.UsageSchema {
		v, ok := facts[field.Key]
		if !ok {
			continue
		}
		switch field.Unit {
		case "second":
			ratios["spec.duration"] = v
		case "count":
			ratios["count"] = v
		}
	}
}

func (a *Adaptor) AdjustBillingOnSubmit(_ *common.RelayInfo, _ []byte) map[string]any {
	return nil
}

// ensureSubmitDesc 调用插件的 buildSubmitRequest 并缓存（BuildRequestURL/Header/Body 三方法共享）。
func (a *Adaptor) ensureSubmitDesc(ctx context.Context) (*contract.BuildRequestResult, *common.TaskError) {
	if a.submitDesc != nil {
		return a.submitDesc, nil
	}
	if a.plan == nil {
		return nil, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "js adaptor: missing submit plan"}
	}
	args, _ := json.Marshal(contract.BuildSubmitContext{
		Body:           a.plan.Decoded.RequestBody,
		Params:         a.plan.Params,
		Model:          a.plan.Decoded.Model,
		UpstreamModel:  a.info.ChannelMeta.UpstreamModelName,
		ChannelBaseURL: a.info.ChannelMeta.BaseURL,
		ApiKey:         a.info.ChannelMeta.ApiKey,
	})
	out, err := a.plan.Plugin.Pool.Call(ctx, "buildSubmitRequest", args)
	if err != nil {
		return nil, jsHookTaskError(err)
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		return nil, &common.TaskError{StatusCode: http.StatusBadGateway, Message: "buildSubmitRequest: " + err.Error()}
	}
	desc, err := contract.ValidateBuiltRequest(raw)
	if err != nil {
		return nil, &common.TaskError{StatusCode: http.StatusBadGateway, Message: "buildSubmitRequest: " + err.Error()}
	}
	a.submitDesc = desc
	return desc, nil
}

func (a *Adaptor) BuildRequestURL(info *common.RelayInfo) (string, error) {
	desc, taskErr := a.ensureSubmitDesc(info.Context)
	if taskErr != nil {
		return "", fmt.Errorf("%s", taskErr.Message)
	}
	return resolveUpstreamURL(info.ChannelMeta.BaseURL, desc.URL)
}

func (a *Adaptor) BuildRequestHeader(header http.Header, info *common.RelayInfo) error {
	desc, taskErr := a.ensureSubmitDesc(info.Context)
	if taskErr != nil {
		return fmt.Errorf("%s", taskErr.Message)
	}
	header.Set("Content-Type", "application/json")
	for k, v := range desc.Headers {
		header.Set(k, v) // 插件声明（含签名计算的 Authorization）覆盖默认
	}
	if header.Get("Authorization") == "" {
		header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	}
	return nil
}

func (a *Adaptor) BuildRequestBody(ctx context.Context, info *common.RelayInfo, _ []byte) (io.Reader, error) {
	desc, taskErr := a.ensureSubmitDesc(ctx)
	if taskErr != nil {
		return nil, fmt.Errorf("%s", taskErr.Message)
	}
	if len(desc.Body) == 0 {
		return bytes.NewReader([]byte("{}")), nil
	}
	return bytes.NewReader(desc.Body), nil
}

func (a *Adaptor) DoRequest(ctx context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	desc, taskErr := a.ensureSubmitDesc(ctx)
	if taskErr != nil {
		return nil, fmt.Errorf("%s", taskErr.Message)
	}
	// FileReference 内联：占位符替换为编码字节（字节仅在宿主侧流转，JS 只见引用）
	if a.plan != nil && len(a.plan.Files) > 0 && hasFilePlaceholders(desc.Body) {
		inlined, err := inlineFileRefs(desc.Body, a.plan.Files)
		if err != nil {
			return nil, fmt.Errorf("file inline: %w", err)
		}
		desc.Body = inlined
		a.submitDesc = desc // 缓存更新：后续重试同体
		requestBody = bytes.NewReader(inlined)
	}
	target, err := resolveUpstreamURL(info.ChannelMeta.BaseURL, desc.URL)
	if err != nil {
		return nil, err
	}
	method := desc.Method
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, target, requestBody)
	if err != nil {
		return nil, err
	}
	if err := a.BuildRequestHeader(req.Header, info); err != nil {
		return nil, err
	}
	client := common.NewPooledClient(120, info.ChannelMeta.Settings.UseProxy)
	return client.Do(req)
}

func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, _ *common.RelayInfo) (string, []byte, *common.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "read response failed"}
	}
	if resp.StatusCode != http.StatusOK {
		return "", body, &common.TaskError{StatusCode: resp.StatusCode, Message: string(body)}
	}
	// responseBody：JSON 原文透传（插件自行解析；非 JSON 内容按字符串包装）
	responseBody := wrapResponseBody(body, resp.Header.Get("Content-Type"))

	args, _ := json.Marshal(contract.ParseSubmitContext{
		Status:       resp.StatusCode,
		ResponseBody: responseBody,
		Path:         a.planPath(),
		Params:       a.planParams(),
		Model:        a.planModel(),
	})
	out, err := a.plan.Plugin.Pool.Call(ctx, "parseSubmitResponse", args)
	if err != nil {
		return "", body, jsHookTaskError(err)
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		return "", body, &common.TaskError{StatusCode: http.StatusBadGateway, Message: "parseSubmitResponse: " + err.Error()}
	}
	result, err := contract.ValidateSubmitted(raw)
	if err != nil {
		return "", body, &common.TaskError{StatusCode: http.StatusBadGateway, Message: "parseSubmitResponse: " + err.Error()}
	}
	taskData := result.TaskData
	if len(taskData) == 0 {
		taskData = json.RawMessage(body)
	}
	// 同步即完成：回填解码计划，分发器在提交成功后立即终态化（免轮询直达结算）
	if result.Im != nil && a.plan != nil {
		a.plan.Immediate = result.Im
	}
	return result.TaskID, taskData, nil
}

// ---------------------------------------------------------------------------
// 轮询链
// ---------------------------------------------------------------------------

// FetchTask 按渠道批次调用：从 taskData.platform 解析插件，调用 buildQueryRequest
// （per_task：单任务引用；batch 模式 Phase 2 接入），发上游返回响应。
func (a *Adaptor) FetchTask(ctx context.Context, baseURL, apiKey string, taskData []byte) (*http.Response, error) {
	var td struct {
		TaskID   string `json:"task_id"`
		Platform string `json:"platform"`
		UseProxy bool   `json:"use_proxy"`
		Model    string `json:"model"`
	}
	if err := json.Unmarshal(taskData, &td); err != nil {
		return nil, fmt.Errorf("jsbridge: parse task data: %w", err)
	}
	pluginKey := strings.TrimPrefix(td.Platform, jsPlatformPrefix)
	if pluginKey == "" || pluginKey == td.Platform {
		return nil, fmt.Errorf("jsbridge: task platform %q is not a js plugin task", td.Platform)
	}
	plugin, ok := jsplugin.CurrentGeneration().Plugin(pluginKey)
	if !ok {
		return nil, fmt.Errorf("jsbridge: plugin %q not active", pluginKey)
	}
	a.pluginKey = pluginKey

	taskRef := contract.QueryTaskRef{TaskID: td.TaskID}
	a.lastQueryTasks = []contract.QueryTaskRef{taskRef}

	// 调用插件的 buildQueryRequest
	args, err := json.Marshal(contract.BuildQueryContext{Tasks: a.lastQueryTasks, Model: td.Model})
	if err != nil {
		return nil, err
	}
	// fetch 白名单 = 渠道 host ∪ manifest.allowedHosts
	qctx := jsplugin.WithAllowedHosts(ctx, append([]string{hostOf(baseURL)}, plugin.Manifest.AllowedHosts...))
	out, err := plugin.Pool.Call(qctx, "buildQueryRequest", args)
	if err != nil {
		return nil, fmt.Errorf("buildQueryRequest: %w", err)
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		return nil, fmt.Errorf("buildQueryRequest: %w", err)
	}
	desc, err := contract.ValidateBuiltRequest(raw)
	if err != nil {
		return nil, fmt.Errorf("buildQueryRequest: %w", err)
	}

	target, err := resolveUpstreamURL(baseURL, desc.URL)
	if err != nil {
		return nil, err
	}
	method := desc.Method
	if method == "" {
		method = http.MethodGet
	}
	var bodyReader io.Reader
	if len(desc.Body) > 0 {
		bodyReader = bytes.NewReader(desc.Body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range desc.Headers {
		req.Header.Set(k, v)
	}
	client := common.NewPooledClient(30, td.UseProxy)
	return client.Do(req)
}

// ParseTaskResult 调用插件的 parseQueryResponse，映射为 TaskInfo。
// 依赖 FetchTask 先行设置的 lastQueryTasks（pollSingleTask 顺序调用保证）。
func (a *Adaptor) ParseTaskResult(body []byte) (*common.TaskInfo, error) {
	plugin, ok := jsplugin.CurrentGeneration().Plugin(a.pluginKey)
	if !ok || len(a.lastQueryTasks) == 0 {
		return nil, fmt.Errorf("jsbridge: no pending query context")
	}
	args, err := json.Marshal(contract.ParseQueryContext{
		Status:       http.StatusOK,
		ResponseBody: wrapResponseBody(body, "application/json"),
		Tasks:        a.lastQueryTasks,
	})
	if err != nil {
		return nil, err
	}
	out, err := plugin.Pool.Call(context.Background(), "parseQueryResponse", args)
	if err != nil {
		return nil, fmt.Errorf("parseQueryResponse: %w", err)
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		return nil, fmt.Errorf("parseQueryResponse: %w", err)
	}
	result, err := contract.ValidateQueried(raw, a.lastQueryTasks)
	if err != nil {
		return nil, fmt.Errorf("parseQueryResponse: %w", err)
	}
	return taskInfoFromQueryItem(plugin, result.Items[0], body), nil
}

// taskInfoFromQueryItem 单项 parseQueryResponse 结果 → TaskInfo（逐任务/批量共用）。
func taskInfoFromQueryItem(plugin *jsplugin.ActivePlugin, item contract.QueryItem, fallbackBody []byte) *common.TaskInfo {
	info := &common.TaskInfo{}
	switch {
	case item.Error != nil && item.Done:
		info.Status = common.TaskStatusFailure
		info.FailReason = item.Error.Message
	case item.Error != nil:
		info.Status = common.TaskStatusInProgress
		info.FailReason = item.Error.Message
	case item.Done:
		info.Status = common.TaskStatusSuccess
		info.Progress = "100%"
	default:
		info.Status = common.TaskStatusInProgress
	}
	if len(item.TaskData) > 0 {
		info.Data = item.TaskData
	} else {
		info.Data = fallbackBody
	}

	// 终态成功：结算用量事实（items[].usage 内联，键 ∈ usageSchema）+ 产物声明
	if item.Done && item.Error == nil {
		attachSettleFacts(info, plugin, &item)
		for _, decl := range item.Artifacts {
			info.Artifacts = append(info.Artifacts, common.TaskArtifact{
				Key:        decl.Key,
				MimeType:   decl.MimeType,
				SourceType: decl.Source.Type,
				SourcePath: decl.Source.Path,
			})
		}
	}
	return info
}

// attachSettleFacts 解析 parseQueryResponse 内联上报的终态用量事实：
// unit=second 映射 OutputSeconds（触发结算期公式重跑），其余数值事实进 Facts（u() 通道）。
// 事实非法/缺失不阻断终态判定（回退预扣口径），仅记录错误。
func attachSettleFacts(info *common.TaskInfo, plugin *jsplugin.ActivePlugin, item *contract.QueryItem) {
	if len(item.Usage) == 0 {
		return
	}
	usageRaw, err := contract.ParseJSONObject(item.Usage)
	if err != nil {
		return
	}
	facts, err := contract.ValidateUsageFacts(usageRaw, plugin.Manifest)
	if err != nil || len(facts) == 0 {
		return
	}
	usage := &common.TaskMaterialUsage{Facts: facts}
	for _, field := range plugin.Manifest.UsageSchema {
		if field.Unit == "second" {
			if v, ok := facts[field.Key]; ok && v > 0 {
				usage.OutputSeconds = v
			}
		}
	}
	info.MaterialUsage = usage
}

// ---------------------------------------------------------------------------
// 批量轮询（fetchMode=batch）
// ---------------------------------------------------------------------------

// FetchMode 返回插件声明的轮询模式（插件未激活时回落 per_task）。
func (a *Adaptor) FetchMode() string {
	if plugin, ok := jsplugin.CurrentGeneration().Plugin(a.pluginKey); ok {
		return plugin.Manifest.EffectiveFetchMode()
	}
	return "per_task"
}

// FetchBatchTasks 一次查询整批任务：调用插件的 buildQueryRequest（tasks 携带全部
// 未终态任务引用），单次上游往返。refs 由轮询循环保证同渠道同插件（platform 一致）。
func (a *Adaptor) FetchBatchTasks(ctx context.Context, baseURL, apiKey string, refs []common.BatchTaskRef) (*http.Response, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("jsbridge: empty batch")
	}
	pluginKey := strings.TrimPrefix(refs[0].Platform, jsPlatformPrefix)
	if pluginKey == "" || pluginKey == refs[0].Platform {
		return nil, fmt.Errorf("jsbridge: batch platform %q is not a js plugin task", refs[0].Platform)
	}
	plugin, ok := jsplugin.CurrentGeneration().Plugin(pluginKey)
	if !ok {
		return nil, fmt.Errorf("jsbridge: plugin %q not active", pluginKey)
	}
	a.pluginKey = pluginKey

	// 任务引用（TaskData 快照随行——批量插件的 parse 钩子可按任务分派）
	a.lastQueryTasks = make([]contract.QueryTaskRef, 0, len(refs))
	for _, ref := range refs {
		a.lastQueryTasks = append(a.lastQueryTasks, contract.QueryTaskRef{
			TaskID:   ref.TaskID,
			TaskData: normalizeRawJSON(ref.TaskData),
		})
	}

	args, err := json.Marshal(contract.BuildQueryContext{Tasks: a.lastQueryTasks})
	if err != nil {
		return nil, err
	}
	qctx := jsplugin.WithAllowedHosts(ctx, append([]string{hostOf(baseURL)}, plugin.Manifest.AllowedHosts...))
	out, err := plugin.Pool.Call(qctx, "buildQueryRequest", args)
	if err != nil {
		return nil, fmt.Errorf("buildQueryRequest: %w", err)
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		return nil, fmt.Errorf("buildQueryRequest: %w", err)
	}
	desc, err := contract.ValidateBuiltRequest(raw)
	if err != nil {
		return nil, fmt.Errorf("buildQueryRequest: %w", err)
	}

	target, err := resolveUpstreamURL(baseURL, desc.URL)
	if err != nil {
		return nil, err
	}
	method := desc.Method
	if method == "" {
		method = http.MethodPost // 批量查询天然带 body（任务清单）
	}
	var bodyReader io.Reader
	if len(desc.Body) > 0 {
		bodyReader = bytes.NewReader(desc.Body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range desc.Headers {
		req.Header.Set(k, v)
	}
	client := common.NewPooledClient(30, false)
	return client.Do(req)
}

// ParseBatchTasks 解析批量响应：parseQueryResponse 的 items 按上游任务 ID 映射；
// 本轮未回报的任务不在 map 中（轮询循环保持其现状）。
func (a *Adaptor) ParseBatchTasks(body []byte) (map[string]*common.TaskInfo, error) {
	plugin, ok := jsplugin.CurrentGeneration().Plugin(a.pluginKey)
	if !ok || len(a.lastQueryTasks) == 0 {
		return nil, fmt.Errorf("jsbridge: no pending batch query context")
	}
	args, err := json.Marshal(contract.ParseQueryContext{
		Status:       http.StatusOK,
		ResponseBody: wrapResponseBody(body, "application/json"),
		Tasks:        a.lastQueryTasks,
	})
	if err != nil {
		return nil, err
	}
	out, err := plugin.Pool.Call(context.Background(), "parseQueryResponse", args)
	if err != nil {
		return nil, fmt.Errorf("parseQueryResponse: %w", err)
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		return nil, fmt.Errorf("parseQueryResponse: %w", err)
	}
	result, err := contract.ValidateQueried(raw, a.lastQueryTasks)
	if err != nil {
		return nil, fmt.Errorf("parseQueryResponse: %w", err)
	}
	infos := make(map[string]*common.TaskInfo, len(result.Items))
	for i := range result.Items {
		item := result.Items[i]
		infos[item.TaskID] = taskInfoFromQueryItem(plugin, item, body)
	}
	return infos, nil
}

// normalizeRawJSON 空值/非法 JSON 归一为 nil（契约透传容器要求合法 JSON 或缺省）。
func normalizeRawJSON(data []byte) json.RawMessage {
	if len(data) == 0 {
		return nil
	}
	if json.Valid(data) {
		return data
	}
	return nil
}

// ---------------------------------------------------------------------------
// 元信息
// ---------------------------------------------------------------------------

func (a *Adaptor) GetModelList() []string {
	if plugin, ok := jsplugin.CurrentGeneration().Plugin(a.pluginKey); ok {
		return plugin.Manifest.Models
	}
	return nil
}

func (a *Adaptor) GetChannelName() string {
	return "JS:" + a.pluginKey
}

// ---------------------------------------------------------------------------
// 辅助
// ---------------------------------------------------------------------------

// resolveUpstreamURL 拼接上游地址：相对路径 → baseURL + path；绝对 URL 原样。
func resolveUpstreamURL(baseURL, target string) (string, error) {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target, nil
	}
	if !strings.HasPrefix(target, "/") {
		target = "/" + target
	}
	return strings.TrimRight(baseURL, "/") + target, nil
}

// hostOf 从 baseURL 提取 host[:port]（fetch 白名单用）。
func hostOf(baseURL string) string {
	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		return strings.ToLower(u.Host)
	}
	return strings.ToLower(baseURL)
}

// wrapResponseBody 非速记：JSON 原文直传；非 JSON 内容包成 JSON 字符串。
func wrapResponseBody(body []byte, contentType string) json.RawMessage {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return trimmed
	}
	if strings.Contains(contentType, "json") && len(trimmed) > 0 {
		return trimmed
	}
	quoted, _ := json.Marshal(string(body))
	return quoted
}

// jsHookTaskError 引擎钩子错误 → TaskError（502 透出）。
func jsHookTaskError(err error) *common.TaskError {
	return &common.TaskError{StatusCode: http.StatusBadGateway, Message: err.Error()}
}
