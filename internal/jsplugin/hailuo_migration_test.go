package jsplugin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

func hailuoSource(t *testing.T) string {
	t.Helper()
	return readBuiltin(t, "hailuo")
}

func TestMigratedHailuoManifest(t *testing.T) {
	compiled := mustCompile(t, hailuoSource(t))
	m := compiled.Manifest
	if m.Key != "hailuo" || len(m.Models) != 10 || len(m.Routes) != 3 {
		t.Fatalf("manifest 异常: models=%d routes=%d", len(m.Models), len(m.Routes))
	}
	if m.UsageFieldByKey("input_video_seconds") == nil {
		t.Fatalf("usageSchema 异常: %+v", m.UsageSchema)
	}
}

func TestMigratedHailuoV1Flow(t *testing.T) {
	cleanup, err := PublishForTest(hailuoSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("hailuo")

	call := func(hook string, arg any) map[string]json.RawMessage {
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

	decoded, err := contract.ValidateDecoded(call("createTask", contract.DecodeContext{
		Path: "/v1/video_generation", Method: "POST",
		Body: json.RawMessage(`{"model":"MiniMax-Hailuo-2.3","prompt":"ocean","duration":6,"resolution":"768P"}`),
	}), plugin.Manifest, false)
	if err != nil || decoded.Model != "MiniMax-Hailuo-2.3" {
		t.Fatalf("v1 解码失败: %+v err=%v", decoded, err)
	}

	facts, err := contract.ValidateUsageFacts(
		call("estimateUsage", contract.EstimateUsageContext{Body: decoded.RequestBody, Model: decoded.Model}),
		plugin.Manifest)
	if err != nil || facts["seconds"] != 6 {
		t.Fatalf("v1 估算异常: %v err=%v", facts, err)
	}

	built, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: decoded.RequestBody, Model: decoded.Model,
	}))
	if err != nil || built.URL != "/v1/video_generation" {
		t.Fatalf("v1 build 异常: %+v err=%v", built, err)
	}

	sub, err := contract.ValidateSubmitted(call("parseSubmitResponse", contract.ParseSubmitContext{
		Status: 200, Model: decoded.Model,
		ResponseBody: json.RawMessage(`{"base_resp":{"status_code":0},"task_id":"hl-42","status":"Queueing"}`),
	}))
	if err != nil || sub.TaskID != "hl-42" {
		t.Fatalf("v1 受理异常: %+v err=%v", sub, err)
	}

	taskRef := contract.QueryTaskRef{TaskID: "hl-42", TaskData: sub.TaskData}
	qBuilt, err := contract.ValidateBuiltRequest(call("buildQueryRequest", contract.BuildQueryContext{
		Tasks: []contract.QueryTaskRef{taskRef}, Model: decoded.Model,
	}))
	if err != nil || qBuilt.URL != "/v1/query/video_generation?task_id=hl-42" {
		t.Fatalf("v1 查询构造异常: %+v err=%v", qBuilt, err)
	}
	// v1 Processing → 进行中（file_id 无 → 无产物解析路径）
	queried, err := contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Status: 200, Tasks: []contract.QueryTaskRef{taskRef},
		ResponseBody: json.RawMessage(`{"base_resp":{"status_code":0},"task_id":"hl-42","status":"Processing"}`),
	}), []contract.QueryTaskRef{taskRef})
	if err != nil || queried.Items[0].Done {
		t.Fatalf("v1 Processing 应进行中: %+v", queried.Items[0])
	}
}

func TestMigratedHailuoH3Flow(t *testing.T) {
	cleanup, err := PublishForTest(hailuoSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("hailuo")

	call := func(hook string, arg any) map[string]json.RawMessage {
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

	decoded, err := contract.ValidateDecoded(call("createH3Task", contract.DecodeContext{
		Path: "/v2/video_generation", Method: "POST",
		Body: json.RawMessage(`{"model":"MiniMax-H3","content":[{"type":"text","text":"dance"},{"type":"image_url","url":"https://img.example.com/a.png"}],"resolution":"768P","duration":5}`),
	}), plugin.Manifest, false)
	if err != nil || decoded.Model != "MiniMax-H3" {
		t.Fatalf("H3 解码失败: %+v err=%v", decoded, err)
	}

	facts, err := contract.ValidateUsageFacts(
		call("estimateUsage", contract.EstimateUsageContext{Body: decoded.RequestBody, Model: decoded.Model}),
		plugin.Manifest)
	if err != nil || facts["seconds"] != 5 || facts["input_images"] != 1 {
		t.Fatalf("H3 估算异常: %v err=%v", facts, err)
	}

	built, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: decoded.RequestBody, Model: decoded.Model,
	}))
	if err != nil || built.URL != "/v2/video_generation" {
		t.Fatalf("H3 build 异常: %+v err=%v", built, err)
	}

	sub, err := contract.ValidateSubmitted(call("parseSubmitResponse", contract.ParseSubmitContext{
		Status: 200, Model: decoded.Model,
		ResponseBody: json.RawMessage(`{"task_id":"h3-77","status":"queued"}`),
	}))
	if err != nil || sub.TaskID != "h3-77" {
		t.Fatalf("H3 受理异常: %+v err=%v", sub, err)
	}

	taskRef := contract.QueryTaskRef{TaskID: "h3-77", TaskData: sub.TaskData}
	qBuilt, err := contract.ValidateBuiltRequest(call("buildQueryRequest", contract.BuildQueryContext{
		Tasks: []contract.QueryTaskRef{taskRef}, Model: decoded.Model,
	}))
	if err != nil || qBuilt.URL != "/v2/query/video_generation/h3-77" {
		t.Fatalf("H3 查询构造异常: %+v err=%v", qBuilt, err)
	}
	// H3 succeeded → usage 精确结算 + CDN 产物
	queried, err := contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Status: 200, Tasks: []contract.QueryTaskRef{taskRef},
		ResponseBody: json.RawMessage(`{"task_id":"h3-77","status":"succeeded","resolution":"768P","content":{"url":"https://cdn.minimax.example/v.mp4"},"usage":{"output_seconds":5,"input_images":1}}`),
	}), []contract.QueryTaskRef{taskRef})
	if err != nil {
		t.Fatalf("H3 轮询解析失败: %v", err)
	}
	item := queried.Items[0]
	if !item.Done || len(item.Artifacts) != 1 {
		t.Fatalf("H3 终态 item 异常: %+v", item)
	}
	termFacts, err := contract.ValidateUsageFacts(mustRawMap(t, item.Usage), plugin.Manifest)
	if err != nil || termFacts["seconds"] != 5 || termFacts["input_images"] != 1 {
		t.Fatalf("H3 终态用量异常: %v err=%v", termFacts, err)
	}
	var td map[string]any
	_ = json.Unmarshal(item.TaskData, &td)
	if td["_video_url"] != "https://cdn.minimax.example/v.mp4" {
		t.Fatalf("H3 产物 URL 缺失: %v", td)
	}
}
