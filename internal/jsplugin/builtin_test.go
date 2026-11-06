package jsplugin

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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
	if _, ok := keys["golden"]; !ok {
		t.Fatal("缺少内置插件 golden")
	}
	if _, ok := keys["sora-js"]; !ok {
		t.Fatal("缺少内置插件 sora-js")
	}
	// sora-js 形态断言：模型/路由/计费单位/产物路径约定
	sora := keys["sora-js"]
	if !sora.Manifest.HasModel("sora-2-js") {
		t.Fatalf("sora-js 模型异常: %v", sora.Manifest.Models)
	}
	if len(sora.Manifest.Routes) != 2 {
		t.Fatalf("sora-js 路由数异常: %+v", sora.Manifest.Routes)
	}
	foundSecond := false
	for _, f := range sora.Manifest.UsageSchema {
		if f.Key == "seconds" && f.Unit == "second" {
			foundSecond = true
		}
	}
	if !foundSecond {
		t.Fatalf("sora-js usageSchema 缺少 seconds(second) 计费单位: %+v", sora.Manifest.UsageSchema)
	}
}

// TestSoraJSHookFlow sora-js 全钩子冒烟（含 utils.config.get 读取与时长钳制）。
func TestSoraJSHookFlow(t *testing.T) {
	builtins, err := compileBuiltins(NewGojaEngine())
	if err != nil {
		t.Fatalf("内置插件编译失败: %v", err)
	}
	var sora *CompiledPlugin
	for _, b := range builtins {
		if b.Key == "sora-js" {
			sora = b
		}
	}
	if sora == nil {
		t.Fatal("sora-js 不在内置列表")
	}

	// 配置注入（模拟管理端配置后的实例装载）
	inst, err := NewGojaEngine().NewInstance(sora, InstanceOptions{
		CallTimeout: 2 * time.Second,
		ConfigValues: map[string]any{
			"default_size": "720x1280",
			"max_seconds":  6,
			"vendor_note":  "demo",
		},
	})
	if err != nil {
		t.Fatalf("创建实例失败: %v", err)
	}
	defer inst.Close()
	_ = context.Background()

	call := func(hook string, arg any) map[string]json.RawMessage {
		t.Helper()
		args, _ := json.Marshal(arg)
		out, err := inst.Call(context.Background(), hook, args)
		if err != nil {
			t.Fatalf("钩子 %s 失败: %v", hook, err)
		}
		raw, err := contract.ParseJSONObject(out)
		if err != nil {
			t.Fatalf("钩子 %s 返回非对象: %v", hook, err)
		}
		return raw
	}

	// decode：缺省模型回落 sora-2-js
	decoded, err := contract.ValidateDecoded(call("decodeRequest", contract.DecodeContext{
		Path: "/videos", Method: "POST", Body: json.RawMessage(`{"prompt":"a cat","seconds":99}`),
	}), sora.Manifest, false)
	if err != nil {
		t.Fatalf("decodeRequest 校验失败: %v", err)
	}
	if decoded.Model != "sora-2-js" {
		t.Fatalf("模型回落异常: %s", decoded.Model)
	}

	// estimateUsage：时长钳制到配置上限 6
	estRaw := call("estimateUsage", contract.EstimateUsageContext{Body: decoded.RequestBody, Model: decoded.Model})
	estFacts, err := contract.ValidateUsageFacts(estRaw, sora.Manifest)
	if err != nil {
		t.Fatalf("estimateUsage 校验失败: %v", err)
	}
	if estFacts["seconds"] != 6 {
		t.Fatalf("时长钳制异常（期望 6）: %v", estFacts)
	}

	// buildSubmitRequest：尺寸走配置默认
	built, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: decoded.RequestBody, Model: decoded.Model, ChannelBaseURL: "https://up.example.com",
	}))
	if err != nil {
		t.Fatalf("buildSubmitRequest 校验失败: %v", err)
	}
	var submitBody map[string]any
	_ = json.Unmarshal(built.Body, &submitBody)
	if submitBody["size"] != "720x1280" || submitBody["note"] != "demo" {
		t.Fatalf("配置读取未生效: %v", submitBody)
	}

	// parseSubmitResponse
	sub, err := contract.ValidateSubmitted(call("parseSubmitResponse", contract.ParseSubmitContext{
		Status: 200, ResponseBody: json.RawMessage(`{"id":"video_abc","status":"queued"}`),
	}))
	if err != nil || sub.TaskID != "video_abc" {
		t.Fatalf("parseSubmitResponse 异常: %+v err=%v", sub, err)
	}

	// parseQueryResponse（completed）：usage + artifacts
	taskRef := contract.QueryTaskRef{TaskID: "video_abc", TaskData: sub.TaskData, State: sub.State}
	q, err := contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Status: 200, Tasks: []contract.QueryTaskRef{taskRef},
		ResponseBody: json.RawMessage(`{"id":"video_abc","status":"completed","progress":100,"seconds":6,"video":{"url":"https://up.example.com/v.mp4","size_mb":12}}`),
	}), []contract.QueryTaskRef{taskRef})
	if err != nil {
		t.Fatalf("parseQueryResponse 校验失败: %v", err)
	}
	item := q.Items[0]
	if !item.Done || len(item.Artifacts) != 1 || item.Artifacts[0].Source.Path != "video.url" {
		t.Fatalf("终态 item 异常: %+v", item)
	}
	usageFacts, err := contract.ValidateUsageFacts(mustRawMap(t, item.Usage), sora.Manifest)
	if err != nil || usageFacts["seconds"] != 6 {
		t.Fatalf("终态 usage 异常: %v err=%v", usageFacts, err)
	}

	// renderTask：转存前的上游 URL 透出
	rend, err := contract.ValidateRendered(call("renderTask", contract.RenderTaskContext{
		Task: contract.RenderTask{
			PublicTaskID: "task_pub1", TaskID: "video_abc", Model: "sora-2-js",
			Status: "SUCCESS", Data: item.TaskData,
		},
	}))
	if err != nil {
		t.Fatalf("renderTask 校验失败: %v", err)
	}
	var body map[string]any
	_ = json.Unmarshal(rend.Body, &body)
	if body["video_url"] != "https://up.example.com/v.mp4" || body["status"] != "completed" {
		t.Fatalf("renderTask 输出异常: %v", body)
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
