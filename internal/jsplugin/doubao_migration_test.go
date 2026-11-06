package jsplugin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

func doubaoSource(t *testing.T) string {
	t.Helper()
	return readBuiltin(t, "doubao")
}

func TestMigratedDoubaoManifest(t *testing.T) {
	compiled := mustCompile(t, doubaoSource(t))
	m := compiled.Manifest
	if m.Key != "doubao" || len(m.Models) != 11 || len(m.Routes) != 3 {
		t.Fatalf("manifest 异常: models=%d routes=%d", len(m.Models), len(m.Routes))
	}
	if m.UsageFieldByKey("tokens") == nil || m.UsageFieldByKey("video_input_ratio") == nil {
		t.Fatalf("usageSchema 异常: %+v", m.UsageSchema)
	}
}

func TestMigratedDoubaoVideoFlow(t *testing.T) {
	cleanup, err := PublishForTest(doubaoSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("doubao")

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
		Path: "/api/v3/contents/generations/tasks", Method: "POST",
		Body: json.RawMessage(`{"model":"doubao-seedance-2-5-260628","content":[{"type":"text","text":"ocean waves"}],"duration":10,"resolution":"720p"}`),
	}), plugin.Manifest, false)
	if err != nil || decoded.Model != "doubao-seedance-2-5-260628" {
		t.Fatalf("视频解码失败: %+v err=%v", decoded, err)
	}

	// tokens = 10 × 1280×720×24/1024 = 216000；video_input=none（无视频输入）
	facts, err := contract.ValidateUsageFacts(
		call("estimateUsage", contract.EstimateUsageContext{Body: decoded.RequestBody, Model: decoded.Model}),
		plugin.Manifest)
	if err != nil || facts["tokens"] != 216000 {
		t.Fatalf("tokens 估算异常: %v err=%v", facts, err)
	}
	if _, ok := facts["video_input"]; ok {
		t.Fatal("enum 事实应过滤不计费")
	}

	built, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: decoded.RequestBody, Model: decoded.Model,
	}))
	if err != nil || built.URL != "/api/v3/contents/generations/tasks" {
		t.Fatalf("视频 build 异常: %+v err=%v", built, err)
	}

	sub, err := contract.ValidateSubmitted(call("parseSubmitResponse", contract.ParseSubmitContext{
		Status: 200, Path: "/api/v3/contents/generations/tasks",
		ResponseBody: json.RawMessage(`{"id":"db-42","status":"queued"}`),
	}))
	if err != nil || sub.TaskID != "db-42" {
		t.Fatalf("受理解析异常: %+v err=%v", sub, err)
	}

	taskRef := contract.QueryTaskRef{TaskID: "db-42", TaskData: sub.TaskData}
	qBuilt, err := contract.ValidateBuiltRequest(call("buildQueryRequest", contract.BuildQueryContext{
		Tasks: []contract.QueryTaskRef{taskRef}, Model: decoded.Model,
	}))
	if err != nil || qBuilt.URL != "/api/v3/contents/generations/tasks/db-42" {
		t.Fatalf("查询构造异常: %+v err=%v", qBuilt, err)
	}
	// 终态：tokens 精确结算 + 视频/尾帧产物
	queried, err := contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Status: 200, Tasks: []contract.QueryTaskRef{taskRef},
		ResponseBody: json.RawMessage(`{"id":"db-42","status":"succeeded","content":{"video_url":"https://ark.example.com/v.mp4","last_frame_url":"https://ark.example.com/lf.png"},"usage":{"completion_tokens":180000,"total_tokens":200000,"video_resolution":"720p"}}`),
	}), []contract.QueryTaskRef{taskRef})
	if err != nil {
		t.Fatalf("轮询解析失败: %v", err)
	}
	item := queried.Items[0]
	if !item.Done || len(item.Artifacts) != 2 {
		t.Fatalf("终态 item 异常: %+v", item)
	}
	termFacts, err := contract.ValidateUsageFacts(mustRawMap(t, item.Usage), plugin.Manifest)
	if err != nil || termFacts["tokens"] != 200000 {
		t.Fatalf("终态 tokens 异常: %v err=%v", termFacts, err)
	}
}

func TestMigratedDoubaoImageImmediate(t *testing.T) {
	cleanup, err := PublishForTest(doubaoSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("doubao")

	args, _ := json.Marshal(contract.DecodeContext{
		Path: "/api/v3/images/generations", Method: "POST",
		Body: json.RawMessage(`{"model":"doubao-seedream-5-0-flash-260915","prompt":"a cat","size":"2K"}`),
	})
	out, err := plugin.Pool.Call(context.Background(), "createImage", args)
	if err != nil {
		t.Fatalf("createImage 失败: %v", err)
	}
	raw, _ := contract.ParseJSONObject(out)
	decoded, err := contract.ValidateDecoded(raw, plugin.Manifest, false)
	if err != nil || decoded.Model != "doubao-seedream-5-0-flash-260915" {
		t.Fatalf("图片解码失败: %+v err=%v", decoded, err)
	}

	// stream 拒绝（SSE 降级）
	streamArgs, _ := json.Marshal(contract.DecodeContext{
		Path: "/api/v3/images/generations", Method: "POST",
		Body: json.RawMessage(`{"model":"doubao-seedream-5-0-flash-260915","prompt":"x","stream":true}`),
	})
	if _, err := plugin.Pool.Call(context.Background(), "createImage", streamArgs); err == nil {
		t.Fatal("stream:true 应拒绝")
	}

	// immediate 终态：SUCCESS + image_count + 产物
	subArgs, _ := json.Marshal(contract.ParseSubmitContext{
		Status: 200, Path: "/api/v3/images/generations",
		ResponseBody: json.RawMessage(`{"data":[{"url":"https://ark.example.com/img.png"}],"usage":{"generated_images":1}}`),
	})
	subOut, err := plugin.Pool.Call(context.Background(), "parseSubmitResponse", subArgs)
	if err != nil {
		t.Fatalf("图片受理失败: %v", err)
	}
	subRaw, _ := contract.ParseJSONObject(subOut)
	sub, err := contract.ValidateSubmitted(subRaw)
	if err != nil {
		t.Fatalf("immediate 校验失败: %v", err)
	}
	if sub.Im == nil || sub.Im.Status != "SUCCESS" || len(sub.Im.Artifacts) != 1 {
		t.Fatalf("immediate 异常: %+v", sub.Im)
	}
	imFacts, err := contract.ValidateUsageFacts(mustRawMap(t, sub.Im.Usage), plugin.Manifest)
	if err != nil || imFacts["image_count"] != 1 {
		t.Fatalf("immediate 用量异常: %v err=%v", imFacts, err)
	}
}
