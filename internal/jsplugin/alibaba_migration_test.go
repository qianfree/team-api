package jsplugin

import (
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
	if !m.SupportsUpstreamModel("wan3.0-video") || !m.SupportsUpstreamModel("qwen-image") {
		t.Fatalf("上游模型清单异常: %v", m.Models)
	}
}

func TestMigratedAlibabaVideoFlow(t *testing.T) {
	cleanup, err := PublishForTest(alibabaSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("alibaba")
	call := hookCaller(t, plugin)

	// 平台模型名是渠道映射别名（不在 meta.models）：decode 原样回显、不校验型号
	decoded, err := contract.ValidateDecoded(call("createTask", contract.DecodeContext{
		Path: "/api/v1/services/aigc/video-generation/video-synthesis", Method: "POST",
		Body: json.RawMessage(`{"model":"my-wan3","input":{"prompt":"city night"},"parameters":{"duration":10,"resolution":"1080P","audio":true}}`),
	}), false)
	if err != nil || decoded.Model != "my-wan3" {
		t.Fatalf("视频解码失败: %+v err=%v", decoded, err)
	}

	// validateRequest 按上游型号校验：非视频模型拒绝、分辨率越档拒绝
	if err := hookCallErr(plugin, "validateRequest", contract.ValidateRequestContext{
		Body: decoded.RequestBody, Model: decoded.Model, UpstreamModel: "qwen-image",
	}); err == nil {
		t.Fatal("上游为图像模型时视频请求应被拒绝")
	}
	vres, err := contract.ValidateValidatedRequest(call("validateRequest", contract.ValidateRequestContext{
		Body: decoded.RequestBody, Model: decoded.Model, UpstreamModel: "wan3.0-video",
	}))
	if err != nil || len(vres.RequestBody) == 0 {
		t.Fatalf("validateRequest 异常: %+v err=%v", vres, err)
	}
	body := vres.RequestBody

	est := mustHostUsageObj(t, call("estimateUsage", contract.EstimateUsageContext{Body: body, Model: decoded.Model, UpstreamModel: "wan3.0-video"}))
	if v, _ := est.Number(contract.UsageOutputSeconds); v != 10 || est.Resolution != "1080P" || est.GenerateAudio == nil || !*est.GenerateAudio {
		t.Fatalf("估算异常: %+v", est)
	}

	built, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: body, Model: decoded.Model, UpstreamModel: "wan3.0-video",
	}))
	if err != nil || built.URL != "/api/v1/services/aigc/video-generation/video-synthesis" {
		t.Fatalf("build 异常: %+v err=%v", built, err)
	}
	if built.Headers["X-DashScope-Async"] != "enable" {
		t.Fatal("异步头缺失")
	}
	var sent map[string]any
	_ = json.Unmarshal(built.Body, &sent)
	if sent["model"] != "wan3.0-video" {
		t.Fatalf("上游请求应使用上游模型名: %v", sent["model"])
	}

	sub, err := contract.ValidateSubmitted(call("parseSubmitResponse", contract.ParseSubmitContext{
		Status: 200, Path: "/api/v1/services/aigc/video-generation/video-synthesis", Model: decoded.Model, Body: body,
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
	// wan3.0 终态：输出/输入双段分别上报（合并计价归宿主定价），型号取受理时记下的上游名
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
	term := mustHostUsage(t, item.Usage)
	out, _ := term.Number(contract.UsageOutputSeconds)
	in, _ := term.Number(contract.UsageInputVideoSeconds)
	if out != 10 || in != 3 || term.Resolution != "1080P" {
		t.Fatalf("wan3.0 双段用量异常: %+v", term)
	}
}

func TestMigratedAlibabaSyncImage(t *testing.T) {
	cleanup, err := PublishForTest(alibabaSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("alibaba")
	call := hookCaller(t, plugin)

	// 流式拒绝（SSE 降级）
	if err := hookCallErr(plugin, "createImage", contract.DecodeContext{
		Path: "/api/v1/services/aigc/multimodal-generation/generation", Method: "POST",
		Body: json.RawMessage(`{"model":"qwen-image","input":{"prompt":"x"},"parameters":{"stream":true}}`),
	}); err == nil {
		t.Fatal("stream 应拒绝")
	}

	// 同步路由 + 异步型号：validateRequest 拒绝
	decoded, err := contract.ValidateDecoded(call("createImage", contract.DecodeContext{
		Path: "/api/v1/services/aigc/multimodal-generation/generation", Method: "POST",
		Body: json.RawMessage(`{"model":"qwen-image","input":{"prompt":"x"}}`),
	}), false)
	if err != nil {
		t.Fatalf("图像解码失败: %v", err)
	}
	if err := hookCallErr(plugin, "validateRequest", contract.ValidateRequestContext{
		Body: decoded.RequestBody, Model: decoded.Model, UpstreamModel: "wan2.6-t2i",
	}); err == nil {
		t.Fatal("同步图像路由映射到异步型号应被拒绝")
	}

	// immediate：双图交付 + 产物
	sub, err := contract.ValidateSubmitted(call("parseSubmitResponse", contract.ParseSubmitContext{
		Status: 200, Path: "/api/v1/services/aigc/multimodal-generation/generation",
		ResponseBody: json.RawMessage(`{"output":{"choices":[{"message":{"content":[{"text":"done"},{"image":"https://dashscope.example.com/a.png"},{"image":"https://dashscope.example.com/b.png"}]}}]},"usage":{"image_count":2}}`),
	}))
	if err != nil || sub.Im == nil || sub.Im.Status != "SUCCESS" {
		t.Fatalf("immediate 异常: %+v err=%v", sub, err)
	}
	if v, _ := mustHostUsage(t, sub.Im.Usage).Number(contract.UsageOutputCount); v != 2 {
		t.Fatalf("immediate 用量异常: %s", sub.Im.Usage)
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

// TestMigratedAlibabaStandardVideos 标准视频端点：decodeVideos 出型号中立体，
// validateRequest 按上游型号落位参考图（wan2.x input.img_url / wan3.0 input.media）
// 与音频开关（仅 wan3.0 系）。
func TestMigratedAlibabaStandardVideos(t *testing.T) {
	cleanup, err := PublishForTest(alibabaSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("alibaba")
	call := hookCaller(t, plugin)

	finalize := func(platform, upstream, body string) json.RawMessage {
		t.Helper()
		decoded, err := contract.ValidateDecoded(call("decodeVideos", contract.StandardEntryContext{
			Surface: contract.SurfaceOpenAIVideos, Path: "/v1/videos", Model: platform, Body: json.RawMessage(body),
		}), false)
		if err != nil {
			t.Fatalf("标准解码失败: %v", err)
		}
		res, err := contract.ValidateValidatedRequest(call("validateRequest", contract.ValidateRequestContext{
			Body: decoded.RequestBody, Model: decoded.Model, UpstreamModel: upstream,
		}))
		if err != nil || len(res.RequestBody) == 0 {
			t.Fatalf("validateRequest 应定稿请求体: %+v err=%v", res, err)
		}
		return res.RequestBody
	}

	wan2Body := finalize("video-pro", "wan2.6-i2v",
		`{"model":"video-pro","prompt":"forest","seconds":8,"metadata":{"duration":8,"resolution":"1080P","ratio":"16:9","image":"https://img.example.com/a.png","sound":"on"}}`)
	var wan2 struct {
		VendorBody struct {
			Input struct {
				ImgURL string `json:"img_url"`
				Media  []any  `json:"media"`
			} `json:"input"`
			Parameters struct {
				Duration   int    `json:"duration"`
				Resolution string `json:"resolution"`
				Ratio      string `json:"ratio"`
				Audio      *bool  `json:"audio"`
			} `json:"parameters"`
		} `json:"vendorBody"`
	}
	_ = json.Unmarshal(wan2Body, &wan2)
	if wan2.VendorBody.Input.ImgURL == "" || len(wan2.VendorBody.Input.Media) != 0 || wan2.VendorBody.Parameters.Duration != 8 ||
		wan2.VendorBody.Parameters.Resolution != "1080P" || wan2.VendorBody.Parameters.Ratio != "16:9" || wan2.VendorBody.Parameters.Audio != nil {
		t.Fatalf("wan2.x vendorBody 映射异常: %s", wan2Body)
	}

	wan3Body := finalize("video-pro", "wan3.0-video",
		`{"model":"video-pro","prompt":"city night","seconds":10,"metadata":{"duration":10,"image":"https://img.example.com/b.png","sound":"on"}}`)
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
	_ = json.Unmarshal(wan3Body, &wan3)
	if len(wan3.VendorBody.Input.Media) != 1 || wan3.VendorBody.Input.Media[0].Type != "image" ||
		wan3.VendorBody.Parameters.Audio == nil || !*wan3.VendorBody.Parameters.Audio {
		t.Fatalf("wan3.0 vendorBody 映射异常: %s", wan3Body)
	}

	// 越档分辨率按上游型号拒绝（wan2.7 不支持 480P）
	decoded, _ := contract.ValidateDecoded(call("decodeVideos", contract.StandardEntryContext{
		Surface: contract.SurfaceOpenAIVideos, Path: "/v1/videos", Model: "video-pro",
		Body: json.RawMessage(`{"model":"video-pro","prompt":"x","metadata":{"resolution":"480P"}}`),
	}), false)
	if err := hookCallErr(plugin, "validateRequest", contract.ValidateRequestContext{
		Body: decoded.RequestBody, Model: decoded.Model, UpstreamModel: "wan2.7-t2v",
	}); err == nil {
		t.Fatal("wan2.7 480P 应被拒绝")
	}
}
