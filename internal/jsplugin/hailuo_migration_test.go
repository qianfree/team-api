package jsplugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
	if m.Key != "hailuo" || len(m.Models) != 13 || len(m.Routes) != 3 {
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
		ResponseBody: json.RawMessage(`{"task_id":"h3-77","status":"succeeded","resolution":"768P","content":{"url":"https://cdn.minimax.example/v.mp4"},"usage":{"output_seconds":5,"input_image_count":1,"input_seconds":3}}`),
	}), []contract.QueryTaskRef{taskRef})
	if err != nil {
		t.Fatalf("H3 轮询解析失败: %v", err)
	}
	item := queried.Items[0]
	if !item.Done || len(item.Artifacts) != 1 {
		t.Fatalf("H3 终态 item 异常: %+v", item)
	}
	termFacts, err := contract.ValidateUsageFacts(mustRawMap(t, item.Usage), plugin.Manifest)
	// 官方 VideoTaskUsage 字段名 input_image_count / input_seconds → 插件事实键 input_images / input_video_seconds
	if err != nil || termFacts["seconds"] != 5 || termFacts["input_images"] != 1 || termFacts["input_video_seconds"] != 3 {
		t.Fatalf("H3 终态用量异常: %v err=%v", termFacts, err)
	}
	var td map[string]any
	_ = json.Unmarshal(item.TaskData, &td)
	if td["_video_url"] != "https://cdn.minimax.example/v.mp4" {
		t.Fatalf("H3 产物 URL 缺失: %v", td)
	}
}

// TestMigratedHailuoV1SuccessFileID v1 成功路径（原缺陷回归）：Success 携带 file_id 时，
// 插件经 utils.fetch 相对路径调 /v1/files/retrieve 换签名 download_url。相对路径由
// 桥接层注入的渠道 baseUrl 解析（与 WithAllowedHosts 同款机制），换回的 URL 进
// taskData._video_url 并声明产物。
func TestMigratedHailuoV1SuccessFileID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/files/retrieve", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("file_id") != "file-9" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":1004,"status_msg":"bad file_id"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"base_resp":{"status_code":0},"file":{"download_url":"https://vid.example/signed.mp4"}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cleanup, err := PublishForTest(hailuoSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("hailuo")

	taskRef := contract.QueryTaskRef{
		TaskID:   "hl-42",
		TaskData: json.RawMessage(`{"task_id":"hl-42","_isH3":false}`),
	}
	args, err := json.Marshal(contract.ParseQueryContext{
		Status:         200,
		Tasks:          []contract.QueryTaskRef{taskRef},
		ResponseBody:   json.RawMessage(`{"base_resp":{"status_code":0},"task_id":"hl-42","status":"Success","file_id":"file-9"}`),
		ChannelBaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("构造入参失败: %v", err)
	}
	// 桥接层同款注入：fetch 白名单（渠道 host ∪ manifest.allowedHosts）+ 相对路径解析基准
	ctx := WithBaseURL(WithAllowedHosts(context.Background(), []string{hostPortOf(srv.URL)}), srv.URL)
	out, err := plugin.Pool.Call(ctx, "parseQueryResponse", args)
	if err != nil {
		t.Fatalf("v1 成功解析失败（相对路径 fetch 未放行？）: %v", err)
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		t.Fatalf("parseQueryResponse 返回非对象: %v", err)
	}
	queried, err := contract.ValidateQueried(raw, []contract.QueryTaskRef{taskRef})
	if err != nil {
		t.Fatalf("ValidateQueried 失败: %v", err)
	}
	item := queried.Items[0]
	if !item.Done || item.Error != nil {
		t.Fatalf("v1 Success 应终态成功: %+v", item)
	}
	var taskData map[string]any
	_ = json.Unmarshal(item.TaskData, &taskData)
	if taskData["_video_url"] != "https://vid.example/signed.mp4" {
		t.Fatalf("签名 download_url 未写入 taskData._video_url: %v", taskData)
	}
	if len(item.Artifacts) != 1 || item.Artifacts[0].Key != "video" || item.Artifacts[0].Source.Path != "_video_url" {
		t.Fatalf("产物声明异常: %+v", item.Artifacts)
	}
}

// hostPortOf 提取 httptest 服务器地址的 host[:port]（fetch 白名单条目形态）。
func hostPortOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Host
}

// TestMigratedHailuoStandardVideos 标准视频端点解码：v1 扁平形态（参考图→
// first_frame_image）与 H3 content[] 形态（首帧图带 role）双分支。
func TestMigratedHailuoStandardVideos(t *testing.T) {
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

	decoded, err := contract.ValidateDecoded(call("decodeVideos", contract.StandardEntryContext{
		Surface: contract.SurfaceOpenAIVideos, Path: "/v1/videos", Model: "MiniMax-Hailuo-2.3",
		Body: json.RawMessage(`{"model":"MiniMax-Hailuo-2.3","prompt":"sea","seconds":6,"metadata":{"duration":6,"size":"768P","image":"https://img.example.com/a.png"}}`),
	}), plugin.Manifest, false)
	if err != nil {
		t.Fatalf("v1 标准解码失败: %v", err)
	}
	var v1 struct {
		VendorBody struct {
			Prompt          string `json:"prompt"`
			Resolution      string `json:"resolution"`
			Duration        int    `json:"duration"`
			FirstFrameImage string `json:"first_frame_image"`
		} `json:"vendorBody"`
	}
	_ = json.Unmarshal(decoded.RequestBody, &v1)
	if v1.VendorBody.Prompt != "sea" || v1.VendorBody.Resolution != "768P" ||
		v1.VendorBody.Duration != 6 || v1.VendorBody.FirstFrameImage == "" {
		t.Fatalf("v1 vendorBody 映射异常: %s", decoded.RequestBody)
	}

	decodedH3, err := contract.ValidateDecoded(call("decodeVideos", contract.StandardEntryContext{
		Surface: contract.SurfaceVideoGenerations, Path: "/v1/video/generations", Model: "MiniMax-H3",
		Body: json.RawMessage(`{"model":"MiniMax-H3","prompt":"dance","seconds":5,"metadata":{"duration":5,"ratio":"16:9","image":"https://img.example.com/b.png"}}`),
	}), plugin.Manifest, false)
	if err != nil {
		t.Fatalf("H3 标准解码失败: %v", err)
	}
	var h3 struct {
		VendorBody struct {
			Content []struct {
				Type     string `json:"type"`
				ImageURL *struct {
					URL string `json:"url"`
				} `json:"image_url"`
				Role string `json:"role"`
			} `json:"content"`
			Resolution string `json:"resolution"`
			Duration   int    `json:"duration"`
			Ratio      string `json:"ratio"`
		} `json:"vendorBody"`
	}
	_ = json.Unmarshal(decodedH3.RequestBody, &h3)
	if len(h3.VendorBody.Content) != 2 || h3.VendorBody.Content[0].Type != "text" ||
		h3.VendorBody.Content[1].ImageURL == nil || h3.VendorBody.Content[1].Role != "first_frame" ||
		h3.VendorBody.Resolution != "768P" || h3.VendorBody.Duration != 5 || h3.VendorBody.Ratio != "16:9" {
		t.Fatalf("H3 vendorBody 映射异常: %s", decodedH3.RequestBody)
	}
}

// TestMigratedHailuoStandardVideosReferences 标准端点 references[] 映射 + H3 家族型号/档位：
//   - H3：role 原样映射为 content 项（保持原序），合计 ≤12，Lite 参考模式须显式比例
//   - H3 家族保留客户端型号（不再强制改写为 MiniMax-H3），2K/480P 档位按型号校验
//   - Hailuo 2.x：首/尾帧 → first/last_frame_image，S2V-01 单张参考图 → subject_reference，其余拒绝
func TestMigratedHailuoStandardVideosReferences(t *testing.T) {
	cleanup, err := PublishForTest(hailuoSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("hailuo")

	decode := func(model, extra string) (string, json.RawMessage, error) {
		body := `{"model":"` + model + `","prompt":"reference image 1 dances","seconds":"6"` + extra + `}`
		out, err := plugin.Pool.Call(context.Background(), "decodeVideos", mustJSON(t, contract.StandardEntryContext{
			Surface: contract.SurfaceOpenAIVideos, Path: "/v1/videos", Model: model, Body: json.RawMessage(body),
		}))
		if err != nil {
			return "", nil, err
		}
		raw, err := contract.ParseJSONObject(out)
		if err != nil {
			return "", nil, err
		}
		decoded, err := contract.ValidateDecoded(raw, plugin.Manifest, false)
		if err != nil {
			return "", nil, err
		}
		return decoded.Model, decoded.RequestBody, nil
	}
	type vendorShape struct {
		Model      string `json:"model"`
		VendorBody struct {
			Content          []map[string]any `json:"content"`
			Resolution       string           `json:"resolution"`
			FirstFrameImage  string           `json:"first_frame_image"`
			LastFrameImage   string           `json:"last_frame_image"`
			SubjectReference []map[string]any `json:"subject_reference"`
		} `json:"vendorBody"`
	}

	// H3-Max：型号保留 + 参考素材映射（含音频、data URL 视频，MiniMax 均支持）
	model, body, err := decode("MiniMax-H3-Max", `,"metadata":{"resolution":"480P"},"images":["https://legacy"],"references":[
		{"role":"reference_image","url":"https://img/1.png"},
		{"role":"reference_video","url":"data:video/mp4;base64,AAAA"},
		{"role":"reference_audio","url":"https://aud/1.mp3"}]`)
	if err != nil {
		t.Fatalf("H3-Max 参考解码失败: %v", err)
	}
	var v vendorShape
	_ = json.Unmarshal(body, &v)
	if model != "MiniMax-H3-Max" || v.Model != "MiniMax-H3-Max" {
		t.Fatalf("H3 家族型号应保留: model=%s body=%s", model, body)
	}
	if v.VendorBody.Resolution != "480P" || len(v.VendorBody.Content) != 4 {
		t.Fatalf("H3-Max 映射异常（兼容字段 images[] 不得混入）: %s", body)
	}
	wantRoles := []string{"", "reference_image", "reference_video", "reference_audio"}
	for i, role := range wantRoles {
		got, _ := v.VendorBody.Content[i]["role"].(string)
		if got != role {
			t.Fatalf("content[%d].role=%q want %q: %s", i, got, role, body)
		}
	}

	// H3：2K 档位（此前 resolutionFor 不识别 2K，被改写成 H3 不支持的 720P）
	if _, body, err = decode("MiniMax-H3", `,"metadata":{"resolution":"2K"}`); err != nil {
		t.Fatalf("H3 2K 应通过: %v", err)
	}
	v = vendorShape{}
	_ = json.Unmarshal(body, &v)
	if v.VendorBody.Resolution != "2K" {
		t.Fatalf("H3 2K 档位异常: %s", body)
	}

	// Hailuo 2.x 首尾帧 + S2V-01 主体参考
	if _, body, err = decode("MiniMax-Hailuo-02", `,"references":[{"role":"first_frame","url":"https://a"},{"role":"last_frame","url":"https://b"}]`); err != nil {
		t.Fatalf("Hailuo-02 首尾帧应通过: %v", err)
	}
	v = vendorShape{}
	_ = json.Unmarshal(body, &v)
	if v.VendorBody.FirstFrameImage != "https://a" || v.VendorBody.LastFrameImage != "https://b" {
		t.Fatalf("Hailuo-02 首尾帧映射异常: %s", body)
	}
	if _, body, err = decode("S2V-01", `,"references":[{"role":"reference_image","url":"https://face"}]`); err != nil {
		t.Fatalf("S2V-01 主体参考应通过: %v", err)
	}
	v = vendorShape{}
	_ = json.Unmarshal(body, &v)
	if len(v.VendorBody.SubjectReference) != 1 {
		t.Fatalf("S2V-01 subject_reference 映射异常: %s", body)
	}

	twelve := make([]string, 0, 13)
	for i := 0; i < 9; i++ {
		twelve = append(twelve, `{"role":"reference_image","url":"https://i"}`)
	}
	for i := 0; i < 3; i++ {
		twelve = append(twelve, `{"role":"reference_video","url":"https://v"}`)
	}
	thirteen := append(append([]string{}, twelve...), `{"role":"reference_audio","url":"https://a"}`)
	if _, _, err := decode("MiniMax-H3", `,"references":[`+strings.Join(twelve, ",")+`]`); err != nil {
		t.Fatalf("12 个参考应通过: %v", err)
	}

	rejects := []struct{ name, model, extra string }{
		{"参考合计超 12", "MiniMax-H3", `,"references":[` + strings.Join(thirteen, ",") + `]`},
		{"Lite 参考模式须显式比例", "MiniMax-H3-Lite", `,"references":[{"role":"reference_image","url":"https://a"}]`},
		{"H3 不支持 480P", "MiniMax-H3", `,"metadata":{"resolution":"480P"}`},
		{"Max 不支持 2K", "MiniMax-H3-Max", `,"metadata":{"resolution":"2K"}`},
		{"Hailuo-02 不支持参考视频", "MiniMax-Hailuo-02", `,"references":[{"role":"reference_video","url":"https://v"}]`},
		{"S2V-01 仅一张参考图", "S2V-01", `,"references":[{"role":"reference_image","url":"https://a"},{"role":"reference_image","url":"https://b"}]`},
	}
	for _, c := range rejects {
		if _, _, err := decode(c.model, c.extra); err == nil {
			t.Errorf("%s：应拒绝", c.name)
		}
	}
	// Max 时长下限 5 秒（H3 为 4）
	if _, err := plugin.Pool.Call(context.Background(), "decodeVideos", mustJSON(t, contract.StandardEntryContext{
		Surface: contract.SurfaceOpenAIVideos, Path: "/v1/videos", Model: "MiniMax-H3-Max",
		Body: json.RawMessage(`{"model":"MiniMax-H3-Max","prompt":"x","seconds":"4"}`),
	})); err == nil {
		t.Error("H3-Max 4 秒应拒绝")
	}
	// Lite 显式比例后通过
	if _, _, err := decode("MiniMax-H3-Lite", `,"ratio":"16:9","references":[{"role":"reference_image","url":"https://a"}]`); err != nil {
		t.Fatalf("Lite 显式比例应通过: %v", err)
	}
}
