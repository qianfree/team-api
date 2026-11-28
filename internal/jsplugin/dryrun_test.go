package jsplugin

import (
	"os"
	"strings"
	"testing"
)

// dryRunGoldenSource 读取 golden 参考实现源码（作者模板 template/plugin.js，
// 与 jsbridge adaptor_test 同一夹具）。
func dryRunGoldenSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("template/plugin.js")
	if err != nil {
		t.Fatalf("读取 golden 模板失败: %v", err)
	}
	return string(data)
}

// dryRunChecksByName 报告按检查项名索引。
func dryRunChecksByName(t *testing.T, report *DryRunReport) map[string]DryRunCheck {
	t.Helper()
	byName := make(map[string]DryRunCheck, len(report.Checks))
	for _, c := range report.Checks {
		byName[c.Name] = c
	}
	return byName
}

// dryRunFixturePlugin 构造一个契约完整的测试插件源码（key/model 可注入）。
func dryRunFixturePlugin(key, model string) string {
	return `export const meta = {
		apiVersion: 1,
		key: "` + key + `",
		name: "DryRun Fixture",
		version: "1.0.0",
		models: ["` + model + `"],
		routes: [{ method: "POST", path: "/create", type: "submit" }],
	};
	export function decodeRequest(ctx) {
		return { kind: "submit", model: "` + model + `", requestBody: ctx.body || {} };
	}
	export function buildSubmitRequest(ctx) {
		return { url: "/v1/create", method: "POST", body: ctx.body || {} };
	}
	export function parseSubmitResponse(ctx) {
		return { taskId: String((ctx.responseBody || {}).data || "t-1") };
	}
	export function buildQueryRequest(ctx) {
		const t = (ctx.tasks || [])[0] || {};
		return { url: "/v1/query?task_id=" + encodeURIComponent(t.taskId), method: "GET" };
	}
	export function parseQueryResponse(ctx) {
		const t = (ctx.tasks || [])[0] || {};
		return { items: [{ taskId: t.taskId, done: true }] };
	}
	`
}

// dryRunPatch 在夹具源码上做一次替换（替换目标不存在时测试直接失败，防夹具漂移后静默失效）。
func dryRunPatch(t *testing.T, src, old, repl string) string {
	t.Helper()
	if !strings.Contains(src, old) {
		t.Fatalf("夹具中不存在替换目标: %s", old)
	}
	return strings.Replace(src, old, repl, 1)
}

// TestDryRunRoutesOmittedStandardOnly 纯标准端点插件（省略 routes，仅声明
// standardVideos）：编译通过、钩子齐备性仍要求提交/轮询链、样例链以标准端点
// 解码结果为起点全部执行；反向（两者皆缺）编译 fail。
func TestDryRunRoutesOmittedStandardOnly(t *testing.T) {
	src := `export const meta = {
  apiVersion: 1, key: "stdonly", name: "Std Only", version: "1.0.0",
  models: ["m1"], standardVideos: "decodeVideos",
};
export function decodeVideos(ctx) {
  return { kind: "submit", model: ctx.model, requestBody: { prompt: "p" } };
}
export function buildSubmitRequest(ctx) {
  return { url: "/v1/videos", method: "POST", body: { model: ctx.upstreamModel || ctx.model, prompt: "p" } };
}
export function parseSubmitResponse(ctx) { return { taskId: "t_std" }; }
export function buildQueryRequest(ctx) {
  return { url: "/v1/videos/" + ctx.tasks[0].taskId, method: "GET" };
}
export function parseQueryResponse(ctx) {
  return { items: [{ taskId: ctx.tasks[0].taskId, done: false, progress: 10 }] };
}
`
	report := DryRun(src)
	if report.Manifest == nil || report.Manifest.Key != "stdonly" {
		t.Fatalf("纯标准端点插件应编译通过: %+v", report.Checks)
	}
	if report.HasFail() {
		t.Fatalf("纯标准端点插件不应有 fail 项: %+v", report.Checks)
	}
	byName := dryRunChecksByName(t, report)
	for _, name := range []string{"compile", "hooks", "standard-endpoint", "sample-build", "sample-parse-submit", "sample-query"} {
		if c, ok := byName[name]; !ok || c.Status != dryRunPass {
			t.Errorf("检查 %s 期望 pass，实际存在=%v %+v", name, ok, c)
		}
	}
	// 零路由插件无原生渲染钩子可检：sample-render 不产生（非缺陷）

	// 反向：省略 routes 且未声明 standardVideos = 无任何入口，编译 fail
	bad := DryRun(`export const meta = {
  apiVersion: 1, key: "noroute", name: "No Route", version: "1.0.0", models: ["m"],
};
export function decodeRequest(ctx) { return { kind: "submit", model: "m" }; }
export function buildSubmitRequest(ctx) { return { url: "/s" }; }
export function parseSubmitResponse(ctx) { return { taskId: "t" }; }
export function buildQueryRequest(ctx) { return { url: "/q" }; }
export function parseQueryResponse(ctx) { return { items: [{ taskId: ctx.tasks[0].taskId, done: true }] }; }
`)
	if !bad.HasFail() {
		t.Fatal("routes 与 standardVideos 均缺省应编译 fail（插件不可达）")
	}
}

// TestDryRunBuiltinsFullChain 全部内置插件：调用链全量样例执行无 fail、静态检查全 pass
// （新检查项不得对存量合格插件误报）。
func TestDryRunBuiltinsFullChain(t *testing.T) {
	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := builtinFS.ReadFile("builtin/" + entry.Name() + "/plugin.js")
		if err != nil {
			t.Fatal(err)
		}
		report := DryRun(string(data))
		byName := dryRunChecksByName(t, report)
		for _, c := range report.Checks {
			if c.Status == dryRunFail {
				t.Errorf("内置插件 %s 检查 %s 意外 fail: %s", entry.Name(), c.Name, c.Detail)
			}
		}
		// 渲染钩子仅源自原生路由：零路由（纯标准端点，如 teamapi）无 sample-render 项
		required := []string{"lint-host-api", "lint-module-state", "sample-parse-submit", "sample-query"}
		if len(report.Manifest.Routes) > 0 {
			required = append(required, "sample-render")
		}
		for _, name := range required {
			if c, ok := byName[name]; !ok || c.Status != dryRunPass {
				t.Errorf("内置插件 %s 检查 %s 期望 pass，实际存在=%v %+v", entry.Name(), name, ok, c)
			}
		}
	}
}

// TestDryRunGoldenFullChain golden：后半段调用链（估算/标准端点/受理解析/轮询/渲染）全部 pass。
func TestDryRunGoldenFullChain(t *testing.T) {
	byName := dryRunChecksByName(t, DryRun(dryRunGoldenSource(t)))
	for _, name := range []string{"standard-endpoint", "sample-estimate", "sample-parse-submit", "sample-query", "sample-render"} {
		c, ok := byName[name]
		if !ok || c.Status != dryRunPass {
			t.Errorf("检查 %s 期望 pass，实际 %+v", name, c)
		}
	}
	// 多模型插件逐个尝试：golden_music 被 decodeVideos 拒绝后 golden_video 通过
	if !strings.Contains(byName["standard-endpoint"].Detail, "golden_video") {
		t.Errorf("standard-endpoint 应以 golden_video 样例通过: %s", byName["standard-endpoint"].Detail)
	}
}

// TestDryRunLintHostAPI 宿主 API 静态检查：AI 生成插件的高频错误写法一律 fail，合法写法与注释不误报。
func TestDryRunLintHostAPI(t *testing.T) {
	cases := []struct {
		name, code string
		fail       bool
		want       string
	}{
		{"log 直调", `utils.log("x");`, true, "utils.log 是对象"},
		{"fetch 字符串首参", `utils.fetch("/v1/x", { method: "GET" });`, true, "单个对象"},
		{"fetch 两参", `utils.fetch(url, opts);`, true, "单个对象"},
		{"不存在的成员", `utils.sleep(10);`, true, "utils.sleep 不存在"},
		{"不存在的子成员", `utils.crypto.md5("a");`, true, "utils.crypto.md5 不存在"},
		{"log 不存在的级别", `utils.log.debug("a");`, true, "utils.log.debug 不存在"},
		{"合法写法", `utils.fetch({ url: "/v1/x" }); utils.log.info("a", {k: 1}); utils.config.get("k");
			utils.crypto.hmacSHA256("m", "s"); utils.now(); utils.uuid();`, false, ""},
		{"注释里的错误写法不计", "// utils.log(\"x\")\n/* utils.fetch(\"/a\", {}) */\nconst u = \"https://a.b\"; // utils.sleep()", false, ""},
	}
	for _, tc := range cases {
		c := dryRunLintHostAPI("export function f(ctx) {\n" + tc.code + "\n}")
		if tc.fail != (c.Status == dryRunFail) || (tc.want != "" && !strings.Contains(c.Detail, tc.want)) {
			t.Errorf("%s: 期望 fail=%v 含 %q，实际 %s: %s", tc.name, tc.fail, tc.want, c.Status, c.Detail)
		}
	}
}

// TestDryRunLintModuleState 模块作用域可变状态：顶层 let/var 与空容器常量 warn，只读常量不报。
func TestDryRunLintModuleState(t *testing.T) {
	c := dryRunLintModuleState("let lastBody = null;\nconst cache = {};\nexport const meta = {};\nconst MAP = { a: 1 };\nfunction f() {\n  let local = 1;\n}")
	if c.Status != dryRunWarn || !strings.Contains(c.Detail, "lastBody") || !strings.Contains(c.Detail, "cache") {
		t.Errorf("期望 warn 并列出 lastBody/cache，实际 %s: %s", c.Status, c.Detail)
	}
	if strings.Contains(c.Detail, "MAP") || strings.Contains(c.Detail, "local") {
		t.Errorf("只读常量与函数内局部变量不应报: %s", c.Detail)
	}
	if c := dryRunLintModuleState("const A = 1;\nfunction f() { let x = 2; }"); c.Status != dryRunPass {
		t.Errorf("无可变状态期望 pass，实际 %s: %s", c.Status, c.Detail)
	}
}

// TestDryRunProgrammingErrorFails 编程错误在任何环节都是 fail：构造响应环节（插件拒绝属预期）
// 也不能掩盖 ReferenceError。
func TestDryRunProgrammingErrorFails(t *testing.T) {
	base := dryRunFixturePlugin("dryrun-prog", "dryrun_prog_model")
	// 提交链：调用未定义函数
	src := dryRunPatch(t, base, `return { url: "/v1/create", method: "POST", body: ctx.body || {} };`,
		`return { url: buildUrl(), method: "POST" };`)
	if c := dryRunChecksByName(t, DryRun(src))["sample-build"]; c.Status != dryRunFail || !strings.Contains(c.Detail, "编程错误") {
		t.Errorf("sample-build 期望 fail(编程错误)，实际 %s: %s", c.Status, c.Detail)
	}
	// 轮询链：构造响应输入下的 ReferenceError
	src = dryRunPatch(t, base, `return { items: [{ taskId: t.taskId, done: true }] };`,
		`return { items: [{ taskId: t.taskId, done: mapStatus(ctx.responseBody) }] };`)
	if c := dryRunChecksByName(t, DryRun(src))["sample-query"]; c.Status != dryRunFail || !strings.Contains(c.Detail, "ReferenceError") {
		t.Errorf("sample-query 期望 fail(ReferenceError)，实际 %s: %s", c.Status, c.Detail)
	}
}

// TestDryRunFabricatedRejectionPasses 插件拒绝构造的上游响应属预期：pass，且轮询链回退构造任务 ID 继续执行。
func TestDryRunFabricatedRejectionPasses(t *testing.T) {
	src := dryRunPatch(t, dryRunFixturePlugin("dryrun-reject", "dryrun_reject_model"),
		`return { taskId: String((ctx.responseBody || {}).data || "t-1") };`,
		`if (ctx.responseBody.vendor_code !== 0) throw new Error("submit rejected");
		return { taskId: "x" };`)
	report := DryRun(src)
	byName := dryRunChecksByName(t, report)
	if c := byName["sample-parse-submit"]; c.Status != dryRunPass || !strings.Contains(c.Detail, "属正常") {
		t.Errorf("sample-parse-submit 期望 pass(拒绝属正常)，实际 %s: %s", c.Status, c.Detail)
	}
	if c := byName["sample-query"]; c.Status != dryRunPass {
		t.Errorf("sample-query 应以构造任务 ID 继续执行并 pass，实际 %s: %s", c.Status, c.Detail)
	}
	if report.HasFail() {
		t.Error("拒绝构造响应不应构成 fail")
	}
}

// TestDryRunUsageAndArtifactContract 用量键不在宿主固定词汇、inline 产物缺 mimeType → fail。
func TestDryRunUsageAndArtifactContract(t *testing.T) {
	withSchema := dryRunFixturePlugin("dryrun-usage", "dryrun_usage_model")

	src := dryRunPatch(t, withSchema, `return { items: [{ taskId: t.taskId, done: true }] };`,
		`return { items: [{ taskId: t.taskId, done: true, usage: { secs: 5 } }] };`)
	if c := dryRunChecksByName(t, DryRun(src))["sample-query"]; c.Status != dryRunFail || !strings.Contains(c.Detail, "secs") {
		t.Errorf("未声明用量键期望 sample-query fail，实际 %s: %s", c.Status, c.Detail)
	}

	src = dryRunPatch(t, withSchema, `return { items: [{ taskId: t.taskId, done: true }] };`,
		`return { items: [{ taskId: t.taskId, done: true, artifacts: [{ key: "video", source: { type: "inline", path: "b64" } }] }] };`)
	if c := dryRunChecksByName(t, DryRun(src))["sample-query"]; c.Status != dryRunFail || !strings.Contains(c.Detail, "mimeType") {
		t.Errorf("inline 缺 mimeType 期望 sample-query fail，实际 %s: %s", c.Status, c.Detail)
	}

	src = withSchema + "\nexport function estimateUsage(ctx) { return { duration: 5 }; }\n"
	if c := dryRunChecksByName(t, DryRun(src))["sample-estimate"]; c.Status != dryRunFail || !strings.Contains(c.Detail, "duration") {
		t.Errorf("estimateUsage 未声明键期望 fail，实际 %s: %s", c.Status, c.Detail)
	}
}

// TestDryRunAsyncHookFails 钩子写成 async（返回 Promise）→ 返回契约 fail（运行期必然失败）。
func TestDryRunAsyncHookFails(t *testing.T) {
	src := dryRunPatch(t, dryRunFixturePlugin("dryrun-async", "dryrun_async_model"),
		"export function buildQueryRequest(ctx) {", "export async function buildQueryRequest(ctx) {")
	if c := dryRunChecksByName(t, DryRun(src))["sample-query"]; c.Status != dryRunFail {
		t.Errorf("async 钩子期望 sample-query fail，实际 %s: %s", c.Status, c.Detail)
	}
}

// TestDryRunRemovedManifestField 带已移除字段（usageSchema/sharedModels）的 manifest → compile fail。
func TestDryRunRemovedManifestField(t *testing.T) {
	src := dryRunPatch(t, dryRunFixturePlugin("dryrun-legacy", "dryrun_legacy_model"),
		`routes: [{ method: "POST", path: "/create", type: "submit" }],`,
		`routes: [{ method: "POST", path: "/create", type: "submit" }],
		usageSchema: [{ key: "seconds", type: "number", unit: "second" }],`)
	if c := dryRunChecksByName(t, DryRun(src))["compile"]; c.Status != dryRunFail || !strings.Contains(c.Detail, "usageSchema") {
		t.Errorf("旧字段 usageSchema 期望 compile fail，实际 %s: %s", c.Status, c.Detail)
	}
}

// TestDryRunGolden golden 参考实现：核心检查项全部 pass 且无任何 fail。
// 样例 decode 为 warn——golden 校验 action ∈ {MUSIC, LYRICS, VIDEO}，固定样例参数被拒属
// 正常分支拒绝；同时证明 config 配置缺失不参与 decode/build（utils.config.get
// 返回 undefined，golden 走默认值），不产生误报。
func TestDryRunGolden(t *testing.T) {
	report := DryRun(dryRunGoldenSource(t))
	if report.Manifest == nil || report.Manifest.Key != "golden" {
		t.Fatalf("manifest 未解析: %+v", report.Manifest)
	}
	if report.HasFail() {
		for _, c := range report.Checks {
			if c.Status == dryRunFail {
				t.Errorf("检查 %s 意外 fail: %s", c.Name, c.Detail)
			}
		}
		t.Fatal("golden 不应存在 fail 项")
	}
	byName := dryRunChecksByName(t, report)
	for _, name := range []string{"compile", "hooks", "sample-build"} {
		c, ok := byName[name]
		if !ok {
			t.Fatalf("缺少检查项 %s", name)
		}
		if c.Status != dryRunPass {
			t.Errorf("检查 %s 期望 pass，实际 %s: %s", name, c.Status, c.Detail)
		}
	}
	dec := byName["sample-decode"]
	if dec.Status != dryRunWarn || !strings.Contains(dec.Detail, "invalid_action") {
		t.Errorf("sample-decode 期望 warn(invalid_action)，实际 %s: %s", dec.Status, dec.Detail)
	}
	if !strings.Contains(byName["sample-build"].Detail, "相对路径") {
		t.Errorf("sample-build 应说明相对路径拼接渠道 baseUrl: %s", byName["sample-build"].Detail)
	}
}

// TestDryRunCompileFail 编译失败：唯一一条 fail，无 manifest。
func TestDryRunCompileFail(t *testing.T) {
	report := DryRun("export const broken = {{{")
	if report.Manifest != nil {
		t.Errorf("编译失败不应解析出 manifest: %+v", report.Manifest)
	}
	if len(report.Checks) != 1 {
		t.Fatalf("期望仅 1 条检查项，实际 %d: %+v", len(report.Checks), report.Checks)
	}
	if report.Checks[0].Name != "compile" || report.Checks[0].Status != dryRunFail {
		t.Errorf("期望 compile fail，实际 %+v", report.Checks[0])
	}
	if !report.HasFail() {
		t.Error("编译失败应 HasFail")
	}
}

// TestDryRunMissingHooks 缺提交链/轮询链钩子 → hooks fail；decode 钩子在位时
// 样例执行仍进行（单项失败不阻断后续可执行的检查）。
func TestDryRunMissingHooks(t *testing.T) {
	report := DryRun(`export const meta = {
		apiVersion: 1, key: "dryrun-bad", name: "DryRun Bad", version: "1.0.0",
		models: ["dryrun_bad_model"],
		routes: [{ method: "POST", path: "/submit", type: "submit" }],
	};
	export function decodeRequest(ctx) {
		return { kind: "submit", model: "dryrun_bad_model", requestBody: ctx.body || {} };
	}
	`)
	byName := dryRunChecksByName(t, report)
	c := byName["hooks"]
	if c.Status != dryRunFail {
		t.Fatalf("hooks 期望 fail，实际 %s: %s", c.Status, c.Detail)
	}
	for _, hook := range []string{"buildSubmitRequest", "parseSubmitResponse", "buildQueryRequest", "parseQueryResponse"} {
		if !strings.Contains(c.Detail, hook) {
			t.Errorf("hooks detail 应包含缺失钩子 %s: %s", hook, c.Detail)
		}
	}
	if byName["sample-decode"].Status != dryRunPass {
		t.Errorf("sample-decode 期望 pass，实际 %s: %s",
			byName["sample-decode"].Status, byName["sample-decode"].Detail)
	}
	if _, has := byName["sample-build"]; has {
		t.Error("buildSubmitRequest 缺失时不应执行 sample-build")
	}
	if !report.HasFail() {
		t.Error("缺必选钩子应 HasFail")
	}
}

// TestDryRunFetchRelativeURL utils.fetch 相对 URL 在运行期合法（按渠道 baseUrl 解析），
// 但试运行不注入渠道上下文 → fetch 被拒（机制上杜绝试运行发真实 HTTP），build 按
// 样例被拒处理为 warn；相对路径字面量不再有 lint 检查项。
func TestDryRunFetchRelativeURL(t *testing.T) {
	src := strings.Replace(dryRunFixturePlugin("dryrun-fetch", "dryrun_fetch_model"),
		`return { url: "/v1/create", method: "POST", body: ctx.body || {} };`,
		`utils.fetch({ url: "/v1/relative" });
	return { url: "/v1/create", method: "POST", body: ctx.body || {} };`, 1)
	report := DryRun(src)
	byName := dryRunChecksByName(t, report)
	if _, ok := byName["lint-fetch-url"]; ok {
		t.Error("lint-fetch-url 检查项应已移除（utils.fetch 相对 URL 已合法）")
	}
	if byName["sample-build"].Status != dryRunWarn {
		t.Errorf("sample-build 期望 warn（fetch 被拒），实际 %s: %s",
			byName["sample-build"].Status, byName["sample-build"].Detail)
	}
	if report.HasFail() {
		t.Error("fetch 拒绝不应构成 fail")
	}
}

// TestDryRunSharedModelNoConflict 与在服务插件同名上游模型为合法形态：无冲突检查项、无 fail。
func TestDryRunSharedModelNoConflict(t *testing.T) {
	cleanup, err := PublishForTest(dryRunFixturePlugin("dryrun-other", "dryrun_shared_model"))
	if err != nil {
		t.Fatalf("发布插件失败: %v", err)
	}
	defer cleanup()

	report := DryRun(dryRunFixturePlugin("dryrun-challenger", "dryrun_shared_model"))
	if _, ok := dryRunChecksByName(t, report)["conflict"]; ok {
		t.Error("跨插件同名模型不再做冲突检查")
	}
	if report.HasFail() {
		t.Error("同名上游模型不应构成 fail")
	}
}

// TestDryRunDecodeInvalidContract decode 返回未过契约校验（空模型名）→ fail；
// 平台模型名不在 meta.models（上游清单）属合法形态，不再拒绝。
func TestDryRunDecodeInvalidContract(t *testing.T) {
	src := strings.Replace(dryRunFixturePlugin("dryrun-undeclared", "dryrun_undeclared_model"),
		`return { kind: "submit", model: "dryrun_undeclared_model", requestBody: ctx.body || {} };`,
		`return { kind: "submit", model: "", requestBody: ctx.body || {} };`, 1)
	c := dryRunChecksByName(t, DryRun(src))["sample-decode"]
	if c.Status != dryRunFail || !strings.Contains(c.Detail, "model") {
		t.Errorf("sample-decode 期望 fail(空模型名)，实际 %s: %s", c.Status, c.Detail)
	}

	alias := strings.Replace(dryRunFixturePlugin("dryrun-alias", "dryrun_alias_model"),
		`return { kind: "submit", model: "dryrun_alias_model", requestBody: ctx.body || {} };`,
		`return { kind: "submit", model: "platform_alias", requestBody: ctx.body || {} };`, 1)
	if c := dryRunChecksByName(t, DryRun(alias))["sample-decode"]; c.Status != dryRunPass {
		t.Errorf("平台别名模型应放行，实际 %s: %s", c.Status, c.Detail)
	}
}

// TestDryRunValidateRequest validateRequest 样例执行：返回契约违规 → fail；合法 → pass。
func TestDryRunValidateRequest(t *testing.T) {
	base := dryRunFixturePlugin("dryrun-validate", "dryrun_validate_model")
	ok := base + "\nexport function validateRequest(ctx) { return { requestBody: { fixed: true } }; }\n"
	if c := dryRunChecksByName(t, DryRun(ok))["sample-validate"]; c.Status != dryRunPass {
		t.Errorf("合法 validateRequest 期望 pass，实际 %s: %s", c.Status, c.Detail)
	}
	bad := base + "\nexport function validateRequest(ctx) { return { body: {} }; }\n"
	if c := dryRunChecksByName(t, DryRun(bad))["sample-validate"]; c.Status != dryRunFail {
		t.Errorf("未知返回字段期望 fail，实际 %s: %s", c.Status, c.Detail)
	}
}

// TestTemplateSource 模板即 golden 参考实现：非空、可编译、key 正确，
// 且自身能通过试运行（作者抄到的第一份代码必须是能过自检的）。
func TestTemplateSource(t *testing.T) {
	src := TemplateSource()
	if src == "" {
		t.Fatal("模板源码为空")
	}
	compiled, err := NewGojaEngine().Compile(src)
	if err != nil {
		t.Fatalf("模板不可编译: %v", err)
	}
	if compiled.Key != "golden" {
		t.Errorf("模板 manifest.key 期望 golden，实际 %s", compiled.Key)
	}
	if DryRun(src).HasFail() {
		t.Error("模板 dry-run 不应存在 fail 项")
	}
}

// TestDryRunStandardEndpoint 标准视频端点能力检查三档：
//   - 声明 + 导出钩子 → pass（golden 形态）
//   - 声明但未导出钩子 → fail（标准端点提交必失败）
//   - 未声明但声明了 referenceRoles → warn（参考素材角色只对标准端点生效，疑似漏声明）
//   - 未声明且无 referenceRoles → pass（与视频无关的插件零打扰）
func TestDryRunStandardEndpoint(t *testing.T) {
	// 声明 + 导出：在夹具上补齐 standardVideos 声明与钩子
	declared := strings.Replace(dryRunFixturePlugin("dryrun-sv-ok", "dryrun_sv_ok_model"),
		"routes: [{ method: \"POST\", path: \"/create\", type: \"submit\" }],",
		"routes: [{ method: \"POST\", path: \"/create\", type: \"submit\" }],\n\tstandardVideos: \"decodeVideos\",", 1) +
		"\nexport function decodeVideos(ctx) {\n" +
		"\treturn { kind: \"submit\", model: \"dryrun_sv_ok_model\", requestBody: ctx.body || {} };\n}\n"
	byName := dryRunChecksByName(t, DryRun(declared))
	if c := byName["standard-endpoint"]; c.Status != dryRunPass {
		t.Errorf("声明+导出期望 pass，实际 %s: %s", c.Status, c.Detail)
	}

	// 声明但未导出钩子 → fail
	missingHook := strings.Replace(dryRunFixturePlugin("dryrun-sv-miss", "dryrun_sv_miss_model"),
		"routes: [{ method: \"POST\", path: \"/create\", type: \"submit\" }],",
		"routes: [{ method: \"POST\", path: \"/create\", type: \"submit\" }],\n\tstandardVideos: \"decodeVideos\",", 1)
	byName = dryRunChecksByName(t, DryRun(missingHook))
	c := byName["standard-endpoint"]
	if c.Status != dryRunFail || !strings.Contains(c.Detail, "未导出") {
		t.Errorf("声明缺钩子期望 fail，实际 %s: %s", c.Status, c.Detail)
	}

	// 未声明 standardVideos 但声明 referenceRoles → warn
	seconds := strings.Replace(dryRunFixturePlugin("dryrun-sv-warn", "dryrun_sv_warn_model"),
		"routes: [{ method: \"POST\", path: \"/create\", type: \"submit\" }],",
		"routes: [{ method: \"POST\", path: \"/create\", type: \"submit\" }],\n\treferenceRoles: [\"first_frame\"],", 1)
	byName = dryRunChecksByName(t, DryRun(seconds))
	c = byName["standard-endpoint"]
	if c.Status != dryRunWarn || !strings.Contains(c.Detail, "standardVideos") {
		t.Errorf("疑似视频插件未声明期望 warn，实际 %s: %s", c.Status, c.Detail)
	}

	// 未声明且无按秒字段 → pass（夹具默认形态）
	byName = dryRunChecksByName(t, DryRun(dryRunFixturePlugin("dryrun-sv-none", "dryrun_sv_none_model")))
	if c := byName["standard-endpoint"]; c.Status != dryRunPass {
		t.Errorf("非视频插件期望 pass，实际 %s: %s", c.Status, c.Detail)
	}
}
