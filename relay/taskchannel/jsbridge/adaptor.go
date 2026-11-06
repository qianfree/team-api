// Package jsbridge 把 JS 任务插件适配为 common.TaskAdaptor，注册进 taskchannel：
// 提交链消费分发器经 ctx 传入的 SubmitPlan（decode 产物），轮询链按 taskData.platform
// 路由到具体插件池。见 docs/JS插件体系架构与调用链分析.md。
package jsbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/frame/g"

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

// ErrPluginNotActive 哨兵错误：任务归属的 JS 插件不在当前 generation（已被禁用/删除）。
// FetchTask/FetchBatchTasks 按此包装返回，轮询链（internal/logic/task）凭 errors.Is 识别
// 「插件已下线」的存量任务，立即按失败终态化并退款，而不是空转到超时网。
// 注意与 platform 无 js: 前缀的错误区分——那属数据损坏而非插件停用，不包装本哨兵。
var ErrPluginNotActive = errors.New("jsbridge: plugin not active")

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
	// lastAllowedHosts / lastBaseURL FetchTask/FetchBatchTasks 算出的 fetch 白名单
	// （渠道 host ∪ manifest.allowedHosts）与渠道地址，紧随其后的 Parse 钩子复用——
	// parse 内 utils.fetch 与 build 钩子保持同一套白名单和相对路径解析基准
	lastAllowedHosts []string
	lastBaseURL      string
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

// planDecodedBody 归一后的提交请求体（parseSubmitResponse 的 action 推断线索）。
func (a *Adaptor) planDecodedBody() json.RawMessage {
	if a.plan == nil || a.plan.Decoded == nil {
		return nil
	}
	return a.plan.Decoded.RequestBody
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
	a.pluginKey = info.ChannelMeta.Settings.PluginKey
	// 渠道未显式配置 plugin_key 时按模型解析（v1 模型对插件独占）
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
	if a.plan == nil || !a.plan.Plugin.HasHook("estimateUsage") {
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
	out, err := a.plan.Plugin.Pool.Call(channelHookCtx(ctx, a.info.ChannelMeta.BaseURL, a.submitAllowedHosts()), "estimateUsage", args)
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
	// fetch 白名单随调用注入（渠道 host ∪ manifest.allowedHosts）：submit 钩子内
	// utils.fetch 访问渠道域与轮询链同口径放行；渠道 baseUrl 同步注入（相对路径解析基准）
	qctx := channelHookCtx(ctx, a.info.ChannelMeta.BaseURL, a.submitAllowedHosts())
	out, err := a.plan.Plugin.Pool.Call(qctx, "buildSubmitRequest", args)
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
	// 绝对 URL host 白名单校验（覆盖 BuildRequestURL/DoRequest 全部消费方）
	if err := validateUpstreamTarget(a.info.ChannelMeta.BaseURL, a.plan.Plugin.Manifest, desc.URL); err != nil {
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
	// 凭证指纹（Debug 级）：上游 401/403 排障时比对「实际发出的 key」与预期——
	// 命名空间冲突/渠道编辑未生效/多渠道同模型选错渠道，看首尾即可定位，不泄露全量
	g.Log().Debugf(ctx, "jsbridge: upstream %s %s channel=%d auth=%s",
		method, target, info.ChannelMeta.ChannelID, authFingerprint(req.Header.Get("Authorization")))
	allowed := append([]string{hostOf(info.ChannelMeta.BaseURL)}, a.plan.Plugin.Manifest.AllowedHosts...)
	client := newWhitelistClient(120, info.ChannelMeta.Settings.UseProxy, allowed)
	return client.Do(req)
}

// authFingerprint 凭证指纹：保留 scheme + key 首 6 尾 4 与长度（key 不足 12 位全掩码）。
func authFingerprint(authHeader string) string {
	if authHeader == "" {
		return "<none>"
	}
	scheme, key := authHeader, ""
	if idx := strings.IndexByte(authHeader, ' '); idx > 0 {
		scheme, key = authHeader[:idx], authHeader[idx+1:]
	}
	if len(key) < 12 {
		return scheme + " ****(len=" + strconv.Itoa(len(key)) + ")"
	}
	return fmt.Sprintf("%s %s...%s(len=%d)", scheme, key[:6], key[len(key)-4:], len(key))
}

func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, _ *common.RelayInfo) (string, []byte, *common.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "read response failed"}
	}
	if resp.StatusCode != http.StatusOK {
		return "", body, &common.TaskError{StatusCode: resp.StatusCode, Message: upstreamErrorMessage(body)}
	}
	// responseBody：JSON 原文透传（插件自行解析；非 JSON 内容按字符串包装）
	responseBody := wrapResponseBody(body, resp.Header.Get("Content-Type"))

	args, _ := json.Marshal(contract.ParseSubmitContext{
		Status:       resp.StatusCode,
		ResponseBody: responseBody,
		Path:         a.planPath(),
		Params:       a.planParams(),
		Model:        a.planModel(),
		Body:         a.planDecodedBody(),
	})
	// parse 钩子与 build 钩子同口径注入 fetch 白名单与渠道 baseUrl（parse 内可能 utils.fetch 补取详情）
	out, err := a.plan.Plugin.Pool.Call(channelHookCtx(ctx, a.info.ChannelMeta.BaseURL, a.submitAllowedHosts()), "parseSubmitResponse", args)
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

// submitAllowedHosts 提交链 fetch 白名单：渠道 host ∪ manifest.allowedHosts，
// 与轮询链 FetchTask 同口径。decode 钩子在渠道选择之前执行、无渠道上下文，不在此列
// （decode 内 utils.fetch 仅允许 manifest.allowedHosts，且不可用相对路径）。
func (a *Adaptor) submitAllowedHosts() []string {
	if a.info == nil || a.plan == nil {
		return nil
	}
	return append([]string{hostOf(a.info.ChannelMeta.BaseURL)}, a.plan.Plugin.Manifest.AllowedHosts...)
}

// channelHookCtx 组装有渠道上下文的钩子调用 ctx：fetch 白名单 + 相对路径解析
// 基准（utils.fetch 相对 URL 按渠道 baseUrl 拼接，与主链路 resolveUpstreamURL 同语义）。
func channelHookCtx(ctx context.Context, baseURL string, allowed []string) context.Context {
	return jsplugin.WithBaseURL(jsplugin.WithAllowedHosts(ctx, allowed), baseURL)
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
		// 插件不在当前 generation：包装哨兵，轮询链凭 errors.Is 立即终态化存量任务
		return nil, fmt.Errorf("%w: %q", ErrPluginNotActive, pluginKey)
	}
	a.pluginKey = pluginKey
	// 白名单与渠道地址留存给紧随其后的 ParseTaskResult（parse 钩子内 utils.fetch 复用）
	a.lastAllowedHosts = append([]string{hostOf(baseURL)}, plugin.Manifest.AllowedHosts...)
	a.lastBaseURL = baseURL

	taskRef := contract.QueryTaskRef{TaskID: td.TaskID}
	a.lastQueryTasks = []contract.QueryTaskRef{taskRef}

	// 调用插件的 buildQueryRequest
	args, err := json.Marshal(contract.BuildQueryContext{Tasks: a.lastQueryTasks, Model: td.Model})
	if err != nil {
		return nil, err
	}
	// fetch 白名单 = 渠道 host ∪ manifest.allowedHosts（同时作为绝对 URL/重定向校验集）
	allowed := append([]string{hostOf(baseURL)}, plugin.Manifest.AllowedHosts...)
	qctx := channelHookCtx(ctx, baseURL, allowed)
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
	if err := validateUpstreamTarget(baseURL, plugin.Manifest, desc.URL); err != nil {
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
	client := newWhitelistClient(30, td.UseProxy, allowed)
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
		Status:         http.StatusOK,
		ResponseBody:   wrapResponseBody(body, "application/json"),
		Tasks:          a.lastQueryTasks,
		ChannelBaseURL: a.lastBaseURL,
	})
	if err != nil {
		return nil, err
	}
	// parse 钩子沿用 FetchTask 的白名单与渠道地址——轮询循环无请求 ctx 可传，
	// 不注入的话 parse 内 utils.fetch 只剩实例级 manifest.allowedHosts，渠道 host 会被拒
	pctx := channelHookCtx(context.Background(), a.lastBaseURL, a.lastAllowedHosts)
	out, err := plugin.Pool.Call(pctx, "parseQueryResponse", args)
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
			// SourceAuth 必须随声明透传：声明 channel-bearer 的鉴权产物若丢失该标记，
			// 产物流水线不会注入渠道鉴权，下载 401 且重试全败（immediate 路径已带）
			info.Artifacts = append(info.Artifacts, common.TaskArtifact{
				Key:        decl.Key,
				MimeType:   decl.MimeType,
				SourceType: decl.Source.Type,
				SourcePath: decl.Source.Path,
				SourceAuth: decl.Source.Auth,
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
	// 时长信号确定性映射（多个 second 字段时优先 "seconds"，否则声明序首个有值）
	usage.OutputSeconds = plugin.Manifest.OutputSecondsFact(facts)
	info.MaterialUsage = usage
}

// ---------------------------------------------------------------------------
// 批量轮询（fetchMode=batch）
// ---------------------------------------------------------------------------

// FetchModeFor 按任务平台返回插件声明的轮询模式（非 JS 平台/插件未激活回落 per_task）。
// 以 platform 为参：轮询循环拿到的适配器是工厂新实例（pluginKey 尚未初始化），
// 按实例状态判定的旧实现恒得 per_task，批量通道从未激活过。
func (a *Adaptor) FetchModeFor(platform string) string {
	pluginKey := strings.TrimPrefix(platform, jsPlatformPrefix)
	if plugin, ok := jsplugin.CurrentGeneration().Plugin(pluginKey); ok {
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
		// 插件不在当前 generation：包装哨兵，轮询链凭 errors.Is 立即终态化存量任务
		return nil, fmt.Errorf("%w: %q", ErrPluginNotActive, pluginKey)
	}
	a.pluginKey = pluginKey
	// 代理设置随任务快照携带（与逐任务路径同口径），白名单与渠道地址留存给 ParseBatchTasks
	useProxy := batchUseProxy(refs)
	a.lastAllowedHosts = append([]string{hostOf(baseURL)}, plugin.Manifest.AllowedHosts...)
	a.lastBaseURL = baseURL

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
	allowed := append([]string{hostOf(baseURL)}, plugin.Manifest.AllowedHosts...)
	qctx := channelHookCtx(ctx, baseURL, allowed)
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
	if err := validateUpstreamTarget(baseURL, plugin.Manifest, desc.URL); err != nil {
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
	client := newWhitelistClient(30, useProxy, allowed)
	return client.Do(req)
}

// batchUseProxy 从批量任务快照解析代理设置（pollSingleTask 组装 taskData 时写入；
// 与逐任务路径同源，配置代理的渠道批量轮询不再被硬编码直连）。
func batchUseProxy(refs []common.BatchTaskRef) bool {
	for _, ref := range refs {
		var td struct {
			UseProxy bool `json:"use_proxy"`
		}
		if json.Unmarshal(ref.TaskData, &td) == nil && td.UseProxy {
			return true
		}
	}
	return false
}

// ParseBatchTasks 解析批量响应：parseQueryResponse 的 items 按上游任务 ID 映射；
// 本轮未回报的任务不在 map 中（轮询循环保持其现状）。
func (a *Adaptor) ParseBatchTasks(body []byte) (map[string]*common.TaskInfo, error) {
	plugin, ok := jsplugin.CurrentGeneration().Plugin(a.pluginKey)
	if !ok || len(a.lastQueryTasks) == 0 {
		return nil, fmt.Errorf("jsbridge: no pending batch query context")
	}
	args, err := json.Marshal(contract.ParseQueryContext{
		Status:         http.StatusOK,
		ResponseBody:   wrapResponseBody(body, "application/json"),
		Tasks:          a.lastQueryTasks,
		ChannelBaseURL: a.lastBaseURL,
	})
	if err != nil {
		return nil, err
	}
	// parse 钩子沿用 FetchBatchTasks 的白名单与渠道地址，与逐任务路径同语义
	pctx := channelHookCtx(context.Background(), a.lastBaseURL, a.lastAllowedHosts)
	out, err := plugin.Pool.Call(pctx, "parseQueryResponse", args)
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

// validateUpstreamTarget 主链路上游目标校验：相对路径放行（由宿主拼渠道 baseURL）；
// 绝对 URL 的 host 必须 ∈ 渠道 baseURL host ∪ manifest.allowedHosts——与 utils.fetch
// 白名单同语义，兑现契约 v1「绝对 URL host 须在白名单内」的声明（此前只写在注释里）。
// 不做 ssrf 内网校验：渠道 baseUrl 本身可指向自托管内网（渠道配置是管理员信任面），
// 白名单命中即为边界；重定向目标由 newWhitelistClient 的 CheckRedirect 同规则把关。
func validateUpstreamTarget(baseURL string, manifest *contract.Manifest, rawURL string) error {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return fmt.Errorf("绝对 URL 不合法：%s", rawURL)
	}
	allowed := append([]string{hostOf(baseURL)}, manifest.AllowedHosts...)
	if !jsplugin.HostAllowed(allowed, u.Host) {
		return fmt.Errorf("目标 host 不在白名单（渠道 baseUrl host ∪ meta.allowedHosts）：%s", u.Host)
	}
	return nil
}

// newWhitelistClient 主链路 HTTP client：重定向仅允许白名单内 host（阻断经 302
// 跳出白名单的绕过，同时保留渠道内/声明 host 间的合法跳转链）。
// timeout<=0 会落进 NewPooledClient 的共享单例分支（改 CheckRedirect 会污染全局），
// 这里钳位到 30s 兜底。
func newWhitelistClient(timeoutSeconds int, useProxy bool, allowed []string) *http.Client {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	client := common.NewPooledClient(timeoutSeconds, useProxy)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		if !jsplugin.HostAllowed(allowed, req.URL.Host) {
			return fmt.Errorf("重定向目标 host 不在白名单：%s", req.URL.Host)
		}
		return nil
	}
	return client
}

// resolveUpstreamURL 拼接上游地址：相对路径 → baseURL + path；绝对 URL 原样
// （绝对 URL 的 host 白名单校验由 validateUpstreamTarget 在构造描述时完成）。
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

// upstreamErrorMessage 上游非 200 响应体 → 客户端可读消息：逐层剥开常见错误信封
// （顶层 {code?,message} 经销商信封 / {error:{code?,message}} OpenAI·Ark 信封 /
// 信封内嵌 stringify 的 JSON——经销商把上游错误整包塞进 message 的形态），最多三层，
// 取最内层的纯文本消息与错误码；剥不开（HTML 错误页/纯文本/未知结构）回落原文，
// 经 truncateMessage 脱敏截断后返回。
func upstreamErrorMessage(body []byte) string {
	current := string(bytes.TrimSpace(body))
	code := ""
	for depth := 0; depth < 3; depth++ {
		obj, err := contract.ParseJSONObject(json.RawMessage(current))
		if err != nil {
			break
		}
		message, envelopeCode := envelopeMessage(obj)
		if envelopeCode != "" {
			code = envelopeCode // 内层信封的错误码更具体，覆盖外层
		}
		if message == "" {
			break
		}
		current = message
		// 消息不再是 JSON 对象形态（拿到纯文本）即到达终点
		trimmed := strings.TrimSpace(current)
		if !strings.HasPrefix(trimmed, "{") {
			break
		}
	}
	message := strings.TrimSpace(current)
	if code != "" && message != "" && !strings.Contains(message, code) {
		message = code + ": " + message
	}
	return truncateMessage(message)
}

// envelopeMessage 从错误信封对象取消息与错误码：顶层 message 优先（经销商信封），
// 回落 error.message（OpenAI/Ark 信封）。返回的 message 可能仍是 stringify 的 JSON
// （由 upstreamErrorMessage 继续剥层）；都没有时返回空。
func envelopeMessage(obj map[string]json.RawMessage) (message, code string) {
	if c, ok := rawStringValue(obj["code"]); ok && c != "" {
		code = c
	}
	if m, ok := rawStringValue(obj["message"]); ok && strings.TrimSpace(m) != "" {
		return m, code
	}
	if e, ok := obj["error"]; ok {
		var inner map[string]json.RawMessage
		if json.Unmarshal(e, &inner) == nil {
			if c, ok := rawStringValue(inner["code"]); ok && c != "" && code == "" {
				code = c
			}
			if m, ok := rawStringValue(inner["message"]); ok && strings.TrimSpace(m) != "" {
				return m, code
			}
		}
	}
	return "", code
}

// rawStringValue JSON 原文 → 字符串值（非字符串形态返回 false）。
func rawStringValue(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, true
	}
	return "", false
}

// truncateMessage 上游错误消息净化（DoResponse 非 200 路径）：HTML 错误页等大响应体
// 原样进 Message 会随错误链路放大进任务记录与日志；错误体还可能内嵌密钥形态
// （{"api_key":...} / ?sig=，writeTaskError 下游的 RedactMessage 只抹 URL/IP 盖不住）。
// 委托 SanitizeHookMessage：脱敏 + 剥控制字符 + 按 rune 截断 512。
func truncateMessage(msg string) string {
	return jsplugin.SanitizeHookMessage(msg)
}

// jsHookTaskError 引擎钩子错误 → TaskError（502 透出）。
// 消息经 SanitizeHookMessage 净化——插件异常是面向用户的业务文案通道（上游 vendor
// message 原样透传），但可能意外内嵌密钥/签名片段，脱敏+截断后透出，不泛化。
func jsHookTaskError(err error) *common.TaskError {
	return &common.TaskError{StatusCode: http.StatusBadGateway, Message: jsplugin.SanitizeHookMessage(err.Error())}
}
