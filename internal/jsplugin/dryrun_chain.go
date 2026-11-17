package jsplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// 试运行调用链后半段：用量估算、标准视频端点解码、提交响应解析、轮询链、渲染。
//
// 输入分两类，错误判定口径不同：
//   - 样例请求（estimateUsage / decodeVideos）：入参贴近真实请求，插件拒绝 = warn（与 decode/build 同口径）
//   - 构造的上游响应（parseSubmitResponse / 轮询链 / 渲染）：样例响应不可能匹配各厂商字段，
//     插件拒绝属预期 = pass（附说明）；只有编程错误与返回契约违规才 fail
//
// 编程错误（ReferenceError、调用非函数、宿主 API 误用）与输入无关、真实请求同样必现，任何
// 环节一律 fail。

// dryRunSampleTaskID 构造上游响应中的任务 ID。
const dryRunSampleTaskID = "dryrun-task-1"

// dryRunFakeSubmitResponses 构造的上游受理响应，按序尝试：厂商对 data 成员的形态互斥
// （suno/golden 式 data 即任务 ID 字符串，kling/doubao 式 data 为对象），单一形态会让
// 另一类插件产出错误类型的 taskId 而误报契约违规。任一形态通过即视为通过。
var dryRunFakeSubmitResponses = []json.RawMessage{
	json.RawMessage(`{"id":"` + dryRunSampleTaskID + `","task_id":"` + dryRunSampleTaskID +
		`","code":"success","status":"queued","data":"` + dryRunSampleTaskID + `"}`),
	json.RawMessage(`{"id":"` + dryRunSampleTaskID + `","task_id":"` + dryRunSampleTaskID +
		`","code":"success","status":"queued","data":{"id":"` + dryRunSampleTaskID + `","task_id":"` + dryRunSampleTaskID + `","status":"queued"}}`),
}

// dryRunFakeQueryResponse 构造的上游查询响应：处理中形态（插件应返回 done:false）。
var dryRunFakeQueryResponse = json.RawMessage(`{"id":"` + dryRunSampleTaskID + `","task_id":"` + dryRunSampleTaskID +
	`","status":"processing","data":{"status":"processing"}}`)

// dryRunHostMisuseMarkers 宿主函数参数误用的报错片段（hostfunc.go 抛出，带 GoError 前缀）。
// 白名单拒绝、相对路径无渠道上下文属试运行环境限制，不在此列。
var dryRunHostMisuseMarkers = []string{
	"utils.fetch 需要一个请求对象参数",
	"utils.fetch 参数必须是对象",
	"参数不足",
	"method 仅支持 GET/HEAD/POST",
	"responseType 仅支持",
	"claims 必须是对象",
	"body 无法序列化为 JSON",
}

// dryRunProgrammingError 判定与输入无关的编程错误：未定义标识符、把非函数当函数调用、宿主 API 误用。
// 「访问 undefined 的属性 / 对象无某成员」可能源于样例缺字段，不在此列。
func dryRunProgrammingError(msg string) bool {
	if strings.Contains(msg, "ReferenceError") || strings.Contains(msg, "Not a function") {
		return true
	}
	if idx := strings.Index(msg, "GoError: "); idx >= 0 {
		rest := msg[idx:]
		for _, marker := range dryRunHostMisuseMarkers {
			if strings.Contains(rest, marker) {
				return true
			}
		}
	}
	return false
}

// dryRunInputDependentError 判定可能由样例缺字段引起的 TypeError。
func dryRunInputDependentError(msg string) bool {
	return strings.Contains(msg, "TypeError")
}

// dryRunRejectionCheck 构造输入被插件拒绝时的判定（编程错误/引擎级失败已由调用方先行判定）。
func dryRunRejectionCheck(name, hook, msg string, fabricated bool) DryRunCheck {
	if fabricated {
		return DryRunCheck{Name: name, Status: dryRunPass,
			Detail: fmt.Sprintf("%s 拒绝了构造的样例上游响应（%s）：样例不含厂商字段属正常，未发现运行期编程错误", hook, msg)}
	}
	if dryRunInputDependentError(msg) {
		return DryRunCheck{Name: name, Status: dryRunWarn,
			Detail: fmt.Sprintf("样例输入下 %s 访问了不存在的字段（%s）：样例参数固定可能缺字段；若真实请求也可能缺该字段，请先判空", hook, msg)}
	}
	return DryRunCheck{Name: name, Status: dryRunWarn,
		Detail: fmt.Sprintf("样例输入被 %s 拒绝（%s）：样例参数固定，可能未通过插件自身入参校验；请以真实请求验证返回契约", hook, msg)}
}

// dryRunClassifyCallError 钩子调用错误分档：引擎级失败 / 编程错误 → fail，其余按输入类别判定。
func dryRunClassifyCallError(name, hook string, err error, fabricated bool) DryRunCheck {
	msg := SanitizeHookMessage(err.Error())
	if dryRunEngineLevelFailure(err) {
		return DryRunCheck{Name: name, Status: dryRunFail, Detail: hook + " 样例执行失败：" + msg}
	}
	if dryRunProgrammingError(msg) {
		return DryRunCheck{Name: name, Status: dryRunFail,
			Detail: hook + " 运行期编程错误（与样例输入无关，真实请求同样必现）：" + msg}
	}
	return dryRunRejectionCheck(name, hook, msg, fabricated)
}

// dryRunCallObject 调用钩子并解析为 JSON 对象；调用失败返回已分档的检查结果。
func dryRunCallObject(ctx context.Context, pool *PluginPool, name, hook string, args any, fabricated bool) (map[string]json.RawMessage, *DryRunCheck) {
	payload, _ := json.Marshal(args)
	out, err := pool.Call(ctx, hook, payload)
	if err != nil {
		chk := dryRunClassifyCallError(name, hook, err, fabricated)
		return nil, &chk
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		return nil, &DryRunCheck{Name: name, Status: dryRunFail,
			Detail: hook + " 返回不是 JSON 对象（钩子须同步返回对象，不能是 async/Promise）：" + SanitizeHookMessage(err.Error())}
	}
	return raw, nil
}

// dryRunContractFail 返回契约违规的 fail 结果。
func dryRunContractFail(name, hook string, err error) DryRunCheck {
	return DryRunCheck{Name: name, Status: dryRunFail,
		Detail: hook + " 返回未通过契约校验：" + SanitizeHookMessage(err.Error())}
}

// dryRunValidateUsage 校验用量事实（键 ∈ usageSchema、number 型非负数值）。
func dryRunValidateUsage(usage json.RawMessage, m *contract.Manifest) error {
	if len(usage) == 0 || string(usage) == "null" {
		return nil
	}
	raw, err := contract.ParseJSONObject(usage)
	if err != nil {
		return fmt.Errorf("usage 不是对象：%w", err)
	}
	_, err = contract.ValidateUsageFacts(raw, m)
	return err
}

// ---------------------------------------------------------------------------
// 用量估算
// ---------------------------------------------------------------------------

// dryRunSampleEstimate 样例执行 estimateUsage（入参取 decode 样例结果），校验用量事实契约。
func dryRunSampleEstimate(ctx context.Context, pool *PluginPool, m *contract.Manifest, model string, body json.RawMessage, params map[string]string) DryRunCheck {
	const name, hook = "sample-estimate", "estimateUsage"
	raw, chk := dryRunCallObject(ctx, pool, name, hook, contract.EstimateUsageContext{Body: body, Model: model, Params: params}, false)
	if chk != nil {
		return *chk
	}
	data, _ := json.Marshal(raw)
	if err := dryRunValidateUsage(data, m); err != nil {
		return dryRunContractFail(name, hook, err)
	}
	return DryRunCheck{Name: name, Status: dryRunPass, Detail: "样例用量估算通过：" + string(data)}
}

// ---------------------------------------------------------------------------
// 标准视频端点解码
// ---------------------------------------------------------------------------

// dryRunSampleStandard 以通用任务体样例执行 standardVideos 解码钩子。逐个模型尝试
// （多模型插件常只有部分是视频模型，如 golden_music/golden_video），首个通过即 pass；
// 编程错误/契约违规立即 fail；全部被拒为 warn。
func dryRunSampleStandard(ctx context.Context, pool *PluginPool, m *contract.Manifest) DryRunCheck {
	const name = "standard-endpoint"
	hook := m.StandardVideos
	var firstReject *DryRunCheck
	for _, model := range m.Models {
		body, _ := json.Marshal(map[string]any{
			"model":    model,
			"prompt":   "dry run sample",
			"metadata": map[string]any{},
		})
		raw, chk := dryRunCallObject(ctx, pool, name, hook, contract.StandardEntryContext{
			Surface: contract.SurfaceOpenAIVideos, Path: "/v1/videos", Body: body, Model: model,
		}, false)
		if chk != nil {
			if chk.Status == dryRunFail {
				return *chk
			}
			if firstReject == nil {
				firstReject = chk
			}
			continue
		}
		decoded, err := contract.ValidateDecoded(raw, m, false)
		if err != nil {
			return dryRunContractFail(name, hook, err)
		}
		if decoded.Kind != "submit" {
			return DryRunCheck{Name: name, Status: dryRunFail, Detail: hook + " 返回 kind 须为 submit（标准端点无查询语义）"}
		}
		return DryRunCheck{Name: name, Status: dryRunPass,
			Detail: fmt.Sprintf("标准视频端点已支持：模型可经 /v1/videos 与 /v1/video/generations 提交（解码钩子 %s，样例 model=%s 通过）", hook, decoded.Model)}
	}
	if firstReject != nil {
		return *firstReject
	}
	return DryRunCheck{Name: name, Status: dryRunWarn, Detail: hook + " 未执行样例（无声明模型）"}
}

// ---------------------------------------------------------------------------
// 提交响应解析 + 轮询链
// ---------------------------------------------------------------------------

// dryRunSampleParseSubmit 以构造的上游受理响应执行 parseSubmitResponse（多形态按序尝试，
// 见 dryRunFakeSubmitResponses）。判定：任一形态通过 → pass；任一形态出现编程错误 → fail；
// 全部形态都产出契约违规 → fail（与响应形态无关的违规）；其余（插件拒绝）→ pass。
// 返回轮询链要用的任务引用：解析通过时取插件自己产出的 taskId/taskData/state，否则回退构造的任务 ID。
func dryRunSampleParseSubmit(ctx context.Context, pool *PluginPool, m *contract.Manifest, route contract.Route, model string, body json.RawMessage) (DryRunCheck, contract.QueryTaskRef, bool) {
	const name, hook = "sample-parse-submit", "parseSubmitResponse"
	fallback := contract.QueryTaskRef{TaskID: dryRunSampleTaskID}
	samplePath, params := dryRunSamplePath(route)
	var rejected, violated *DryRunCheck
	for _, resp := range dryRunFakeSubmitResponses {
		raw, chk := dryRunCallObject(ctx, pool, name, hook, contract.ParseSubmitContext{
			Status: 200, Path: samplePath, Params: params, Model: model,
			ResponseBody: resp, Body: body,
		}, true)
		if chk != nil {
			if chk.Status == dryRunFail {
				return *chk, fallback, false
			}
			rejected = chk
			continue
		}
		res, err := contract.ValidateSubmitted(raw)
		if err == nil && res.Im != nil {
			if uerr := dryRunValidateUsage(res.Im.Usage, m); uerr != nil {
				err = fmt.Errorf("immediate.%w", uerr)
			}
		}
		if err != nil {
			c := dryRunContractFail(name, hook, err)
			violated = &c
			continue
		}
		ref := contract.QueryTaskRef{TaskID: res.TaskID, TaskData: res.TaskData, State: res.State}
		return DryRunCheck{Name: name, Status: dryRunPass,
			Detail: fmt.Sprintf("构造的受理响应解析通过：taskId=%s", res.TaskID)}, ref, true
	}
	if violated != nil && rejected == nil {
		return *violated, fallback, false
	}
	return *rejected, fallback, false
}

// dryRunSampleQuery 执行轮询链：buildQueryRequest（任务引用来自 parseSubmitResponse 或构造值）+
// parseQueryResponse（构造的处理中响应），校验请求/结果契约与 items 用量事实。
// 返回 parseQueryResponse 产出的 taskData（渲染样例使用）。
func dryRunSampleQuery(ctx context.Context, pool *PluginPool, m *contract.Manifest, model string, ref contract.QueryTaskRef, refFromPlugin bool) (DryRunCheck, json.RawMessage) {
	const name = "sample-query"
	tasks := []contract.QueryTaskRef{ref}

	// buildQueryRequest：任务引用取自插件自己的解析结果时属真实输入（拒绝 = warn），
	// 回退构造值时插件可能依赖缺失的 taskData/state（拒绝 = pass）
	raw, chk := dryRunCallObject(ctx, pool, name, "buildQueryRequest",
		contract.BuildQueryContext{Tasks: tasks, Model: model}, !refFromPlugin)
	if chk != nil {
		return *chk, nil
	}
	built, err := contract.ValidateBuiltRequest(raw)
	if err != nil {
		return dryRunContractFail(name, "buildQueryRequest", err), nil
	}

	raw, chk = dryRunCallObject(ctx, pool, name, "parseQueryResponse", contract.ParseQueryContext{
		Status: 200, ResponseBody: dryRunFakeQueryResponse, Tasks: tasks, ChannelBaseURL: "https://dryrun.invalid",
	}, true)
	if chk != nil {
		if chk.Status == dryRunPass {
			chk.Detail = fmt.Sprintf("buildQueryRequest 通过（%s）；", built.URL) + chk.Detail
		}
		return *chk, nil
	}
	res, err := contract.ValidateQueried(raw, tasks)
	if err != nil {
		return dryRunContractFail(name, "parseQueryResponse", err), nil
	}
	var taskData json.RawMessage
	for i, item := range res.Items {
		if err := dryRunValidateUsage(item.Usage, m); err != nil {
			return dryRunContractFail(name, "parseQueryResponse", fmt.Errorf("items[%d].%w", i, err)), nil
		}
		if item.TaskID == ref.TaskID {
			taskData = item.TaskData
		}
	}
	return DryRunCheck{Name: name, Status: dryRunPass,
		Detail: fmt.Sprintf("轮询链通过：buildQueryRequest → %s；parseQueryResponse 返回 %d 个结果", built.URL, len(res.Items))}, taskData
}

// ---------------------------------------------------------------------------
// 渲染
// ---------------------------------------------------------------------------

// dryRunRenderHooks 按路由解析已导出的渲染钩子（名 → 是否批量 renderTasks 形态）。
func dryRunRenderHooks(m *contract.Manifest, has func(string) bool) ([]string, map[string]bool) {
	var order []string
	batch := map[string]bool{}
	add := func(hook string, isBatch bool) {
		if hook == "" || !has(hook) {
			return
		}
		if _, ok := batch[hook]; ok {
			return
		}
		order = append(order, hook)
		batch[hook] = isBatch
	}
	for _, route := range m.Routes {
		switch route.Type {
		case contract.RouteTypeSubmit:
			add(route.ResolveRender(contract.HookNameRenderSubmit), false)
		case contract.RouteTypeQuery:
			add(route.ResolveRender(contract.HookNameRenderTask), false)
		case contract.RouteTypeDynamic:
			add(route.ResolveRender(contract.HookNameRenderSubmit), false)
			add(route.ResolveRender("renderTasks"), true)
		}
	}
	return order, batch
}

// dryRunSampleRender 以构造的进行中任务执行渲染钩子，校验返回契约（{status?, body}）。
func dryRunSampleRender(ctx context.Context, pool *PluginPool, m *contract.Manifest, hooks []string, batch map[string]bool, taskData json.RawMessage) DryRunCheck {
	const name = "sample-render"
	if len(taskData) == 0 || string(taskData) == "null" {
		taskData = dryRunFakeQueryResponse
	}
	task := contract.RenderTask{
		PublicTaskID: "task_dryrun", TaskID: dryRunSampleTaskID, Model: m.Models[0],
		Status: "IN_PROGRESS", Progress: "50%", Data: taskData,
	}
	var passed []string
	for _, hook := range hooks {
		var args any = contract.RenderTaskContext{Task: task}
		if batch[hook] {
			args = contract.RenderTasksContext{Tasks: []contract.RenderTask{task}}
		}
		raw, chk := dryRunCallObject(ctx, pool, name, hook, args, true)
		if chk != nil {
			if chk.Status != dryRunPass {
				return *chk
			}
			passed = append(passed, hook+"（拒绝构造任务，属正常）")
			continue
		}
		if _, err := contract.ValidateRendered(raw); err != nil {
			return dryRunContractFail(name, hook, err)
		}
		passed = append(passed, hook)
	}
	return DryRunCheck{Name: name, Status: dryRunPass, Detail: "渲染钩子样例通过：" + strings.Join(passed, "、")}
}
