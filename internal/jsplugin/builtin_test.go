package jsplugin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// TestBuiltinPluginsCompile 内置插件（go:embed）全部可编译且 manifest 合法——
// 这是发布质量门：内置插件编译失败不许随二进制出门。
func TestBuiltinPluginsCompile(t *testing.T) {
	builtins, err := compileBuiltins(NewGojaEngine())
	if err != nil {
		t.Fatalf("内置插件编译失败: %v", err)
	}
	keys := map[string]*CompiledPlugin{}
	for _, b := range builtins {
		keys[b.Key] = b
	}
	for _, key := range []string{"alibaba", "doubao", "hailuo", "teamapi"} {
		if _, ok := keys[key]; !ok {
			t.Fatalf("缺少内置插件 %s", key)
		}
	}
	// golden 已改为纯作者模板（template/plugin.js），不再作为内置实例 seed
	if _, ok := keys["golden"]; ok {
		t.Fatal("golden 已下线，不应再作为内置插件 seed")
	}
}

// TestRetiredBuiltinKeys 下线清理纯逻辑：仅命中「DB 有 builtin 行、二进制内已不存在」
// 的 key；在线内置、upload 行、纯上传插件均不触发。
func TestRetiredBuiltinKeys(t *testing.T) {
	live := map[string]struct{}{"hailuo": {}, "teamapi": {}}
	rows := []PluginVersionRow{
		{PluginKey: "hailuo", Origin: "builtin"},
		{PluginKey: "golden", Origin: "builtin"},
		{PluginKey: "golden", Origin: "builtin"},  // 同 key 多行去重
		{PluginKey: "golden", Origin: "upload"},   // upload 行不参与判定
		{PluginKey: "reseller", Origin: "upload"}, // 纯上传插件不触发
	}
	got := retiredBuiltinKeys(rows, live)
	if len(got) != 1 || got[0] != "golden" {
		t.Fatalf("应仅清理已下线的 golden: %v", got)
	}
	if got := retiredBuiltinKeys(nil, live); got != nil {
		t.Fatalf("无 builtin 行时应返回空: %v", got)
	}
}

// TestBuiltinPluginsShareModelsAllowed 跨插件同名上游模型为合法形态：内置插件与声明
// 同名模型的上传插件可同时发布进 generation。
func TestBuiltinPluginsShareModelsAllowed(t *testing.T) {
	other := `export const meta = {
  apiVersion: 1, key: "reseller", name: "Reseller", version: "1.0.0",
  models: ["MiniMax-H3", "doubao-seedance-2-0-260128"],
  routes: [{ method: "POST", path: "/submit", type: "submit" }, { method: "GET", path: "/fetch/:task_id", type: "query" }],
};
export function decodeRequest(ctx) { return { kind: "submit", model: ctx.body.model, requestBody: ctx.body }; }
export function buildSubmitRequest(ctx) { return { url: "/submit", body: ctx.body }; }
export function parseSubmitResponse(ctx) { return { taskId: "t1" }; }
export function buildQueryRequest(ctx) { return { url: "/fetch" }; }
export function parseQueryResponse(ctx) { return { items: [{ taskId: ctx.tasks[0].taskId, done: false }] }; }
`
	cleanup, err := PublishForTest(readBuiltin(t, "hailuo"), readBuiltin(t, "doubao"), other)
	if err != nil {
		t.Fatalf("同名上游模型的多个插件应可同时发布: %v", err)
	}
	t.Cleanup(cleanup)
	for _, key := range []string{"hailuo", "doubao", "reseller"} {
		if _, ok := CurrentGeneration().Plugin(key); !ok {
			t.Fatalf("插件 %s 未在 generation 内", key)
		}
	}
}

func mustRawMap(t *testing.T, raw json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	m, err := contract.ParseJSONObject(raw)
	if err != nil {
		t.Fatalf("usage 非对象: %v", err)
	}
	return m
}

// mustHostUsage 解析并校验宿主固定词汇用量（键非法即失败）。
func mustHostUsage(t *testing.T, raw json.RawMessage) *contract.HostUsage {
	t.Helper()
	u, err := contract.ValidateHostUsage(mustRawMap(t, raw))
	if err != nil {
		t.Fatalf("用量不符合宿主固定词汇: %v (%s)", err, raw)
	}
	return u
}

// mustHostUsageObj 同 mustHostUsage，入参为已解析对象（钩子返回值）。
func mustHostUsageObj(t *testing.T, raw map[string]json.RawMessage) *contract.HostUsage {
	t.Helper()
	u, err := contract.ValidateHostUsage(raw)
	if err != nil {
		t.Fatalf("用量不符合宿主固定词汇: %v", err)
	}
	return u
}

// hookCaller 返回调用指定激活插件钩子的测试闭包（失败即 Fatal）。
func hookCaller(t *testing.T, plugin *ActivePlugin) func(hook string, arg any) map[string]json.RawMessage {
	return func(hook string, arg any) map[string]json.RawMessage {
		t.Helper()
		args, _ := json.Marshal(arg)
		out, err := plugin.Pool.Call(context.Background(), hook, args)
		if err != nil {
			t.Fatalf("钩子 %s 失败: %v", hook, err)
		}
		raw, err := contract.ParseJSONObject(out)
		if err != nil {
			t.Fatalf("钩子 %s 返回非对象: %v", hook, err)
		}
		return raw
	}
}

// hookCallErr 调用钩子并返回错误（断言拒绝路径用）。
func hookCallErr(plugin *ActivePlugin, hook string, arg any) error {
	args, _ := json.Marshal(arg)
	_, err := plugin.Pool.Call(context.Background(), hook, args)
	return err
}
