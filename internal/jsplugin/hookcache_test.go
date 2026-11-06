package jsplugin

import (
	"testing"
)

// TestHookCacheCoversAliasesAndStandardHooks 缓存必须覆盖路由别名（decode/render
// 指名）与标准钩子名——miss 的名字会回落 Pool.HasHook，池全忙时退化为每次新建
// runtime，缓存优化对别名插件失效。
func TestHookCacheCoversAliasesAndStandardHooks(t *testing.T) {
	src := `export const meta = {
  apiVersion: 1, key: "aliashk", name: "Alias", version: "1.0.0",
  models: ["alias_m"],
  routes: [{ method: "POST", path: "/s", type: "submit", decode: "decodeSubmit", render: "renderSubmitNamed" }],
};
export function decodeSubmit() { return { kind: "submit", model: "alias_m", requestBody: {} }; }
export function renderSubmitNamed() { return { status: 200, body: { ok: 1 } }; }
export function buildSubmitRequest() { return { url: "/v1/s", method: "POST" }; }
export function parseSubmitResponse() { return { taskId: "t1" }; }
`
	cleanup, err := PublishForTest(src)
	if err != nil {
		t.Fatalf("发布插件失败: %v", err)
	}
	t.Cleanup(cleanup)

	plugin, ok := CurrentGeneration().Plugin("aliashk")
	if !ok {
		t.Fatal("插件未进 generation")
	}
	if plugin.hookCache == nil {
		t.Fatal("hookCache 未构建")
	}
	for _, name := range []string{"decodeSubmit", "renderSubmitNamed", "buildSubmitRequest", "parseSubmitResponse"} {
		if !plugin.HasHook(name) {
			t.Errorf("钩子 %s 应命中缓存为已导出", name)
		}
	}
	if plugin.HasHook("estimateUsage") {
		t.Error("未导出的 estimateUsage 应为 false")
	}
	// 缓存命中路径不得触碰池：live 实例数保持 0（探测实例已关闭）。
	// 若 miss 回落 Pool.HasHook，会新建实例使 live 变 1。
	if got := plugin.Pool.LiveInstances(); got != 0 {
		t.Fatalf("缓存命中不应新建实例，live=%d", got)
	}
}
