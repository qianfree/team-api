package jsplugin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

func alibabaSource(t *testing.T) string {
	t.Helper()
	return readBuiltin(t, "alibaba")
}

func TestMigratedAlibabaManifest(t *testing.T) {
	compiled := mustCompile(t, alibabaSource(t))
	m := compiled.Manifest
	if m.Key != "alibaba" || len(m.Models) != 42 || len(m.Routes) != 5 {
		t.Fatalf("manifest 异常: models=%d routes=%d", len(m.Models), len(m.Routes))
	}
	if m.UsageFieldByKey("seconds") == nil || m.UsageFieldByKey("audio") == nil {
		t.Fatalf("usageSchema 异常: %+v", m.UsageSchema)
	}
}

func TestMigratedAlibabaVideoFlow(t *testing.T) {
	cleanup, err := PublishForTest(alibabaSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("alibaba")

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

	// wan3.0：input+output 双段计费
	decoded, err := contract.ValidateDecoded(call("createTask", contract.DecodeContext{
		Path: "/api/v1/services/aigc/video-generation/video-synthesis", Method: "POST",
		Body: json.RawMessage(`{"model":"wan3.0-video","input":{"prompt":"city night"},"parameters":{"duration":10,"resolution":"1080P","audio":true}}`),
	}), plugin.Manifest, false)
	if err != nil || decoded.Model != "wan3.0-video" {
		t.Fatalf("视频解码失败: %+v err=%v", decoded, err)
	}

	facts, err := contract.ValidateUsageFacts(
		call("estimateUsage", contract.EstimateUsageContext{Body: decoded.RequestBody, Model: decoded.Model}),
		plugin.Manifest)
	if err != nil || facts["seconds"] != 10 {
		t.Fatalf("估算异常: %v err=%v", facts, err)
	}

	built, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: decoded.RequestBody, Model: decoded.Model,
	}))
	if err != nil || built.URL != "/api/v1/services/aigc/video-generation/video-synthesis" {
		t.Fatalf("build 异常: %+v err=%v", built, err)
	}
	if built.Headers["X-DashScope-Async"] != "enable" {
		t.Fatal("异步头缺失")
	}

	sub, err := contract.ValidateSubmitted(call("parseSubmitResponse", contract.ParseSubmitContext{
		Status: 200, Path: "/api/v1/services/aigc/video-generation/video-synthesis", Model: "wan3.0-video",
		ResponseBody: json.RawMessage(`{"output":{"task_id":"ds-42","task_status":"PENDING"},"request_id":"req-1"}`),
	}))
	if err != nil || sub.TaskID != "ds-42" {
		t.Fatalf("受理解析异常: %+v err=%v", sub, err)
	}

	taskRef := contract.QueryTaskRef{TaskID: "ds-42", TaskData: sub.TaskData}
	qBuilt, err := contract.ValidateBuiltRequest(call("buildQueryRequest", contract.BuildQueryContext{
		Tasks: []contract.QueryTaskRef{taskRef}, Model: decoded.Model,
	}))
	if err != nil || qBuilt.URL != "/api/v1/tasks/ds-42" {
		t.Fatalf("查询构造异常: %+v err=%v", qBuilt, err)
	}
	// wan3.0 终态：seconds = input(3) + output(10) = 13 + video_url 产物
	queried, err := contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Status: 200, Tasks: []contract.QueryTaskRef{taskRef}, ChannelBaseURL: "https://dashscope.example.com",
		ResponseBody: json.RawMessage(`{"output":{"task_id":"ds-42","task_status":"SUCCEEDED","video_url":"https://dashscope.example.com/v.mp4"},"usage":{"input_video_duration":3,"output_video_duration":10,"SR":"1080P","audio":true}}`),
	}), []contract.QueryTaskRef{taskRef})
	if err != nil {
		t.Fatalf("轮询解析失败: %v", err)
	}
	item := queried.Items[0]
	if !item.Done || len(item.Artifacts) != 1 {
		t.Fatalf("终态 item 异常: %+v", item)
	}
	termFacts, err := contract.ValidateUsageFacts(mustRawMap(t, item.Usage), plugin.Manifest)
	if err != nil || termFacts["seconds"] != 13 {
		t.Fatalf("wan3.0 双段时长异常: %v err=%v", termFacts, err)
	}
}

func TestMigratedAlibabaSyncImage(t *testing.T) {
	cleanup, err := PublishForTest(alibabaSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("alibaba")

	// 流式拒绝（SSE 降级）
	streamArgs, _ := json.Marshal(contract.DecodeContext{
		Path: "/api/v1/services/aigc/multimodal-generation/generation", Method: "POST",
		Body: json.RawMessage(`{"model":"qwen-image","input":{"prompt":"x"},"parameters":{"stream":true}}`),
	})
	if _, err := plugin.Pool.Call(context.Background(), "createImage", streamArgs); err == nil {
		t.Fatal("stream 应拒绝")
	}

	// immediate：双图交付 + 产物
	subArgs, _ := json.Marshal(contract.ParseSubmitContext{
		Status: 200, Path: "/api/v1/services/aigc/multimodal-generation/generation",
		ResponseBody: json.RawMessage(`{"output":{"choices":[{"message":{"content":[{"text":"done"},{"image":"https://dashscope.example.com/a.png"},{"image":"https://dashscope.example.com/b.png"}]}}]},"usage":{"image_count":2}}`),
	})
	subOut, err := plugin.Pool.Call(context.Background(), "parseSubmitResponse", subArgs)
	if err != nil {
		t.Fatalf("图片受理失败: %v", err)
	}
	subRaw, _ := contract.ParseJSONObject(subOut)
	sub, err := contract.ValidateSubmitted(subRaw)
	if err != nil || sub.Im == nil || sub.Im.Status != "SUCCESS" {
		t.Fatalf("immediate 异常: %+v err=%v", sub.Im, err)
	}
	imFacts, err := contract.ValidateUsageFacts(mustRawMap(t, sub.Im.Usage), plugin.Manifest)
	if err != nil || imFacts["image_count"] != 2 {
		t.Fatalf("immediate 用量异常: %v err=%v", imFacts, err)
	}
	if len(sub.Im.Artifacts) != 1 {
		t.Fatalf("immediate 产物异常: %+v", sub.Im.Artifacts)
	}
	var td map[string]any
	_ = json.Unmarshal(sub.Im.TaskData, &td)
	urls, _ := td["_image_urls"].([]any)
	if len(urls) != 2 {
		t.Fatalf("_image_urls 异常: %v", td["_image_urls"])
	}
}

// TestMigratedAlibabaStandardVideos 标准视频端点解码：wan2.x（input.img_url +
// parameters 双段）与 wan3.0（input.media + 音频开关）双形态。
func TestMigratedAlibabaStandardVideos(t *testing.T) {
	cleanup, err := PublishForTest(alibabaSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("alibaba")

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

	decoded, err := contract.ValidateDecoded(call("decodeVideos", contract.StandardEntryContext{
		Surface: contract.SurfaceOpenAIVideos, Path: "/v1/videos", Model: "wan2.6-i2v",
		Body: json.RawMessage(`{"model":"wan2.6-i2v","prompt":"forest","seconds":8,"metadata":{"duration":8,"resolution":"1080P","ratio":"16:9","image":"https://img.example.com/a.png"}}`),
	}), plugin.Manifest, false)
	if err != nil {
		t.Fatalf("wan2.x 标准解码失败: %v", err)
	}
	var wan2 struct {
		VendorBody struct {
			Input struct {
				ImgURL string `json:"img_url"`
			} `json:"input"`
			Parameters struct {
				Duration   int    `json:"duration"`
				Resolution string `json:"resolution"`
				Ratio      string `json:"ratio"`
			} `json:"parameters"`
		} `json:"vendorBody"`
	}
	_ = json.Unmarshal(decoded.RequestBody, &wan2)
	if wan2.VendorBody.Input.ImgURL == "" || wan2.VendorBody.Parameters.Duration != 8 ||
		wan2.VendorBody.Parameters.Resolution != "1080P" || wan2.VendorBody.Parameters.Ratio != "16:9" {
		t.Fatalf("wan2.x vendorBody 映射异常: %s", decoded.RequestBody)
	}

	decoded3, err := contract.ValidateDecoded(call("decodeVideos", contract.StandardEntryContext{
		Surface: contract.SurfaceVideoGenerations, Path: "/v1/video/generations", Model: "wan3.0-video",
		Body: json.RawMessage(`{"model":"wan3.0-video","prompt":"city night","seconds":10,"metadata":{"duration":10,"image":"https://img.example.com/b.png","sound":"on"}}`),
	}), plugin.Manifest, false)
	if err != nil {
		t.Fatalf("wan3.0 标准解码失败: %v", err)
	}
	var wan3 struct {
		VendorBody struct {
			Input struct {
				Media []struct {
					Type string `json:"type"`
					URL  string `json:"url"`
				} `json:"media"`
			} `json:"input"`
			Parameters struct {
				Audio *bool `json:"audio"`
			} `json:"parameters"`
		} `json:"vendorBody"`
	}
	_ = json.Unmarshal(decoded3.RequestBody, &wan3)
	if len(wan3.VendorBody.Input.Media) != 1 || wan3.VendorBody.Input.Media[0].Type != "image" ||
		wan3.VendorBody.Parameters.Audio == nil || !*wan3.VendorBody.Parameters.Audio {
		t.Fatalf("wan3.0 vendorBody 映射异常: %s", decoded3.RequestBody)
	}
}
