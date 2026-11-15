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

// TestMigratedDoubaoStandardVideos 标准视频端点解码：content[] 组装（text +
// image_url 项）、resolution 档位校验透传、图像模型拒绝。
func TestMigratedDoubaoStandardVideos(t *testing.T) {
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

	decoded, err := contract.ValidateDecoded(call("decodeVideos", contract.StandardEntryContext{
		Surface: contract.SurfaceOpenAIVideos, Path: "/v1/videos", Model: "doubao-seedance-2-5-260628",
		Body: json.RawMessage(`{"model":"doubao-seedance-2-5-260628","prompt":"city","seconds":10,"metadata":{"duration":10,"resolution":"1080p","image":"https://img.example.com/a.png"}}`),
	}), plugin.Manifest, false)
	if err != nil {
		t.Fatalf("标准解码失败: %v", err)
	}
	var vendor struct {
		VendorBody struct {
			Content []struct {
				Type     string `json:"type"`
				ImageURL *struct {
					URL string `json:"url"`
				} `json:"image_url"`
			} `json:"content"`
			Duration   int    `json:"duration"`
			Resolution string `json:"resolution"`
		} `json:"vendorBody"`
	}
	_ = json.Unmarshal(decoded.RequestBody, &vendor)
	if len(vendor.VendorBody.Content) != 2 || vendor.VendorBody.Content[1].ImageURL == nil ||
		vendor.VendorBody.Duration != 10 || vendor.VendorBody.Resolution != "1080p" {
		t.Fatalf("vendorBody 映射异常: %s", decoded.RequestBody)
	}

	if _, err := plugin.Pool.Call(context.Background(), "decodeVideos", mustJSON(t, contract.StandardEntryContext{
		Surface: contract.SurfaceOpenAIVideos, Path: "/v1/videos", Model: "doubao-seedream-5-0-pro-260628",
		Body: json.RawMessage(`{"model":"doubao-seedream-5-0-pro-260628","prompt":"x"}`),
	})); err == nil {
		t.Fatal("图像模型应拒绝视频端点")
	}
}

// TestMigratedDoubaoStandardVideosReferences 标准端点 references[] → 带 role 的 Ark content 项
// （保持原序）+ Seedance 模型层约束（2.x 才支持多模态参考、音频须搭配图/视频、
// 参考视频不收 base64、尾帧须搭配首帧）。
func TestMigratedDoubaoStandardVideosReferences(t *testing.T) {
	cleanup, err := PublishForTest(doubaoSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("doubao")

	decode := func(model, refs string) (json.RawMessage, error) {
		body := `{"model":"` + model + `","prompt":"图片1的人物在视频1里","seconds":"10","images":["https://legacy/x.png"],"references":` + refs + `}`
		out, err := plugin.Pool.Call(context.Background(), "decodeVideos", mustJSON(t, contract.StandardEntryContext{
			Surface: contract.SurfaceOpenAIVideos, Path: "/v1/videos", Model: model, Body: json.RawMessage(body),
		}))
		if err != nil {
			return nil, err
		}
		raw, err := contract.ParseJSONObject(out)
		if err != nil {
			return nil, err
		}
		decoded, err := contract.ValidateDecoded(raw, plugin.Manifest, false)
		if err != nil {
			return nil, err
		}
		return decoded.RequestBody, nil
	}

	const model = "doubao-seedance-2-0-260128"
	body, err := decode(model, `[
		{"role":"reference_image","url":"https://img/a.png"},
		{"role":"reference_video","url":"https://vid/b.mp4"},
		{"role":"reference_image","url":"data:image/png;base64,AAAA"},
		{"role":"reference_audio","url":"https://aud/c.mp3"}]`)
	if err != nil {
		t.Fatalf("多模态参考解码失败: %v", err)
	}
	var vendor struct {
		VendorBody struct {
			Content []map[string]any `json:"content"`
		} `json:"vendorBody"`
	}
	_ = json.Unmarshal(body, &vendor)
	content := vendor.VendorBody.Content
	wantTypes := []string{"text", "image_url", "video_url", "image_url", "audio_url"}
	wantRoles := []string{"", "reference_image", "reference_video", "reference_image", "reference_audio"}
	if len(content) != len(wantTypes) {
		t.Fatalf("content 项数异常（兼容字段 images[] 不得混入）: %s", body)
	}
	for i := range wantTypes {
		role, _ := content[i]["role"].(string)
		if content[i]["type"] != wantTypes[i] || role != wantRoles[i] {
			t.Fatalf("content[%d] 异常: %v", i, content[i])
		}
	}
	if u := content[2]["video_url"].(map[string]any)["url"]; u != "https://vid/b.mp4" {
		t.Fatalf("video_url 映射异常: %v", content[2])
	}

	// 首尾帧
	if _, err := decode(model, `[{"role":"first_frame","url":"https://a"},{"role":"last_frame","url":"https://b"}]`); err != nil {
		t.Fatalf("首尾帧应通过: %v", err)
	}

	rejects := []struct {
		name, model, refs string
	}{
		{"1.x 不支持多模态参考", "doubao-seedance-1-0-pro-250528", `[{"role":"reference_image","url":"https://a"}]`},
		{"音频须搭配图或视频", model, `[{"role":"reference_audio","url":"https://a.mp3"}]`},
		{"参考视频不收 base64", model, `[{"role":"reference_video","url":"data:video/mp4;base64,AAAA"}]`},
		{"尾帧须搭配首帧", model, `[{"role":"last_frame","url":"https://b"}]`},
		{"参考视频超 3 段", model, `[{"role":"reference_video","url":"https://1"},{"role":"reference_video","url":"https://2"},{"role":"reference_video","url":"https://3"},{"role":"reference_video","url":"https://4"}]`},
		{"未解析的 file_id", model, `[{"role":"reference_image","file_id":"file-abc12345"}]`},
	}
	for _, c := range rejects {
		if _, err := decode(c.model, c.refs); err == nil {
			t.Errorf("%s：应拒绝", c.name)
		}
	}
}
