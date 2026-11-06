package jsplugin

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

func goldenSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("builtin/golden/plugin.js")
	if err != nil {
		t.Fatalf("读取 golden 插件失败: %v", err)
	}
	return string(data)
}

func mustCompile(t *testing.T, source string) *CompiledPlugin {
	t.Helper()
	compiled, err := NewGojaEngine().Compile(source)
	if err != nil {
		t.Fatalf("编译失败: %v", err)
	}
	return compiled
}

func TestCompileGoldenManifest(t *testing.T) {
	compiled := mustCompile(t, goldenSource(t))
	m := compiled.Manifest
	if m.Key != "golden" || m.Version == "" {
		t.Fatalf("manifest 解析异常: %+v", m)
	}
	if len(m.Models) != 1 || m.Models[0] != "golden_music" {
		t.Fatalf("models 解析异常: %v", m.Models)
	}
	if len(m.Routes) != 2 || m.Routes[0].Type != contract.RouteTypeSubmit || m.Routes[1].Type != contract.RouteTypeQuery {
		t.Fatalf("routes 解析异常: %+v", m.Routes)
	}
	if m.EffectiveFetchMode() != "per_task" {
		t.Fatalf("fetchMode 异常: %s", m.EffectiveFetchMode())
	}
}

func TestCompileRejectsImport(t *testing.T) {
	src := `import fs from "fs";` + "\n" + `export const meta = { apiVersion: 1, key: "x-y", name: "X", version: "1.0.0", models: ["m"], routes: [{method:"POST",path:"/s",type:"submit"}] };`
	if _, err := NewGojaEngine().Compile(src); err == nil {
		t.Fatal("带 import 的插件应当编译失败（v1 仅接受零依赖单文件）")
	}
}

func TestCompileRejectsBadManifest(t *testing.T) {
	src := `export const meta = { apiVersion: 1, key: "BAD KEY", name: "X", version: "1.0.0", models: ["m"], routes: [] };`
	if _, err := NewGojaEngine().Compile(src); err == nil {
		t.Fatal("非法 key/空 routes 应当校验失败")
	}
}

func callHook(t *testing.T, inst Instance, hook string, arg any) json.RawMessage {
	t.Helper()
	args, err := json.Marshal(arg)
	if err != nil {
		t.Fatalf("入参序列化失败: %v", err)
	}
	out, err := inst.Call(context.Background(), hook, args)
	if err != nil {
		t.Fatalf("钩子 %s 调用失败: %v", hook, err)
	}
	return out
}

func TestGoldenHookFlow(t *testing.T) {
	compiled := mustCompile(t, goldenSource(t))
	engine := NewGojaEngine()
	inst, err := engine.NewInstance(compiled, InstanceOptions{CallTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("创建实例失败: %v", err)
	}
	defer inst.Close()

	// decodeRequest
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(callHook(t, inst, "decodeRequest", contract.DecodeContext{
		Path: "/submit/music", Method: "POST",
		Params: map[string]string{"action": "music"},
		Body:   json.RawMessage(`{"prompt":"hello"}`),
	}), &decoded); err != nil {
		t.Fatalf("decodeRequest 返回非对象: %v", err)
	}
	res, err := contract.ValidateDecoded(decoded, compiled.Manifest, false)
	if err != nil {
		t.Fatalf("decodeRequest 契约校验失败: %v", err)
	}
	if res.Kind != "submit" || res.Model != "golden_music" {
		t.Fatalf("decodeRequest 结果异常: %+v", res)
	}

	// buildSubmitRequest
	var builtRaw map[string]json.RawMessage
	if err := json.Unmarshal(callHook(t, inst, "buildSubmitRequest", contract.BuildSubmitContext{
		Body: res.RequestBody, Model: res.Model, ChannelBaseURL: "https://up.example.com",
	}), &builtRaw); err != nil {
		t.Fatalf("buildSubmitRequest 返回非对象: %v", err)
	}
	built, err := contract.ValidateBuiltRequest(builtRaw)
	if err != nil {
		t.Fatalf("buildSubmitRequest 契约校验失败: %v", err)
	}
	if built.Method != "POST" || !strings.HasPrefix(built.URL, "/v1/golden/submit") {
		t.Fatalf("buildSubmitRequest 结果异常: %+v", built)
	}

	// parseSubmitResponse
	var subRaw map[string]json.RawMessage
	_ = json.Unmarshal(callHook(t, inst, "parseSubmitResponse", contract.ParseSubmitContext{
		Status: 200, ResponseBody: json.RawMessage(`{"code":"success","data":"task-abc"}`),
	}), &subRaw)
	sub, err := contract.ValidateSubmitted(subRaw)
	if err != nil {
		t.Fatalf("parseSubmitResponse 契约校验失败: %v", err)
	}
	if sub.TaskID != "task-abc" {
		t.Fatalf("taskId 异常: %s", sub.TaskID)
	}

	// parseQueryResponse（processing → 未终态；state.polls 递增）
	taskRef := contract.QueryTaskRef{TaskID: "task-abc", TaskData: sub.TaskData, State: sub.State}
	var qRaw map[string]json.RawMessage
	_ = json.Unmarshal(callHook(t, inst, "parseQueryResponse", contract.ParseQueryContext{
		Status: 200, Tasks: []contract.QueryTaskRef{taskRef},
		ResponseBody: json.RawMessage(`{"status":"processing"}`),
	}), &qRaw)
	queried, err := contract.ValidateQueried(qRaw, []contract.QueryTaskRef{taskRef})
	if err != nil {
		t.Fatalf("parseQueryResponse 契约校验失败: %v", err)
	}
	if queried.Items[0].Done {
		t.Fatal("processing 状态不应终态")
	}

	// parseQueryResponse（success → done + taskData）
	_ = json.Unmarshal(callHook(t, inst, "parseQueryResponse", contract.ParseQueryContext{
		Status:       200,
		Tasks:        []contract.QueryTaskRef{{TaskID: "task-abc", State: queried.Items[0].State}},
		ResponseBody: json.RawMessage(`{"status":"success","data":{"audio_url":"https://oss/x.mp3"}}`),
	}), &qRaw)
	queried, err = contract.ValidateQueried(qRaw, []contract.QueryTaskRef{{TaskID: "task-abc"}})
	if err != nil {
		t.Fatalf("parseQueryResponse(success) 契约校验失败: %v", err)
	}
	if !queried.Items[0].Done {
		t.Fatal("success 状态应当终态")
	}

	// renderTask
	var rendRaw map[string]json.RawMessage
	_ = json.Unmarshal(callHook(t, inst, "renderTask", contract.RenderTaskContext{
		Task: contract.RenderTask{
			PublicTaskID: "task_pub1", TaskID: "task-abc", Model: "golden_music",
			Status: "SUCCESS", Data: queried.Items[0].TaskData,
		},
	}), &rendRaw)
	rend, err := contract.ValidateRendered(rendRaw)
	if err != nil {
		t.Fatalf("renderTask 契约校验失败: %v", err)
	}
	if !strings.Contains(string(rend.Body), "task_pub1") {
		t.Fatalf("renderTask 输出异常: %s", rend.Body)
	}
}

func TestCallTimeoutInterrupts(t *testing.T) {
	src := `export const meta = { apiVersion: 1, key: "spin-me", name: "S", version: "1.0.0", models: ["m"], routes: [{method:"POST",path:"/s",type:"submit"}] };
export function spin() { while (true) {} }`
	compiled := mustCompile(t, src)
	inst, err := NewGojaEngine().NewInstance(compiled, InstanceOptions{CallTimeout: 300 * time.Millisecond})
	if err != nil {
		t.Fatalf("创建实例失败: %v", err)
	}
	defer inst.Close()

	started := time.Now()
	_, err = inst.Call(context.Background(), "spin", json.RawMessage("null"))
	if err == nil {
		t.Fatal("死循环钩子应当超时")
	}
	if time.Since(started) > 3*time.Second {
		t.Fatalf("中断耗时异常: %s", time.Since(started))
	}
	var hookErr *HookError
	if !asHookError(err, &hookErr) {
		t.Fatalf("应当返回 HookError，得到: %v", err)
	}
}

func asHookError(err error, target **HookError) bool {
	he, ok := err.(*HookError)
	if ok {
		*target = he
	}
	return ok
}

func TestHookThrowBecomesHookError(t *testing.T) {
	src := `export const meta = { apiVersion: 1, key: "thrower", name: "T", version: "1.0.0", models: ["m"], routes: [{method:"POST",path:"/s",type:"submit"}] };
export function boom() { throw new Error("invalid_action"); }`
	compiled := mustCompile(t, src)
	inst, _ := NewGojaEngine().NewInstance(compiled, InstanceOptions{})
	defer inst.Close()

	_, err := inst.Call(context.Background(), "boom", json.RawMessage("null"))
	if err == nil {
		t.Fatal("应当返回错误")
	}
	hookErr, ok := err.(*HookError)
	if !ok {
		t.Fatalf("应当返回 HookError，得到: %T %v", err, err)
	}
	if !strings.Contains(hookErr.Message, "invalid_action") {
		t.Fatalf("错误消息异常: %s", hookErr.Message)
	}
}

func TestPoolConcurrencyAndReuse(t *testing.T) {
	compiled := mustCompile(t, goldenSource(t))
	pool := NewPluginPool(NewGojaEngine(), compiled, InstanceOptions{Concurrency: 4})
	defer pool.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := pool.Call(context.Background(), "decodeRequest", mustJSON(t, contract.DecodeContext{
				Path: "/submit/music", Params: map[string]string{"action": "music"},
			}))
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("并发调用失败: %v", err)
	}
	if pool.LiveInstances() > 4 {
		t.Fatalf("存活实例 %d 超过并发闸 4", pool.LiveInstances())
	}
}

func TestUnknownTopLevelFieldRejected(t *testing.T) {
	raw := map[string]json.RawMessage{
		"taskId":   json.RawMessage(`"t1"`),
		"surprise": json.RawMessage(`1`),
	}
	if _, err := contract.ValidateSubmitted(raw); err == nil {
		t.Fatal("未知顶层字段应当被拒绝")
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	return data
}
