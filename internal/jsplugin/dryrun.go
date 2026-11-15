package jsplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// 试运行（上传前自检）：把 hailuo 那类「上传成功、激活成功、运行期才暴露」的缺陷
// 提前到上传期。整个过程不落库、不发布 generation、不发真实 HTTP——样例执行的
// fetch 白名单故意留空（任何 utils.fetch 都会被拒），从机制上杜绝外呼。
//
// 检查项（按序执行，单项失败不阻断后续可执行的检查）：
//   - compile         编译 + manifest 校验 + 加密配置类型限制（与 Upload 同口径）
//   - hooks           钩子齐备性（必选钩子缺失 = fail；渲染钩子缺失 = warn 走默认透传）
//   - conflict        与当前在服务插件的跨插件路由/模型冲突预检（复用激活时的规则）
//   - sample-decode   样例请求执行 decode 钩子并校验返回契约
//   - sample-build    样例执行 buildSubmitRequest 并校验返回契约
//
// 编译失败是唯一例外：后续检查全部无从谈起，只返回一条 fail。

// 检查状态。
const (
	dryRunPass = "pass"
	dryRunWarn = "warn"
	dryRunFail = "fail"
)

// dryRunSampleTimeout 样例执行的单钩子超时（比生产默认 5s 收紧——试运行不该久等）。
const dryRunSampleTimeout = 3 * time.Second

// dryRunSampleParamValue 样例路径参数填充值：:param 段一律填 "sample"。
const dryRunSampleParamValue = "sample"

// DryRunReport 试运行报告：manifest（编译通过时）+ 逐项检查结果。
type DryRunReport struct {
	Manifest *contract.Manifest `json:"manifest,omitempty"`
	Checks   []DryRunCheck      `json:"checks"`
}

// DryRunCheck 单项检查结果。
type DryRunCheck struct {
	Name   string `json:"name"`   // compile | hooks | conflict | sample-decode | sample-build | lint-fetch-url
	Status string `json:"status"` // pass=通过 / warn=提示（不阻断上传） / fail=缺陷（不建议上传）
	Detail string `json:"detail"` // 中文说明
}

// HasFail 报告是否存在 fail 项（true = 存在上传/激活后必然出错的缺陷）。
func (r *DryRunReport) HasFail() bool {
	for _, c := range r.Checks {
		if c.Status == dryRunFail {
			return true
		}
	}
	return false
}

// DryRun 执行上传前试运行，永不返回 error——所有失败都体现在报告的检查项里，
// 由前端按 fail/warn 分级呈现（编译失败也只是一条 fail，不是接口错误）。
func DryRun(source string) *DryRunReport {
	report := &DryRunReport{}
	engine := NewGojaEngine()
	compiled, err := engine.Compile(source)
	if err != nil {
		report.Checks = append(report.Checks, DryRunCheck{
			Name:   "compile",
			Status: dryRunFail,
			Detail: "编译或 manifest 校验失败：" + err.Error(),
		})
		return report
	}
	report.Manifest = compiled.Manifest
	// 加密配置类型限制与 Upload 同口径（上传会被拒的源码在试运行期就应报出）
	if err := validateEncryptedConfigTypes(compiled.Manifest); err != nil {
		report.Checks = append(report.Checks, DryRunCheck{
			Name:   "compile",
			Status: dryRunFail,
			Detail: "加密配置类型不合法（上传会被拒）：" + err.Error(),
		})
		return report
	}
	report.Checks = append(report.Checks, DryRunCheck{
		Name:   "compile",
		Status: dryRunPass,
		Detail: fmt.Sprintf("编译通过：%s@%s（%d 条路由 / %d 个模型）",
			compiled.Manifest.Key, compiled.Manifest.Version,
			len(compiled.Manifest.Routes), len(compiled.Manifest.Models)),
	})

	// 样例执行选项：AllowedHosts 故意留空——白名单为空时 utils.fetch 对任何 host
	// 都被拒，杜绝试运行发出真实 HTTP（插件内 fetch 失败按「样例被拒绝」处理）。
	opts := InstanceOptions{CallTimeout: dryRunSampleTimeout}
	inst, instErr := engine.NewInstance(compiled, opts)
	if instErr != nil {
		report.Checks = append(report.Checks, DryRunCheck{
			Name:   "hooks",
			Status: dryRunFail,
			Detail: "临时实例创建失败（顶层执行崩溃/超时）：" + instErr.Error(),
		})
	} else {
		defer inst.Close()
		report.Checks = append(report.Checks, dryRunHookCompleteness(compiled.Manifest, inst.HasHook))
	}
	report.Checks = append(report.Checks, dryRunConflict(compiled.Manifest))

	if instErr == nil {
		if route, ok := dryRunFirstDecodeRoute(compiled.Manifest); ok {
			pool := NewPluginPool(engine, compiled, opts)
			defer pool.Close()
			ctx := withDryRunMarker(context.Background())
			// 缺钩子的样例检查直接跳过（hooks 项已 fail，不重复报）
			decodeHook := route.ResolveDecode()
			var decoded *contract.DecodeResult
			if inst.HasHook(decodeHook) {
				var chk DryRunCheck
				chk, decoded = dryRunSampleDecode(ctx, pool, route, compiled.Manifest, decodeHook)
				report.Checks = append(report.Checks, chk)
			}
			if inst.HasHook("buildSubmitRequest") {
				report.Checks = append(report.Checks, dryRunSampleBuild(ctx, pool, compiled.Manifest, decoded))
			}
		}
	}
	return report
}

// ---------------------------------------------------------------------------
// 钩子齐备性
// ---------------------------------------------------------------------------

// dryRunHookCompleteness 按 manifest.routes 逐条判定运行期必需钩子是否导出：
//   - submit/dynamic 路由：decode 钩子（别名优先，缺省 decodeRequest）+
//     buildSubmitRequest + parseSubmitResponse 必选——分发器与任务桥直接 Pool.Call，缺失即 502
//   - 存在任一 submit/dynamic 路由（能产生任务）：轮询链 buildQueryRequest +
//     parseQueryResponse 必选——任务终态只能靠轮询抵达（immediate 同步终态是少数形态）
//   - render 钩子（别名/缺省 renderSubmit/renderTask/renderTasks）可选：未导出则
//     响应走默认透传（见 jsdispatcher），缺失只提示
func dryRunHookCompleteness(m *contract.Manifest, has func(string) bool) DryRunCheck {
	var missing []string
	reported := map[string]bool{}
	// req 必选钩子检查（按钩子名去重——多条路由共用标准钩子名只报一次；
	// decode 别名每条路由独立成条，不同别名本就是不同缺陷）
	req := func(hook, where string) {
		if reported[hook] {
			return
		}
		reported[hook] = true
		if !has(hook) {
			missing = append(missing, hook+"（"+where+"）")
		}
	}
	// opt 可选渲染钩子缺失收集（按钩子名聚合受影响路由，避免逐路由刷屏）
	renderMissing := map[string][]string{}
	var renderOrder []string
	opt := func(hook, routeDesc string) {
		if has(hook) {
			return
		}
		if _, ok := renderMissing[hook]; !ok {
			renderOrder = append(renderOrder, hook)
		}
		renderMissing[hook] = append(renderMissing[hook], routeDesc)
	}

	hasSubmitRoute := false
	for _, route := range m.Routes {
		switch route.Type {
		case contract.RouteTypeSubmit, contract.RouteTypeDynamic:
			hasSubmitRoute = true
			req(route.ResolveDecode(), fmt.Sprintf("%s %s 的 decode", route.Method, route.Path))
			req("buildSubmitRequest", "提交链")
			req("parseSubmitResponse", "提交链")
			if route.Type == contract.RouteTypeDynamic {
				// dynamic 一型两用：查询侧缺省 renderTasks、提交侧缺省 renderSubmit
				opt(route.ResolveRender("renderTasks"), fmt.Sprintf("%s %s（dynamic 查询侧）", route.Method, route.Path))
				opt(route.ResolveRender(contract.HookNameRenderSubmit), fmt.Sprintf("%s %s（dynamic 提交侧）", route.Method, route.Path))
			} else {
				opt(route.ResolveRender(contract.HookNameRenderSubmit), route.Method+" "+route.Path)
			}
		case contract.RouteTypeQuery:
			opt(route.ResolveRender(contract.HookNameRenderTask), route.Method+" "+route.Path)
		}
	}
	if hasSubmitRoute {
		req("buildQueryRequest", "轮询链")
		req("parseQueryResponse", "轮询链")
	}

	var renderWarns []string
	for _, hook := range renderOrder {
		renderWarns = append(renderWarns, fmt.Sprintf("渲染钩子 %s 未导出（%s），响应走默认透传",
			hook, strings.Join(renderMissing[hook], "、")))
	}

	status, detail := dryRunPass, "必选钩子齐备（decode/提交链/轮询链），渲染钩子已导出"
	switch {
	case len(missing) > 0:
		status = dryRunFail
		detail = "缺少必选钩子：" + strings.Join(missing, "；")
		if len(renderWarns) > 0 {
			detail += "；" + strings.Join(renderWarns, "；")
		}
	case len(renderWarns) > 0:
		status = dryRunWarn
		detail = strings.Join(renderWarns, "；")
	}
	return DryRunCheck{Name: "hooks", Status: status, Detail: detail}
}

// ---------------------------------------------------------------------------
// 跨插件冲突预检
// ---------------------------------------------------------------------------

// dryRunConflict 与当前 generation 内其他插件比路由形状与模型名（复用 Activate 事务
// 内的 checkManifestConflict 同一规则）。命中即 fail——激活会被拒绝，提前告知。
// 只比对在服务插件：禁用插件的冲突由激活时的事务内预检兜底，试运行不读 DB。
func dryRunConflict(m *contract.Manifest) DryRunCheck {
	others := 0
	for _, p := range CurrentGeneration().Plugins() {
		if p.Key == m.Key {
			continue // 同 key 换版本是替换语义，不构成自我冲突
		}
		others++
		if err := checkManifestConflict(m.Key, m, p.Key, p.Manifest); err != nil {
			return DryRunCheck{Name: "conflict", Status: dryRunFail, Detail: "激活将被拒绝：" + err.Error()}
		}
	}
	return DryRunCheck{Name: "conflict", Status: dryRunPass,
		Detail: fmt.Sprintf("与当前 %d 个在服务插件无路由/模型冲突", others)}
}

// ---------------------------------------------------------------------------
// 样例执行
// ---------------------------------------------------------------------------

// dryRunFirstDecodeRoute 取首条 submit/dynamic 路由（decode 仅此类路由适用；
// 纯 query 插件没有解码语义，跳过样例执行）。
func dryRunFirstDecodeRoute(m *contract.Manifest) (contract.Route, bool) {
	for _, route := range m.Routes {
		if route.Type == contract.RouteTypeSubmit || route.Type == contract.RouteTypeDynamic {
			return route, true
		}
	}
	return contract.Route{}, false
}

// dryRunSamplePath 路由声明 → 样例路径与路径参数：:param 段一律填 "sample"。
func dryRunSamplePath(route contract.Route) (string, map[string]string) {
	segs := strings.Split(route.Path, "/")
	params := map[string]string{}
	for i, seg := range segs {
		if strings.HasPrefix(seg, ":") {
			params[seg[1:]] = dryRunSampleParamValue
			segs[i] = dryRunSampleParamValue
		}
	}
	return strings.Join(segs, "/"), params
}

// dryRunSampleBody 构造最小样例请求体：带首个声明模型（decode 返回 submit 时
// ValidateDecoded 要求 model ∈ meta.models，带上它能命中多数插件的模型回显）。
func dryRunSampleBody(model string) json.RawMessage {
	data, _ := json.Marshal(map[string]any{"model": model})
	return data
}

// dryRunSampleDecode 样例请求执行 decode 钩子并校验返回契约。判定分档：
//   - 返回并通过 ValidateDecoded → pass
//   - 返回未过契约校验 / 引擎级失败（实例创建、执行超时）→ fail
//   - 钩子抛错 → warn：样例输入固定（参数一律 "sample"、体只有 model），合法插件的
//     入参校验（如 action 白名单、prompt 必填）完全可能拒绝它；抛错已证明代码可执行，
//     返回契约是否合法请作者以真实请求自测
func dryRunSampleDecode(ctx context.Context, pool *PluginPool, route contract.Route, m *contract.Manifest, decodeHook string) (DryRunCheck, *contract.DecodeResult) {
	samplePath, params := dryRunSamplePath(route)
	args, _ := json.Marshal(contract.DecodeContext{
		Method: route.Method,
		Path:   samplePath,
		Params: params,
		Body:   dryRunSampleBody(m.Models[0]),
	})
	out, err := pool.Call(ctx, decodeHook, args)
	if err != nil {
		return dryRunCallErrorCheck("sample-decode", decodeHook, err), nil
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		return DryRunCheck{Name: "sample-decode", Status: dryRunFail,
			Detail: decodeHook + " 返回不是 JSON 对象：" + SanitizeHookMessage(err.Error())}, nil
	}
	decoded, err := contract.ValidateDecoded(raw, m, route.Type == contract.RouteTypeDynamic)
	if err != nil {
		return DryRunCheck{Name: "sample-decode", Status: dryRunFail,
			Detail: decodeHook + " 返回未通过契约校验：" + SanitizeHookMessage(err.Error())}, nil
	}
	detail := fmt.Sprintf("样例请求解码通过：kind=%s", decoded.Kind)
	if decoded.Kind == "submit" {
		detail += "，model=" + decoded.Model
	} else {
		detail += fmt.Sprintf("，taskIds=%d 个", len(decoded.TaskIDs))
	}
	return DryRunCheck{Name: "sample-decode", Status: dryRunPass, Detail: detail}, decoded
}

// dryRunSampleBuild 样例执行 buildSubmitRequest：入参取 decode 的样例结果（decode
// 被拒绝或跳过时回退最小样例体），校验返回契约。返回相对路径属正常形态——提交时
// 由宿主拼接到渠道 baseUrl；绝对 URL 的 host 在运行期须 fetch 白名单内。
func dryRunSampleBuild(ctx context.Context, pool *PluginPool, m *contract.Manifest, decoded *contract.DecodeResult) DryRunCheck {
	model := m.Models[0]
	body := dryRunSampleBody(model)
	if decoded != nil && decoded.Kind == "submit" && len(decoded.RequestBody) > 0 {
		model = decoded.Model
		body = decoded.RequestBody
	}
	args, _ := json.Marshal(contract.BuildSubmitContext{
		Body:           body,
		Model:          model,
		UpstreamModel:  model,
		ChannelBaseURL: "https://dryrun.invalid",
		ApiKey:         "dry-run",
	})
	out, err := pool.Call(ctx, "buildSubmitRequest", args)
	if err != nil {
		return dryRunCallErrorCheck("sample-build", "buildSubmitRequest", err)
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		return DryRunCheck{Name: "sample-build", Status: dryRunFail,
			Detail: "buildSubmitRequest 返回不是 JSON 对象：" + SanitizeHookMessage(err.Error())}
	}
	built, err := contract.ValidateBuiltRequest(raw)
	if err != nil {
		return DryRunCheck{Name: "sample-build", Status: dryRunFail,
			Detail: "buildSubmitRequest 返回未通过契约校验：" + SanitizeHookMessage(err.Error())}
	}
	detail := fmt.Sprintf("样例提交请求构造通过：返回相对路径 %s（提交时拼接到渠道 baseUrl，正常）", built.URL)
	if !strings.HasPrefix(built.URL, "/") {
		detail = "样例提交请求构造通过：返回绝对 URL（运行期 host 须在白名单：渠道 host ∪ allowedHosts）"
	}
	return DryRunCheck{Name: "sample-build", Status: dryRunPass, Detail: detail}
}

// dryRunCallErrorCheck 样例执行的调用错误分档（引擎级失败 → fail，钩子抛错 → warn）。
func dryRunCallErrorCheck(name, hook string, err error) DryRunCheck {
	msg := SanitizeHookMessage(err.Error())
	if dryRunEngineLevelFailure(err) {
		return DryRunCheck{Name: name, Status: dryRunFail, Detail: hook + " 样例执行失败：" + msg}
	}
	return DryRunCheck{Name: name, Status: dryRunWarn,
		Detail: fmt.Sprintf("样例输入被 %s 拒绝（%s）：样例参数固定，可能未通过插件自身入参校验；请以真实请求验证返回契约", hook, msg)}
}

// dryRunEngineLevelFailure 判定样例调用错误是否引擎级失败（fail 档）：实例创建失败
// （顶层崩溃/超时）、执行超时（疑似死循环）、钩子未导出（齐备性检查漏网）。
// 其余为钩子内 JS 异常——样例输入固定，多为插件自身入参校验拒绝，归 warn 档。
func dryRunEngineLevelFailure(err error) bool {
	msg := err.Error()
	for _, marker := range []string{"创建实例失败", "插件加载失败", "执行超时", "钩子未导出"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 观测抑制
// ---------------------------------------------------------------------------

// dryRunMarkerKey 试运行调用的 context 标记。样例执行复用 PluginPool（与生产同一条
// 调用路径），但指标不应计入——插件尚未入库，会以幽灵 key 混进管理端指标列表。
type dryRunMarkerKey struct{}

func withDryRunMarker(ctx context.Context) context.Context {
	return context.WithValue(ctx, dryRunMarkerKey{}, struct{}{})
}

func isDryRunCall(ctx context.Context) bool {
	_, ok := ctx.Value(dryRunMarkerKey{}).(struct{})
	return ok
}
