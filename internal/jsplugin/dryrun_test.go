package jsplugin

import (
	"os"
	"strings"
	"testing"
)

// dryRunGoldenSource 读取 golden 参考实现源码（与 jsbridge adaptor_test 同一夹具）。
func dryRunGoldenSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("builtin/golden/plugin.js")
	if err != nil {
		t.Fatalf("读取 golden 插件失败: %v", err)
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

// TestDryRunGolden golden 参考实现：核心检查项全部 pass 且无任何 fail。
// 样例 decode 为 warn——golden 校验 action ∈ {MUSIC, LYRICS}，固定样例参数被拒属
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
	for _, name := range []string{"compile", "hooks", "conflict", "sample-build"} {
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

// TestDryRunConflict 与在服务插件的模型冲突 → conflict fail（激活会被拒，提前告知）。
func TestDryRunConflict(t *testing.T) {
	cleanup, err := PublishForTest(dryRunFixturePlugin("dryrun-other", "dryrun_conflict_model"))
	if err != nil {
		t.Fatalf("发布冲突插件失败: %v", err)
	}
	defer cleanup()

	report := DryRun(dryRunFixturePlugin("dryrun-challenger", "dryrun_conflict_model"))
	byName := dryRunChecksByName(t, report)
	c := byName["conflict"]
	if c.Status != dryRunFail {
		t.Fatalf("conflict 期望 fail，实际 %s: %s", c.Status, c.Detail)
	}
	if !strings.Contains(c.Detail, "dryrun-other") {
		t.Errorf("conflict detail 应指明冲突方 dryrun-other: %s", c.Detail)
	}
	if !report.HasFail() {
		t.Error("冲突应 HasFail")
	}
}

// TestDryRunDecodeInvalidContract decode 返回未过契约校验（模型未声明）→ fail。
func TestDryRunDecodeInvalidContract(t *testing.T) {
	src := strings.Replace(dryRunFixturePlugin("dryrun-undeclared", "dryrun_undeclared_model"),
		`return { kind: "submit", model: "dryrun_undeclared_model", requestBody: ctx.body || {} };`,
		`return { kind: "submit", model: "undeclared_model", requestBody: ctx.body || {} };`, 1)
	report := DryRun(src)
	byName := dryRunChecksByName(t, report)
	c := byName["sample-decode"]
	if c.Status != dryRunFail || !strings.Contains(c.Detail, "未在 meta.models") {
		t.Errorf("sample-decode 期望 fail(模型未声明)，实际 %s: %s", c.Status, c.Detail)
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
