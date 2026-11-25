package jsplugin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
)

// errorProbeSource 错误文案探针插件：atMessage 模拟插件自身的参数校验提示
// （含 " at "），refError 触发 goja 运行期 ReferenceError（消息带位置后缀）。
const errorProbeSource = `
export const meta = {
  apiVersion: 1, key: "msgprobe", name: "Msg Probe", version: "1.0.0",
  models: ["msgprobe_model"],
  routes: [{ method: "POST", path: "/submit", type: "submit" }],
};
export function decodeRequest() { return { kind: "submit", model: "msgprobe_model", requestBody: {} }; }
export function buildSubmitRequest() { return { url: "/submit" }; }
export function parseSubmitResponse() { return { taskId: "t1" }; }
export function buildQueryRequest(ctx) { return { url: "/q" }; }
export function parseQueryResponse(ctx) { return { items: [{ taskId: ctx.tasks[0].taskId, done: false }] }; }
export function atMessage() { throw new Error("duration must be at least 5 seconds"); }
export function atMessageTwice() { throw new Error("meet at 9 at the latest"); }
export function refError() { return missingIdentifier.x; }
`

// TestSanitizeJSMessage 单元：只剥末尾的 goja 位置后缀，消息正文里的 " at " 保留。
func TestSanitizeJSMessage(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Error: duration must be at least 5 seconds", "Error: duration must be at least 5 seconds"},
		{"Error: supports at most 10 images", "Error: supports at most 10 images"},
		{"Error: meet at 9 at the latest", "Error: meet at 9 at the latest"},
		{"ReferenceError: missingIdentifier is not defined at refError (<eval>:1:18(3))", "ReferenceError: missingIdentifier is not defined"},
		{"TypeError: Cannot read property 'x' of undefined at <eval>:1:1(0)", "TypeError: Cannot read property 'x' of undefined"},
		{"Error: bad input at validate (<eval>:12:5(7))", "Error: bad input"},
		// 不符合位置后缀形态的结尾原样保留（宁可不剥，不可多剥）
		{"Error: meet at 9", "Error: meet at 9"},
		{"Error: index at 3:4", "Error: index at 3:4"},
	}
	for _, c := range cases {
		if got := sanitizeJSMessage(c.in); got != c.want {
			t.Errorf("sanitizeJSMessage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestHookErrorMessageIntact 经真实引擎调用：插件 throw 的文案完整透出，
// goja 运行期错误只去掉末尾位置后缀。
func TestHookErrorMessageIntact(t *testing.T) {
	// 先看 goja 原始消息形态，确保断言对着真实格式（而非对正则的自我印证）
	rt := goja.New()
	if _, err := rt.RunString(`function boom() { return missingIdentifier.x; }`); err != nil {
		t.Fatalf("goja 脚本加载失败: %v", err)
	}
	rawFn, ok := goja.AssertFunction(rt.Get("boom"))
	if !ok {
		t.Fatal("goja 函数断言失败: boom 不是函数")
	}
	_, rawErr := rawFn(goja.Undefined())
	if rawErr == nil {
		t.Fatal("期望 ReferenceError")
	}
	t.Logf("goja 原始消息: %q", rawErr.Error())
	if !strings.Contains(rawErr.Error(), " (<eval>:") {
		t.Fatalf("goja 消息形态与预期不符（无位置后缀）: %q", rawErr.Error())
	}

	inst, err := NewGojaEngine().NewInstance(mustCompile(t, errorProbeSource), InstanceOptions{CallTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("创建实例失败: %v", err)
	}
	t.Cleanup(inst.Close)
	call := func(hook string) error {
		_, err := inst.Call(context.Background(), hook, json.RawMessage(`{}`))
		return err
	}

	if err := call("atMessage"); err == nil || !strings.HasSuffix(err.Error(), "Error: duration must be at least 5 seconds") {
		t.Fatalf("含 \" at \" 的插件文案应完整透出: %v", err)
	}
	if err := call("atMessageTwice"); err == nil || !strings.HasSuffix(err.Error(), "Error: meet at 9 at the latest") {
		t.Fatalf("多个 \" at \" 的插件文案应完整透出: %v", err)
	}
	err = call("refError")
	if err == nil || !strings.Contains(err.Error(), "ReferenceError") || strings.Contains(err.Error(), "(<eval>:") {
		t.Fatalf("运行期错误应保留类型与说明、只剥位置后缀: %v", err)
	}
}
