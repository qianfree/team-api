package jsplugin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

func xaiSource(t *testing.T) string {
	t.Helper()
	return readBuiltin(t, "xai")
}

func TestMigratedXaiManifest(t *testing.T) {
	compiled := mustCompile(t, xaiSource(t))
	m := compiled.Manifest
	if m.Key != "xai" || len(m.Models) != 3 || len(m.Routes) != 4 {
		t.Fatalf("manifest 异常: models=%v routes=%d", m.Models, len(m.Routes))
	}
	if m.UsageFieldByKey("seconds") == nil || m.UsageFieldByKey("moderated") == nil {
		t.Fatalf("usageSchema 异常: %+v", m.UsageSchema)
	}
	if m.UsageFieldByKey("moderated").Type != "boolean" {
		t.Fatal("moderated 应为 boolean 型")
	}
}

func TestMigratedXaiGenerationFlow(t *testing.T) {
	cleanup, err := PublishForTest(xaiSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("xai")

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

	// 解码（generations，模型校验 + 分辨率白名单）
	decoded, err := contract.ValidateDecoded(call("createGeneration", contract.DecodeContext{
		Path: "/videos/generations", Method: "POST",
		Body: json.RawMessage(`{"model":"grok-imagine-video-1.5","prompt":"sunset","duration":8,"resolution":"720p","image":"https://img.example.com/a.png"}`),
	}), plugin.Manifest, false)
	if err != nil || decoded.Model != "grok-imagine-video-1.5" {
		t.Fatalf("解码失败: %+v err=%v", decoded, err)
	}

	// 估算：seconds=8 input_images=1
	facts, err := contract.ValidateUsageFacts(
		call("estimateUsage", contract.EstimateUsageContext{Body: decoded.RequestBody, Model: decoded.Model}),
		plugin.Manifest)
	if err != nil || facts["seconds"] != 8 || facts["input_images"] != 1 {
		t.Fatalf("估算异常: %v err=%v", facts, err)
	}
	// moderated（boolean）过滤不计费
	if _, ok := facts["moderated"]; ok {
		t.Fatal("boolean 事实应被过滤")
	}

	// build：_action 剥离 + 相对 URL
	built, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: decoded.RequestBody, Model: decoded.Model,
	}))
	if err != nil || built.URL != "/v1/videos/generations" {
		t.Fatalf("build 异常: %+v err=%v", built, err)
	}
	var builtBody map[string]any
	_ = json.Unmarshal(built.Body, &builtBody)
	if builtBody["_action"] != nil || builtBody["duration"] != float64(8) {
		t.Fatalf("body 异常: %v", builtBody)
	}

	// 受理（request_id）
	sub, err := contract.ValidateSubmitted(call("parseSubmitResponse", contract.ParseSubmitContext{
		Status: 200, Path: "/videos/generations",
		ResponseBody: json.RawMessage(`{"request_id":"req-77","status":"pending"}`),
	}))
	if err != nil || sub.TaskID != "req-77" {
		t.Fatalf("受理解析异常: %+v err=%v", sub, err)
	}

	// 轮询：done → usage（seconds/resolution）+ video.url 产物
	taskRef := contract.QueryTaskRef{TaskID: "req-77", TaskData: sub.TaskData}
	queried, err := contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Status: 200, Tasks: []contract.QueryTaskRef{taskRef},
		ResponseBody: json.RawMessage(`{"request_id":"req-77","status":"done","video":{"url":"https://vidgen.x.ai/v/abc.mp4","duration":8},"resolution":"720p"}`),
	}), []contract.QueryTaskRef{taskRef})
	if err != nil {
		t.Fatalf("轮询解析失败: %v", err)
	}
	item := queried.Items[0]
	if !item.Done || len(item.Artifacts) != 1 || item.Artifacts[0].Source.Path != "video.url" {
		t.Fatalf("终态 item 异常: %+v", item)
	}
	termFacts, err := contract.ValidateUsageFacts(mustRawMap(t, item.Usage), plugin.Manifest)
	if err != nil || termFacts["seconds"] != 8 {
		t.Fatalf("终态用量异常: %v err=%v", termFacts, err)
	}

	// 审核拦截仍计费（chargedFailure → done + moderated）
	queried2, err := contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Status: 200, Tasks: []contract.QueryTaskRef{taskRef},
		ResponseBody: json.RawMessage(`{"status":"failed","failure":{"billed":true,"message":"Usage Guidelines Violation"}}`),
	}), []contract.QueryTaskRef{taskRef})
	if err != nil || !queried2.Items[0].Done {
		t.Fatalf("审核计费失败结算异常: %+v err=%v", queried2.Items[0], err)
	}
	facts2, _ := contract.ValidateUsageFacts(mustRawMap(t, queried2.Items[0].Usage), plugin.Manifest)
	_ = facts2
	var usageRaw map[string]json.RawMessage
	_ = json.Unmarshal(queried2.Items[0].Usage, &usageRaw)
	if string(usageRaw["moderated"]) != "true" {
		t.Fatalf("moderated 标记缺失: %s", queried2.Items[0].Usage)
	}
	_ = strings.TrimSpace
}

// TestMigratedXaiStandardVideos 标准视频端点解码：复用 generationRequest 全套
// 校验；参考图 → image_to_video action；metadata.size → resolution。
func TestMigratedXaiStandardVideos(t *testing.T) {
	cleanup, err := PublishForTest(xaiSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("xai")

	args, _ := json.Marshal(contract.StandardEntryContext{
		Surface: contract.SurfaceOpenAIVideos, Path: "/v1/videos", Model: "grok-imagine-video",
		Body: json.RawMessage(`{"model":"grok-imagine-video","prompt":"race","seconds":6,"metadata":{"duration":6,"size":"720p","aspect_ratio":"16:9","image":"https://img.example.com/a.png"}}`),
	})
	out, err := plugin.Pool.Call(context.Background(), "decodeVideos", args)
	if err != nil {
		t.Fatalf("decodeVideos 失败: %v", err)
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		t.Fatalf("decodeVideos 返回非对象: %v", err)
	}
	decoded, err := contract.ValidateDecoded(raw, plugin.Manifest, false)
	if err != nil || decoded.Model != "grok-imagine-video" {
		t.Fatalf("标准解码失败: %+v err=%v", decoded, err)
	}
	var body struct {
		Duration    int    `json:"duration"`
		Resolution  string `json:"resolution"`
		AspectRatio string `json:"aspect_ratio"`
		Image       *struct {
			URL string `json:"url"`
		} `json:"image"`
		Action string `json:"_action"`
	}
	_ = json.Unmarshal(decoded.RequestBody, &body)
	if body.Duration != 6 || body.Resolution != "720p" || body.AspectRatio != "16:9" ||
		body.Image == nil || body.Image.URL != "https://img.example.com/a.png" || body.Action != "image_to_video" {
		t.Fatalf("标准映射异常: %s", decoded.RequestBody)
	}
}
